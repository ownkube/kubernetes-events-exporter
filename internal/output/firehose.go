package output

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type FirehoseConfig struct {
	DeliveryStreamName string                 `yaml:"deliveryStreamName"`
	Region             string                 `yaml:"region"`
	Layout             map[string]interface{} `yaml:"layout"`
	// DeDot all labels and annotations in the event. For both the event and the involvedObject
	DeDot bool `yaml:"deDot"`
}

type FirehoseHandler struct {
	cfg *FirehoseConfig
	svc *firehose.Firehose
}

func NewFirehoseHandler(cfg *FirehoseConfig) (Handler, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.Region)},
	)
	if err != nil {
		return nil, err
	}

	return &FirehoseHandler{
		cfg: cfg,
		svc: firehose.New(sess),
	}, nil
}

func (f *FirehoseHandler) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	var toSend []byte

	if f.cfg.DeDot {
		de := ev.DeDot()
		ev = &de
	}

	if f.cfg.Layout != nil {
		res, err := ConvertLayoutTemplate(f.cfg.Layout, ev)
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

	_, err := f.svc.PutRecord(&firehose.PutRecordInput{
		Record: &firehose.Record{
			Data: toSend,
		},
		DeliveryStreamName: aws.String(f.cfg.DeliveryStreamName),
	})

	return err
}

func (f *FirehoseHandler) Close() {
	// No-op
}
