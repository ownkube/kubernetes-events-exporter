package pipeline

import (
	"testing"

	"github.com/ownkube/kubernetes-events-exporter/internal/config"
	"github.com/ownkube/kubernetes-events-exporter/internal/output"
	"github.com/stretchr/testify/require"
)

func TestApplyReceiverDefaultsInheritsPrometheusMetricExpiration(t *testing.T) {
	cfg := &config.AppConfig{
		MaxEventAgeSeconds: 90,
		Receivers: []output.OutputConfig{
			{
				Name: "prom-default",
				Prometheus: &output.PrometheusConfig{
					ReasonFilter: map[string][]string{"Pod": {"Started"}},
				},
			},
			{
				Name: "prom-explicit",
				Prometheus: &output.PrometheusConfig{
					MetricExpirationSeconds: 5,
					ReasonFilter:            map[string][]string{"Pod": {"Started"}},
				},
			},
		},
	}

	applyReceiverDefaults(cfg)

	require.Equal(t, int64(90), cfg.Receivers[0].Prometheus.MetricExpirationSeconds)
	require.Equal(t, int64(5), cfg.Receivers[1].Prometheus.MetricExpirationSeconds)
}
