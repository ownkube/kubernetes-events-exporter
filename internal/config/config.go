package config

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/ownkube/kubernetes-events-exporter/internal/cluster"
	"github.com/ownkube/kubernetes-events-exporter/internal/output"
	"github.com/ownkube/kubernetes-events-exporter/internal/routing"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/rest"
)

const (
	DefaultCacheSize = 1024
)

type AppConfig struct {
	LogLevel           string               `yaml:"logLevel"`
	LogFormat          string               `yaml:"logFormat"`
	ThrottlePeriod     int64                `yaml:"throttlePeriod"`
	MaxEventAgeSeconds int64                `yaml:"maxEventAgeSeconds"`
	ClusterName        string               `yaml:"clusterName,omitempty"`
	ClusterEnvironment string               `yaml:"clusterEnvironment,omitempty"`
	Namespace          string               `yaml:"namespace"`
	LeaderElection     cluster.LeaderConfig `yaml:"leaderElection"`
	Route              routing.RoutingRule  `yaml:"route"`
	Receivers          []output.OutputConfig `yaml:"receivers"`
	KubeQPS            float32              `yaml:"kubeQPS,omitempty"`
	KubeBurst          int                  `yaml:"kubeBurst,omitempty"`
	MetricsNamePrefix  string               `yaml:"metricsNamePrefix,omitempty"`
	OmitLookup         bool                 `yaml:"omitLookup,omitempty"`
	CacheSize          int                  `yaml:"cacheSize,omitempty"`
}

func (c *AppConfig) SetDefaults() {
	if c.CacheSize == 0 {
		c.CacheSize = DefaultCacheSize
		slog.Debug("setting config.cacheSize=1024 (default)")
	}

	if c.KubeBurst == 0 {
		c.KubeBurst = rest.DefaultBurst
		slog.Debug(fmt.Sprintf("setting config.kubeBurst=%d (default)", rest.DefaultBurst))
	}

	if c.KubeQPS == 0 {
		c.KubeQPS = rest.DefaultQPS
		slog.Debug(fmt.Sprintf("setting config.kubeQPS=%.2f (default)", rest.DefaultQPS))
	}
}

func (c *AppConfig) Validate() error {
	if err := c.validateDefaults(); err != nil {
		return err
	}
	if err := c.validateMetricsNamePrefix(); err != nil {
		return err
	}
	return nil
}

func (c *AppConfig) validateDefaults() error {
	if err := c.validateMaxEventAgeSeconds(); err != nil {
		return err
	}
	return nil
}

func (c *AppConfig) validateMaxEventAgeSeconds() error {
	if c.ThrottlePeriod == 0 && c.MaxEventAgeSeconds == 0 {
		c.MaxEventAgeSeconds = 5
		slog.Info("setting config.maxEventAgeSeconds=5 (default)")
	} else if c.ThrottlePeriod != 0 && c.MaxEventAgeSeconds != 0 {
		slog.Error("cannot set both throttlePeriod (depricated) and MaxEventAgeSeconds")
		return errors.New("validateMaxEventAgeSeconds failed")
	} else if c.ThrottlePeriod != 0 {
		logValue := strconv.FormatInt(c.ThrottlePeriod, 10)
		slog.Info("config.maxEventAgeSeconds=" + logValue)
		slog.Warn("config.throttlePeriod is depricated, consider using config.maxEventAgeSeconds instead")
		c.MaxEventAgeSeconds = c.ThrottlePeriod
	} else {
		logValue := strconv.FormatInt(c.MaxEventAgeSeconds, 10)
		slog.Info("config.maxEventAgeSeconds=" + logValue)
	}
	return nil
}

func (c *AppConfig) validateMetricsNamePrefix() error {
	if c.MetricsNamePrefix != "" {
		checkResult, err := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9_:]*_$", c.MetricsNamePrefix)
		if err != nil {
			return err
		}
		if checkResult {
			slog.Info("config.metricsNamePrefix='" + c.MetricsNamePrefix + "'")
		} else {
			slog.Error("config.metricsNamePrefix should match the regex: ^[a-zA-Z][a-zA-Z0-9_:]*_$")
			return errors.New("validateMetricsNamePrefix failed")
		}
	} else {
		slog.Warn("metrics name prefix is empty, setting config.metricsNamePrefix='event_exporter_' is recommended")
	}
	return nil
}

func ParseFromBytes(configBytes []byte) (AppConfig, error) {
	var config AppConfig
	err := yaml.Unmarshal(configBytes, &config)
	if err != nil {
		errMsg := err.Error()
		errLines := strings.Split(errMsg, "\n")
		if len(errLines) > 0 {
			errMsg = errLines[0]
		}
		for _, line := range errLines {
			if strings.Contains(line, "> ") {
				errMsg += ": [ line " + line + "]"
				if strings.Contains(line, "{{") {
					errMsg += ": " + "Need to wrap values with special characters in quotes"
				}
			}
		}
		errMsg = "Cannot parse config to YAML: " + errMsg
		return AppConfig{}, errors.New(errMsg)
	}

	return config, nil
}
