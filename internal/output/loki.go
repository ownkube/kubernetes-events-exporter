package output

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type promtailStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

type LokiMsg struct {
	Streams []promtailStream `json:"streams"`
}

type LokiConfig struct {
	Layout       map[string]interface{} `yaml:"layout"`
	StreamLabels map[string]string      `yaml:"streamLabels"`
	TLS          TLSConfig              `yaml:"tls"`
	URL          string                 `yaml:"url"`
	Headers      map[string]string      `yaml:"headers"`
	Username     string                 `yaml:"username"`
	Password     string                 `yaml:"password"`
}

type Loki struct {
	cfg       *LokiConfig
	transport *http.Transport
}

func NewLokiHandler(cfg *LokiConfig) (Handler, error) {
	tlsClientConfig, err := SetupTLS(&cfg.TLS)
	if err != nil {
		return nil, fmt.Errorf("failed to setup TLS: %w", err)
	}
	return &Loki{cfg: cfg, transport: &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsClientConfig,
	}}, nil
}

func generateTimestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10) + "000000000"
}

func renderStreamLabels(labels map[string]string, ev *event.EnrichedEvent) (map[string]string, error) {
	result := make(map[string]string, len(labels))
	for key, value := range labels {
		rendered, err := RenderTemplate(ev, value)
		if err != nil {
			return nil, fmt.Errorf("failed to render stream label %q: %w", key, err)
		}
		result[key] = rendered
	}
	return result, nil
}

func (l *Loki) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	eventBody, err := SerializeEventWithLayout(l.cfg.Layout, ev)
	if err != nil {
		return err
	}
	streamLabels, err := renderStreamLabels(l.cfg.StreamLabels, ev)
	if err != nil {
		return err
	}
	timestamp := generateTimestamp()
	a := LokiMsg{
		Streams: []promtailStream{{
			Stream: streamLabels,
			Values: [][]string{{timestamp, string(eventBody)}},
		}},
	}
	reqBody, err := json.Marshal(a)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, l.cfg.URL, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	if l.cfg.Username != "" && l.cfg.Password != "" {
		req.SetBasicAuth(l.cfg.Username, l.cfg.Password)
	}

	for k, v := range l.cfg.Headers {
		realValue, err := RenderTemplate(ev, v)
		if err != nil {
			slog.Debug("parse template failed", "error", err, "template", v)
			req.Header.Add(k, v)
		} else {
			slog.Debug("request header", "key", k, "value", realValue)
			req.Header.Add(k, realValue)
		}
	}

	client := &http.Client{Transport: l.transport}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return errors.New("not successfull (2xx) response: " + string(body))
	}

	return nil
}

func (l *Loki) Close() {
	l.transport.CloseIdleConnections()
}
