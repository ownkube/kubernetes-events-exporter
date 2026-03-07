package output

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ownkube/kubernetes-events-exporter/internal/event"
	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusConfig struct {
	EventsMetricsNamePrefix string              `yaml:"eventsMetricsNamePrefix"`
	ReasonFilter            map[string][]string `yaml:"reasonFilter"`
}

type PrometheusSink struct {
	cfg           *PrometheusConfig
	kinds         []string
	metricsByKind map[string]*prometheus.GaugeVec
}

func NewPrometheusHandler(config *PrometheusConfig) (Handler, error) {
	if config.EventsMetricsNamePrefix == "" {
		config.EventsMetricsNamePrefix = "event_exporter_"
	}

	metricsByKind := make(map[string]*prometheus.GaugeVec)

	slog.Info("Initializing Prometheus sink")
	kinds := make([]string, 0, len(config.ReasonFilter))
	for kind := range config.ReasonFilter {
		kinds = append(kinds, kind)
		metricName := config.EventsMetricsNamePrefix + strings.ToLower(kind) + "_event_count"
		metricLabels := []string{strings.ToLower(kind), "namespace", "reason"}
		gaugeVec := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricName,
				Help: "Event counts for " + kind + " resources.",
			}, metricLabels)
		prometheus.MustRegister(gaugeVec)
		metricsByKind[kind] = gaugeVec

		slog.Info("Created Prometheus metric",
			"kind", kind,
			"reasons", config.ReasonFilter[kind],
			"labels", metricLabels)
	}

	return &PrometheusSink{
		cfg:           config,
		kinds:         kinds,
		metricsByKind: metricsByKind,
	}, nil
}

func (p *PrometheusSink) Send(_ context.Context, ev *event.EnrichedEvent) error {
	kind := ev.InvolvedObject.Kind
	gaugeVec, ok := p.metricsByKind[kind]
	if !ok {
		return nil
	}

	for _, reason := range p.cfg.ReasonFilter[kind] {
		labels := prometheus.Labels{
			strings.ToLower(kind): ev.InvolvedObject.Name,
			"namespace":           ev.InvolvedObject.Namespace,
			"reason":              reason,
		}
		if ev.Reason == reason {
			gaugeVec.With(labels).Set(float64(ev.Count))
		} else {
			gaugeVec.Delete(labels)
		}
	}

	return nil
}

func (p *PrometheusSink) Close() {
	// No-op
}
