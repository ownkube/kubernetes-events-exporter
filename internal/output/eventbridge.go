package output

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type EventBridgeConfig struct {
	DetailType   string                 `yaml:"detailType"`
	Details      map[string]interface{} `yaml:"details"`
	Source       string                 `yaml:"source"`
	EventBusName string                 `yaml:"eventBusName"`
	Region       string                 `yaml:"region"`
}

type EventBridgeHandler struct {
	cfg *EventBridgeConfig
	svc *eventbridge.EventBridge
}

func NewEventBridgeHandler(cfg *EventBridgeConfig) (Handler, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.Region),
		Retryer: client.DefaultRetryer{
			NumMaxRetries:    client.DefaultRetryerMaxNumRetries,
			MinRetryDelay:    client.DefaultRetryerMinRetryDelay,
			MinThrottleDelay: client.DefaultRetryerMinThrottleDelay,
			MaxRetryDelay:    client.DefaultRetryerMaxRetryDelay,
			MaxThrottleDelay: client.DefaultRetryerMaxThrottleDelay,
		},
	})
	if err != nil {
		return nil, err
	}

	svc := eventbridge.New(sess)
	return &EventBridgeHandler{
		cfg: cfg,
		svc: svc,
	}, nil
}

func (s *EventBridgeHandler) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	slog.Info("Sending event to EventBridge")
	var toSend string
	if s.cfg.Details != nil {
		res, err := ConvertLayoutTemplate(s.cfg.Details, ev)
		if err != nil {
			return err
		}

		b, err := json.Marshal(res)
		toSend = string(b)
		if err != nil {
			return err
		}
	} else {
		toSend = string(ev.ToJSON())
	}
	tym := time.Now()
	inputRequest := eventbridge.PutEventsRequestEntry{
		Detail:       &toSend,
		DetailType:   &s.cfg.DetailType,
		Time:         &tym,
		Source:       &s.cfg.Source,
		EventBusName: &s.cfg.EventBusName,
	}
	slog.Info("Request", "InputEvent", inputRequest.String())

	req, _ := s.svc.PutEventsRequest(&eventbridge.PutEventsInput{Entries: []*eventbridge.PutEventsRequestEntry{&inputRequest}})
	// TODO: Retry failed events
	err := req.Send()
	if err != nil {
		slog.Error("EventBridge Error", "error", err)
		return err
	}
	return nil
}

func (s *EventBridgeHandler) Close() {
}
