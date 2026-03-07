package output

import "errors"

// OutputConfig defines a named output destination with exactly one handler type configured.
type OutputConfig struct {
	Name          string               `yaml:"name"`
	InMemory      *InMemoryConfig      `yaml:"inMemory"`
	Webhook       *WebhookConfig       `yaml:"webhook"`
	File          *FileConfig          `yaml:"file"`
	Syslog        *SyslogConfig        `yaml:"syslog"`
	Stdout        *StdoutConfig        `yaml:"stdout"`
	Elasticsearch *ElasticsearchConfig `yaml:"elasticsearch"`
	Kinesis       *KinesisConfig       `yaml:"kinesis"`
	Firehose      *FirehoseConfig      `yaml:"firehose"`
	OpenSearch    *OpenSearchConfig    `yaml:"opensearch"`
	Opsgenie      *OpsgenieConfig      `yaml:"opsgenie"`
	Loki          *LokiConfig          `yaml:"loki"`
	SQS           *SQSConfig           `yaml:"sqs"`
	SNS           *SNSConfig           `yaml:"sns"`
	Slack         *SlackConfig         `yaml:"slack"`
	Kafka         *KafkaConfig         `yaml:"kafka"`
	Pubsub        *PubsubConfig        `yaml:"pubsub"`
	Opscenter     *OpsCenterConfig     `yaml:"opscenter"`
	Teams         *TeamsConfig         `yaml:"teams"`
	BigQuery      *BigQueryConfig      `yaml:"bigquery"`
	EventBridge   *EventBridgeConfig   `yaml:"eventbridge"`
	Pipe          *PipeConfig          `yaml:"pipe"`
	Prometheus    *PrometheusConfig    `yaml:"prometheus"`
}

func (r *OutputConfig) Validate() error {
	return nil
}

func (r *OutputConfig) GetHandler() (Handler, error) {
	if r.InMemory != nil {
		// This reference is used for test purposes to count the events in the handler.
		// It should not be used in production since it will only cause memory leak and (b)OOM
		handler := &InMemory{Config: r.InMemory}
		r.InMemory.Ref = handler
		return handler, nil
	}

	if r.Pipe != nil {
		return NewPipeHandler(r.Pipe)
	}

	if r.Webhook != nil {
		return NewWebhookHandler(r.Webhook)
	}

	if r.File != nil {
		return NewFileHandler(r.File)
	}

	if r.Syslog != nil {
		return NewSyslogHandler(r.Syslog)
	}

	if r.Stdout != nil {
		return NewStdoutHandler(r.Stdout)
	}

	if r.Elasticsearch != nil {
		return NewElasticsearchHandler(r.Elasticsearch)
	}

	if r.Kinesis != nil {
		return NewKinesisHandler(r.Kinesis)
	}

	if r.Firehose != nil {
		return NewFirehoseHandler(r.Firehose)
	}

	if r.OpenSearch != nil {
		return NewOpenSearchHandler(r.OpenSearch)
	}

	if r.Opsgenie != nil {
		return NewOpsgenieHandler(r.Opsgenie)
	}

	if r.SQS != nil {
		return NewSQSHandler(r.SQS)
	}

	if r.SNS != nil {
		return NewSNSHandler(r.SNS)
	}

	if r.Slack != nil {
		return NewSlackHandler(r.Slack)
	}

	if r.Kafka != nil {
		return NewKafkaHandler(r.Kafka)
	}

	if r.Pubsub != nil {
		return NewPubsubHandler(r.Pubsub)
	}

	if r.Opscenter != nil {
		return NewOpsCenterHandler(r.Opscenter)
	}

	if r.Teams != nil {
		return NewTeamsHandler(r.Teams)
	}

	if r.BigQuery != nil {
		return NewBigQueryHandler(r.BigQuery)
	}

	if r.EventBridge != nil {
		return NewEventBridgeHandler(r.EventBridge)
	}

	if r.Loki != nil {
		return NewLokiHandler(r.Loki)
	}

	if r.Prometheus != nil {
		return NewPrometheusHandler(r.Prometheus)
	}

	return nil, errors.New("unknown sink")
}
