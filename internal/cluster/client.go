package cluster

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewClientset() (*kubernetes.Clientset, error) {
	config, err := NewRESTConfig("")
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func NewRESTConfig(kubeconfig string) (*rest.Config, error) {
	if len(kubeconfig) > 0 {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	} else if err != rest.ErrNotInCluster {
		return nil, err
	}

	return clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
}
