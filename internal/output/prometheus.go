package output

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ownkube/kubernetes-events-exporter/internal/event"
	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusConfig struct {
	EventsMetricsNamePrefix string              `yaml:"eventsMetricsNamePrefix"`
	MetricExpirationSeconds int64               `yaml:"metricExpirationSeconds,omitempty"`
	ReasonFilter            map[string][]string `yaml:"reasonFilter"`
}

type trackedPrometheusSeries struct {
	labels   prometheus.Labels
	lastSeen time.Time
}

type PrometheusSink struct {
	cfg            *PrometheusConfig
	eventGauge     *prometheus.GaugeVec
	allowedReasons map[string]map[string]struct{}
	seriesByKey    map[string]trackedPrometheusSeries
	metricTTL      time.Duration
	cleanupTicker  *time.Ticker
	stopCh         chan struct{}
	doneCh         chan struct{}
	mu             sync.Mutex
}

func NewPrometheusHandler(config *PrometheusConfig) (Handler, error) {
	if config.EventsMetricsNamePrefix == "" {
		config.EventsMetricsNamePrefix = "event_exporter_"
	}

	metricLabels := []string{
		"name",
		"namespace",
		"involved_object_namespace",
		"involved_object_name",
		"involved_object_kind",
		"reason",
		"type",
		"source",
	}
	eventGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: config.EventsMetricsNamePrefix + "event_count",
			Help: "Event counts by event metadata and involved object.",
		}, metricLabels,
	)
	prometheus.MustRegister(eventGauge)

	slog.Info("Initializing Prometheus sink")
	allowedReasons := make(map[string]map[string]struct{}, len(config.ReasonFilter))
	for kind, reasons := range config.ReasonFilter {
		kindReasons := make(map[string]struct{}, len(reasons))
		for _, reason := range reasons {
			kindReasons[reason] = struct{}{}
		}
		allowedReasons[kind] = kindReasons
	}

	slog.Info("Created Prometheus metric", "name", config.EventsMetricsNamePrefix+"event_count", "labels", metricLabels)

	sink := &PrometheusSink{
		cfg:            config,
		eventGauge:     eventGauge,
		allowedReasons: allowedReasons,
		seriesByKey:    make(map[string]trackedPrometheusSeries),
		metricTTL:      time.Duration(config.MetricExpirationSeconds) * time.Second,
	}

	if sink.metricTTL > 0 {
		sink.startCleanupLoop()
		slog.Info("Prometheus sink metric expiration enabled", "ttl", sink.metricTTL.String())
	}

	return sink, nil
}

func (p *PrometheusSink) Send(_ context.Context, ev *event.EnrichedEvent) error {
	kind := ev.InvolvedObject.Kind
	reasonsForKind, ok := p.allowedReasons[kind]
	if !ok {
		return nil
	}
	if _, allowed := reasonsForKind[ev.Reason]; !allowed {
		return nil
	}

	labels := prometheus.Labels{
		"name":                      ev.Name,
		"namespace":                 ev.Namespace,
		"involved_object_namespace": ev.InvolvedObject.Namespace,
		"involved_object_name":      ev.InvolvedObject.Name,
		"involved_object_kind":      ev.InvolvedObject.Kind,
		"reason":                    ev.Reason,
		"type":                      ev.Type,
		"source":                    eventSource(ev),
	}

	p.eventGauge.With(labels).Set(float64(ev.Count))
	p.trackSeries(labels, eventObservedAt(ev))

	return nil
}

func (p *PrometheusSink) Close() {
	if p.stopCh != nil {
		close(p.stopCh)
		p.cleanupTicker.Stop()
		<-p.doneCh
	}

	prometheus.Unregister(p.eventGauge)
}

func (p *PrometheusSink) startCleanupLoop() {
	interval := p.metricTTL / 2
	if interval <= 0 {
		interval = p.metricTTL
	}
	if interval > time.Minute {
		interval = time.Minute
	}

	p.cleanupTicker = time.NewTicker(interval)
	p.stopCh = make(chan struct{})
	p.doneCh = make(chan struct{})

	go func() {
		defer close(p.doneCh)
		for {
			select {
			case <-p.cleanupTicker.C:
				p.deleteExpiredSeries(time.Now())
			case <-p.stopCh:
				return
			}
		}
	}()
}

func (p *PrometheusSink) trackSeries(labels prometheus.Labels, observedAt time.Time) {
	if p.metricTTL <= 0 {
		return
	}

	if observedAt.IsZero() {
		observedAt = time.Now()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	key := seriesKey(labels)
	existing, exists := p.seriesByKey[key]
	if exists && existing.lastSeen.After(observedAt) {
		return
	}

	p.seriesByKey[key] = trackedPrometheusSeries{labels: cloneLabels(labels), lastSeen: observedAt}
}

func (p *PrometheusSink) deleteExpiredSeries(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, series := range p.seriesByKey {
		if now.Sub(series.lastSeen) < p.metricTTL {
			continue
		}

		p.eventGauge.Delete(series.labels)
		delete(p.seriesByKey, key)

		slog.Debug("Expired Prometheus metric series",
			"kind", series.labels["involved_object_kind"],
			"namespace", series.labels["namespace"],
			"name", series.labels["name"],
			"reason", series.labels["reason"],
			"ttl", p.metricTTL.String())
	}
}

func seriesKey(labels prometheus.Labels) string {
	parts := []string{
		labels["name"],
		labels["namespace"],
		labels["involved_object_namespace"],
		labels["involved_object_name"],
		labels["involved_object_kind"],
		labels["reason"],
		labels["type"],
		labels["source"],
	}
	return strings.Join(parts, "\x00")
}

func cloneLabels(labels prometheus.Labels) prometheus.Labels {
	cloned := make(prometheus.Labels, len(labels))
	for key, value := range labels {
		cloned[key] = value
	}
	return cloned
}

func eventSource(ev *event.EnrichedEvent) string {
	host := strings.TrimSpace(ev.Source.Host)
	component := strings.TrimSpace(ev.Source.Component)

	if host == "" && component == "" {
		return ""
	}
	if host == "" {
		return component
	}
	if component == "" {
		return host
	}

	return host + "/" + component
}

func eventObservedAt(ev *event.EnrichedEvent) time.Time {
	if !ev.LastTimestamp.Time.IsZero() {
		return ev.LastTimestamp.Time
	}
	if !ev.CreationTimestamp.Time.IsZero() {
		return ev.CreationTimestamp.Time
	}
	return time.Now()
}
