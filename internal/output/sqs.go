package output

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type SQSConfig struct {
	QueueName string                 `yaml:"queueName"`
	Region    string                 `yaml:"region"`
	Layout    map[string]interface{} `yaml:"layout"`
}

type SQSHandler struct {
	cfg      *SQSConfig
	svc      *sqs.SQS
	queueURL string
}

func NewSQSHandler(cfg *SQSConfig) (Handler, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.Region)},
	)
	if err != nil {
		return nil, err
	}

	svc := sqs.New(sess)
	out, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &cfg.QueueName,
	})

	if err != nil {
		return nil, err
	}

	return &SQSHandler{
		cfg:      cfg,
		svc:      svc,
		queueURL: *out.QueueUrl,
	}, nil
}

func (s *SQSHandler) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	toSend, e := SerializeEventWithLayout(s.cfg.Layout, ev)
	if e != nil {
		return e
	}

	_, err := s.svc.SendMessageWithContext(ctx, &sqs.SendMessageInput{
		MessageBody: aws.String(string(toSend)),
		QueueUrl:    &s.queueURL,
	})

	return err
}

func (s *SQSHandler) Close() {
	// No-op
}
