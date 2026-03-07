package output

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type KinesisConfig struct {
	StreamName string                 `yaml:"streamName"`
	Region     string                 `yaml:"region"`
	Layout     map[string]interface{} `yaml:"layout"`
}

type KinesisHandler struct {
	cfg *KinesisConfig
	svc *kinesis.Kinesis
}

func NewKinesisHandler(cfg *KinesisConfig) (Handler, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.Region)},
	)
	if err != nil {
		return nil, err
	}

	return &KinesisHandler{
		cfg: cfg,
		svc: kinesis.New(sess),
	}, nil
}

func (k *KinesisHandler) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	var toSend []byte

	if k.cfg.Layout != nil {
		res, err := ConvertLayoutTemplate(k.cfg.Layout, ev)
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

	_, err := k.svc.PutRecord(&kinesis.PutRecordInput{
		Data:         toSend,
		PartitionKey: aws.String(string(ev.UID)),
		StreamName:   aws.String(k.cfg.StreamName),
	})

	return err
}

func (k *KinesisHandler) Close() {
	// No-op
}
