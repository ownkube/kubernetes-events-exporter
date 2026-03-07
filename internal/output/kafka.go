package output

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/IBM/sarama"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
	"github.com/xdg-go/scram"
)

// KafkaConfig is the Kafka producer configuration
type KafkaConfig struct {
	Topic            string                 `yaml:"topic"`
	Brokers          []string               `yaml:"brokers"`
	Layout           map[string]interface{} `yaml:"layout"`
	ClientId         string                 `yaml:"clientId"`
	CompressionCodec string                 `yaml:"compressionCodec" default:"none"`
	Version          string                 `yaml:"version"`
	TLS              struct {
		Enable             bool   `yaml:"enable"`
		CaFile             string `yaml:"caFile"`
		CertFile           string `yaml:"certFile"`
		KeyFile            string `yaml:"keyFile"`
		InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
	} `yaml:"tls"`
	SASL struct {
		Enable    bool   `yaml:"enable"`
		Username  string `yaml:"username"`
		Password  string `yaml:"password"`
		Mechanism string `yaml:"mechanism" default:"plain"`
	} `yaml:"sasl"`
	KafkaEncode Avro `yaml:"avro"`
}

// KafkaEncoder is an interface type for adding an
// encoder to the kafka data pipeline
type KafkaEncoder interface {
	encode([]byte) ([]byte, error)
}

// KafkaHandler is a handler that sends events to a Kafka topic
type KafkaHandler struct {
	producer sarama.SyncProducer
	cfg      *KafkaConfig
	encoder  KafkaEncoder
}

var CompressionCodecs = map[string]sarama.CompressionCodec{
	"none":   sarama.CompressionNone,
	"snappy": sarama.CompressionSnappy,
	"gzip":   sarama.CompressionGZIP,
	"lz4":    sarama.CompressionLZ4,
	"zstd":   sarama.CompressionZSTD,
}

func NewKafkaHandler(cfg *KafkaConfig) (Handler, error) {
	var avro KafkaEncoder
	producer, err := createSaramaProducer(cfg)
	if err != nil {
		return nil, err
	}

	slog.Info("kafka: Producer initialized", "topic", cfg.Topic, "brokers", cfg.Brokers)
	if len(cfg.KafkaEncode.SchemaID) > 0 {
		var err error
		avro, err = NewAvroEncoder(cfg.KafkaEncode.SchemaID, cfg.KafkaEncode.Schema)
		if err != nil {
			return nil, err
		}
		slog.Info("kafka: Producer using avro encoding", "schemaID", cfg.KafkaEncode.SchemaID)
	}

	return &KafkaHandler{
		producer: producer,
		cfg:      cfg,
		encoder:  avro,
	}, nil
}

// Send an event to Kafka synchronously
func (k *KafkaHandler) Send(ctx context.Context, ev *event.EnrichedEvent) error {
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
	} else if len(k.cfg.KafkaEncode.SchemaID) > 0 {
		var err error
		toSend, err = k.encoder.encode(ev.ToJSON())
		if err != nil {
			return err
		}
	} else {
		toSend = ev.ToJSON()
	}

	_, _, err := k.producer.SendMessage(&sarama.ProducerMessage{
		Topic: k.cfg.Topic,
		Key:   sarama.StringEncoder(string(ev.UID)),
		Value: sarama.ByteEncoder(toSend),
	})

	return err
}

// Close the Kafka producer
func (k *KafkaHandler) Close() {
	slog.Info("kafka: Closing producer...")

	if err := k.producer.Close(); err != nil {
		slog.Error("Failed to shut down the Kafka producer cleanly", "error", err)
	} else {
		slog.Info("kafka: Closed producer")
	}
}

func createSaramaProducer(cfg *KafkaConfig) (sarama.SyncProducer, error) {
	// Default Sarama config
	saramaConfig := sarama.NewConfig()
	if cfg.Version != "" {
		version, err := sarama.ParseKafkaVersion(cfg.Version)
		if err != nil {
			return nil, err
		}
		saramaConfig.Version = version
	} else {
		saramaConfig.Version = sarama.MaxVersion
	}
	saramaConfig.Metadata.Full = true
	saramaConfig.ClientID = cfg.ClientId

	// Necessary for SyncProducer
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	if _, ok := CompressionCodecs[cfg.CompressionCodec]; ok {
		saramaConfig.Producer.Compression = CompressionCodecs[cfg.CompressionCodec]
	}

	// TLS Client auth override
	if cfg.TLS.Enable {
		caCert, err := os.ReadFile(cfg.TLS.CaFile)
		if err != nil {
			return nil, fmt.Errorf("error loading ca file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		saramaConfig.Net.TLS.Enable = true
		saramaConfig.Net.TLS.Config = &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		}

		if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
			if err != nil {
				return nil, err
			}

			saramaConfig.Net.TLS.Config.Certificates = []tls.Certificate{cert}
		}
	}

	// SASL Client auth
	if cfg.SASL.Enable {
		saramaConfig.Net.SASL.Enable = true
		saramaConfig.Net.SASL.User = cfg.SASL.Username
		saramaConfig.Net.SASL.Password = cfg.SASL.Password
		if cfg.SASL.Mechanism == "sha512" {
			saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		} else if cfg.SASL.Mechanism == "sha256" {
			saramaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		} else if cfg.SASL.Mechanism == "plain" || cfg.SASL.Mechanism == "" {
			saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		} else {
			return nil, fmt.Errorf("invalid scram sha mechanism: %s: can be one of 'sha256', 'sha512' or 'plain'", cfg.SASL.Mechanism)
		}
	}

	// Build producer
	producer, err := sarama.NewSyncProducer(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, err
	}

	return producer, nil
}

var (
	SHA256 scram.HashGeneratorFcn = sha256.New
	SHA512 scram.HashGeneratorFcn = sha512.New
)

type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
