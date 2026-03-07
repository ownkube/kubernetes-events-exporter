package cluster

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type LeaderConfig struct {
	Enabled          bool   `yaml:"enabled"`
	LeaderElectionID string `yaml:"leaderElectionID"`
}

const (
	inClusterNamespacePath  = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	defaultLeaderElectionID = "kubernetes-events-exporter"
	defaultNamespace        = "default"
	defaultLeaseDuration    = 15 * time.Second
	defaultRenewDeadline    = 10 * time.Second
	defaultRetryPeriod      = 2 * time.Second
)

func GetLeaseDuration() time.Duration {
	return defaultLeaseDuration
}

func newResourceLock(config *rest.Config, leaderElectionID string) (resourcelock.Interface, error) {
	if leaderElectionID == "" {
		leaderElectionID = defaultLeaderElectionID
	}

	leaderElectionNamespace, err := getInClusterNamespace()
	if err != nil {
		leaderElectionNamespace = defaultNamespace
	}

	id, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	id = id + "_" + string(uuid.NewUUID())

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return resourcelock.New(resourcelock.LeasesResourceLock,
		leaderElectionNamespace,
		leaderElectionID,
		client.CoreV1(),
		client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		})
}

func getInClusterNamespace() (string, error) {
	_, err := os.Stat(inClusterNamespacePath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("not running in-cluster, please specify leaderElectionIDspace")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
}

func NewLeaderElector(leaderElectionID string, config *rest.Config, startFunc func(context.Context), stopFunc func(), newLeaderFunc func(string)) (*leaderelection.LeaderElector, error) {
	resourceLock, err := newResourceLock(config, leaderElectionID)
	if err != nil {
		return &leaderelection.LeaderElector{}, err
	}

	l, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          resourceLock,
		LeaseDuration: defaultLeaseDuration,
		RenewDeadline: defaultRenewDeadline,
		RetryPeriod:   defaultRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: startFunc,
			OnStoppedLeading: stopFunc,
			OnNewLeader:      newLeaderFunc,
		},
	})
	return l, err
}
