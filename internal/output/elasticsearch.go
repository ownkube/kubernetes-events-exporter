package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type ElasticsearchConfig struct {
	// Connection specific
	Hosts    []string          `yaml:"hosts"`
	Username string            `yaml:"username"`
	Password string            `yaml:"password"`
	CloudID  string            `yaml:"cloudID"`
	APIKey   string            `yaml:"apiKey"`
	Headers  map[string]string `yaml:"headers"`
	// Indexing preferences
	UseEventID bool `yaml:"useEventID"`
	// DeDot all labels and annotations in the event. For both the event and the involvedObject
	DeDot       bool                   `yaml:"deDot"`
	Index       string                 `yaml:"index"`
	IndexFormat string                 `yaml:"indexFormat"`
	Type        string                 `yaml:"type"`
	TLS         TLSConfig              `yaml:"tls"`
	Layout      map[string]interface{} `yaml:"layout"`
}

func NewElasticsearchHandler(cfg *ElasticsearchConfig) (*Elasticsearch, error) {
	tlsClientConfig, err := SetupTLS(&cfg.TLS)
	if err != nil {
		return nil, fmt.Errorf("failed to setup TLS: %w", err)
	}

	var header = http.Header{}
	if len(cfg.Headers) > 0 {
		for k, v := range cfg.Headers {
			header.Add(k, v)
		}
	}

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.Hosts,
		Username:  cfg.Username,
		Password:  cfg.Password,
		Header:    header,
		CloudID:   cfg.CloudID,
		APIKey:    cfg.APIKey,
		Transport: &http.Transport{
			TLSClientConfig: tlsClientConfig,
		},
	})
	if err != nil {
		return nil, err
	}

	return &Elasticsearch{
		client: client,
		cfg:    cfg,
	}, nil
}

type Elasticsearch struct {
	client *elasticsearch.Client
	cfg    *ElasticsearchConfig
}

var esIndexRegex = regexp.MustCompile(`(?s){(.*)}`)

func formatIndexName(pattern string, when time.Time) string {
	m := esIndexRegex.FindAllStringSubmatchIndex(pattern, -1)
	current := 0
	var builder strings.Builder

	for i := 0; i < len(m); i++ {
		pair := m[i]
		builder.WriteString(pattern[current:pair[0]])
		builder.WriteString(when.Format(pattern[pair[0]+1 : pair[1]-1]))
		current = pair[1]
	}

	builder.WriteString(pattern[current:])
	return builder.String()
}

func (e *Elasticsearch) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	var toSend []byte

	if e.cfg.DeDot {
		de := ev.DeDot()
		ev = &de
	}
	if e.cfg.Layout != nil {
		res, err := ConvertLayoutTemplate(e.cfg.Layout, ev)
		if err != nil {
			return err
		}

		toSend, err = json.Marshal(res)
		if err != nil {
			return err
		}
	} else {
		toSend = ev.ToJSON()
	}

	var index string
	if len(e.cfg.IndexFormat) > 0 {
		now := time.Now()
		index = formatIndexName(e.cfg.IndexFormat, now)
	} else {
		index = e.cfg.Index
	}

	req := esapi.IndexRequest{
		Body:  bytes.NewBuffer(toSend),
		Index: index,
	}

	if e.cfg.UseEventID {
		req.DocumentID = string(ev.UID)
	}

	resp, err := req.Do(ctx, e.client)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode > 399 {
		rb, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		slog.Error("Indexing failed", "response", string(rb))
	}
	return nil
}

func (e *Elasticsearch) Close() {
	// No-op
}
