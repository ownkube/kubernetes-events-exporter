package observability

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ownkube/kubernetes-events-exporter/internal/buildinfo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
)

type Counters struct {
	EventsProcessed      prometheus.Counter
	EventsDiscarded      prometheus.Counter
	WatchErrors          prometheus.Counter
	SendErrors           prometheus.Counter
	BuildInfo            prometheus.GaugeFunc
	KubeApiReadCacheHits prometheus.Counter
	KubeApiReadRequests  prometheus.Counter
}

type promLogger struct{}

func (pl promLogger) Println(v ...interface{}) {
	slog.Error(fmt.Sprint(v...))
}

func (pl promLogger) Log(v ...interface{}) error {
	slog.Info(fmt.Sprint(v...))
	return nil
}

func Init(addr string, tlsConf string) {
	prometheus.MustRegister(collectors.NewBuildInfoCollector())

	promLogger := promLogger{}
	metricsPath := "/metrics"

	http.Handle(metricsPath, promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	))

	landingConfig := web.LandingConfig{
		Name:        "kubernetes-events-exporter",
		Description: "Export Kubernetes Events to multiple destinations with routing and filtering",
		Links: []web.LandingLinks{
			{
				Address: metricsPath,
				Text:    "Metrics",
			},
		},
	}
	landingPage, _ := web.NewLandingPage(landingConfig)
	http.Handle("/", landingPage)

	http.HandleFunc("/-/healthy", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})
	http.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	metricsServer := http.Server{
		ReadHeaderTimeout: 5 * time.Second}

	metricsFlags := web.FlagConfig{
		WebListenAddresses: &[]string{addr},
		WebSystemdSocket:   new(bool),
		WebConfigFile:      &tlsConf,
	}

	go web.ListenAndServe(&metricsServer, &metricsFlags, promLogger)
}

func NewCounters(namePrefix string) *Counters {
	return &Counters{
		BuildInfo: promauto.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: namePrefix + "build_info",
				Help: "A metric with a constant '1' value labeled by version, revision, branch, and goversion from which kubernetes-events-exporter was built.",
				ConstLabels: prometheus.Labels{
					"version":   buildinfo.Version,
					"revision":  buildinfo.Revision(),
					"goversion": buildinfo.GoVersion,
					"goos":      buildinfo.GoOS,
					"goarch":    buildinfo.GoArch,
				},
			},
			func() float64 { return 1 },
		),
		EventsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: namePrefix + "events_sent",
			Help: "The total number of events processed",
		}),
		EventsDiscarded: promauto.NewCounter(prometheus.CounterOpts{
			Name: namePrefix + "events_discarded",
			Help: "The total number of events discarded because of being older than the maxEventAgeSeconds specified",
		}),
		WatchErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: namePrefix + "watch_errors",
			Help: "The total number of errors received from the informer",
		}),
		SendErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: namePrefix + "send_event_errors",
			Help: "The total number of send event errors",
		}),
		KubeApiReadCacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: namePrefix + "kube_api_read_cache_hits",
			Help: "The total number of read requests served from cache when looking up object metadata",
		}),
		KubeApiReadRequests: promauto.NewCounter(prometheus.CounterOpts{
			Name: namePrefix + "kube_api_read_cache_misses",
			Help: "The total number of read requests served from kube-apiserver when looking up object metadata",
		}),
	}
}

func DestroyCounters(counters *Counters) {
	prometheus.Unregister(counters.EventsProcessed)
	prometheus.Unregister(counters.EventsDiscarded)
	prometheus.Unregister(counters.WatchErrors)
	prometheus.Unregister(counters.SendErrors)
	prometheus.Unregister(counters.BuildInfo)
	prometheus.Unregister(counters.KubeApiReadCacheHits)
	prometheus.Unregister(counters.KubeApiReadRequests)
	counters = nil
}
