package watcher

import (
	"context"
	"strings"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/ownkube/kubernetes-events-exporter/internal/observability"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
)

type MetadataResolver interface {
	GetObjectMetadata(reference *v1.ObjectReference, clientset *kubernetes.Clientset, dynClient dynamic.Interface, counters *observability.Counters) (ObjectMetadata, error)
}

type MetadataCache struct {
	cache *lru.Cache[string, ObjectMetadata]
}

var _ MetadataResolver = &MetadataCache{}

type ObjectMetadata struct {
	Annotations     map[string]string
	Labels          map[string]string
	OwnerReferences []metav1.OwnerReference
	Deleted         bool
}

func NewMetadataResolver(size int) MetadataResolver {
	cache, err := lru.New[string, ObjectMetadata](size)
	if err != nil {
		panic("cannot init cache: " + err.Error())
	}

	var o MetadataResolver = &MetadataCache{
		cache: cache,
	}

	return o
}

func (o *MetadataCache) GetObjectMetadata(reference *v1.ObjectReference, clientset *kubernetes.Clientset, dynClient dynamic.Interface, counters *observability.Counters) (ObjectMetadata, error) {
	// UID and ResourceVersion are not always present, so check before using as cache key.
	var cacheKey string
	if reference.UID != "" && reference.ResourceVersion != "" {
		cacheKey = strings.Join([]string{string(reference.UID), reference.ResourceVersion}, "/")
		if val, ok := o.cache.Get(cacheKey); ok {
			counters.KubeApiReadCacheHits.Inc()
			return val, nil
		}
	}

	var group, version string
	s := strings.Split(reference.APIVersion, "/")
	if len(s) == 1 {
		group = ""
		version = s[0]
	} else {
		group = s[0]
		version = s[1]
	}

	gk := schema.GroupKind{Group: group, Kind: reference.Kind}

	groupResources, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return ObjectMetadata{}, err
	}

	rm := restmapper.NewDiscoveryRESTMapper(groupResources)
	mapping, err := rm.RESTMapping(gk, version)
	if err != nil {
		return ObjectMetadata{}, err
	}

	item, err := dynClient.
		Resource(mapping.Resource).
		Namespace(reference.Namespace).
		Get(context.Background(), reference.Name, metav1.GetOptions{})

	counters.KubeApiReadRequests.Inc()

	if err != nil {
		return ObjectMetadata{}, err
	}

	objectMetadata := ObjectMetadata{
		OwnerReferences: item.GetOwnerReferences(),
		Labels:          item.GetLabels(),
		Annotations:     item.GetAnnotations(),
	}

	if item.GetDeletionTimestamp() != nil {
		objectMetadata.Deleted = true
	}

	if cacheKey != "" {
		o.cache.Add(cacheKey, objectMetadata)
	}
	return objectMetadata, nil
}
