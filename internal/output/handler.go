package output

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type Handler interface {
	Send(ctx context.Context, ev *event.EnrichedEvent) error
	Close()
}

type BatchHandler interface {
	Handler
	SendBatch([]*event.EnrichedEvent) error
}

type TLSConfig struct {
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
	ServerName         string `yaml:"serverName"`
	CaFile             string `yaml:"caFile"`
	KeyFile            string `yaml:"keyFile"`
	CertFile           string `yaml:"certFile"`
}

func SetupTLS(cfg *TLSConfig) (*tls.Config, error) {
	tlsClientConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         cfg.ServerName,
	}

	if len(cfg.CaFile) > 0 {
		readFile, err := os.ReadFile(cfg.CaFile)
		if err != nil {
			return nil, err
		}

		tlsClientConfig.RootCAs = x509.NewCertPool()
		tlsClientConfig.RootCAs.AppendCertsFromPEM(readFile)
	}

	if len(cfg.KeyFile) > 0 && len(cfg.CertFile) > 0 {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("could not read client certificate or key: %w", err)
		}
		tlsClientConfig.Certificates = append(tlsClientConfig.Certificates, cert)
	}
	if len(cfg.KeyFile) > 0 && len(cfg.CertFile) == 0 {
		return nil, errors.New("configured keyFile but forget certFile for client certificate authentication")
	}
	if len(cfg.KeyFile) == 0 && len(cfg.CertFile) > 0 {
		return nil, errors.New("configured certFile but forget keyFile for client certificate authentication")
	}
	return tlsClientConfig, nil
}
