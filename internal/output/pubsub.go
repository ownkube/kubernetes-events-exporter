package output

import (
	"context"
	"log/slog"

	"cloud.google.com/go/pubsub"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type PubsubConfig struct {
	GcloudProjectId string `yaml:"gcloud_project_id"`
	Topic           string `yaml:"topic"`
	CreateTopic     bool   `yaml:"create_topic"`
}

type PubsubHandler struct {
	cfg          *PubsubConfig
	pubsubClient *pubsub.Client
	topic        *pubsub.Topic
}

func NewPubsubHandler(cfg *PubsubConfig) (Handler, error) {
	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, cfg.GcloudProjectId)
	if err != nil {
		return nil, err
	}

	var topic *pubsub.Topic
	if cfg.CreateTopic {
		topic, err = pubsubClient.CreateTopic(context.Background(), cfg.Topic)
		if err != nil {
			return nil, err
		}
		slog.Info("pubsub: created topic", "topic", cfg.Topic)
	} else {
		topic = pubsubClient.Topic(cfg.Topic)
	}

	return &PubsubHandler{
		pubsubClient: pubsubClient,
		topic:        topic,
		cfg:          cfg,
	}, nil
}

func (ps *PubsubHandler) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	msg := &pubsub.Message{
		Data: ev.ToJSON(),
	}
	_, err := ps.topic.Publish(ctx, msg).Get(ctx)
	return err
}

func (ps *PubsubHandler) Close() {
	slog.Info("pubsub: Closing topic...")
	ps.pubsubClient.Close()
}
