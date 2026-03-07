package output

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type SNSConfig struct {
	TopicARN       string                 `yaml:"topicARN"`
	Region         string                 `yaml:"region"`
	Layout         map[string]interface{} `yaml:"layout"`
	MessageGroupId string                 `yaml:"messageGroupId"`
}

type SNSHandler struct {
	cfg *SNSConfig
	svc *sns.SNS
}

func NewSNSHandler(cfg *SNSConfig) (Handler, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.Region)},
	)
	if err != nil {
		return nil, err
	}

	svc := sns.New(sess)
	return &SNSHandler{
		cfg: cfg,
		svc: svc,
	}, nil
}

func (s *SNSHandler) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	toSend, e := SerializeEventWithLayout(s.cfg.Layout, ev)
	if e != nil {
		return e
	}

	input := &sns.PublishInput{
		Message:  aws.String(string(toSend)),
		TopicArn: aws.String(s.cfg.TopicARN),
	}
	if s.cfg.MessageGroupId != "" {
		input.MessageGroupId = aws.String(s.cfg.MessageGroupId)
	}

	_, err := s.svc.PublishWithContext(ctx, input)

	return err
}

func (s *SNSHandler) Close() {
}
