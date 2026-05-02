package watcher

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ownkube/kubernetes-events-exporter/internal/event"
	"github.com/ownkube/kubernetes-events-exporter/internal/observability"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var startUpTime = time.Now()

type EventCallback func(event *event.EnrichedEvent)

type Watcher struct {
	wg                  sync.WaitGroup
	informers           []cache.SharedInformer
	stopper             chan struct{}
	objectMetadataCache MetadataResolver
	omitLookup          bool
	fn                  EventCallback
	maxEventAgeSeconds  time.Duration
	counters            *observability.Counters
	dynamicClient       *dynamic.DynamicClient
	clientset           *kubernetes.Clientset
}

func NewWatcher(config *rest.Config, namespace string, MaxEventAgeSeconds int64, counters *observability.Counters, fn EventCallback, omitLookup bool, cacheSize int) *Watcher {
	clientset := kubernetes.NewForConfigOrDie(config)
	namespaces := parseNamespaces(namespace)
	informerList := make([]cache.SharedInformer, 0, len(namespaces))
	for _, ns := range namespaces {
		factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithNamespace(ns))
		informerList = append(informerList, factory.Core().V1().Events().Informer())
	}

	w := &Watcher{
		informers:           informerList,
		stopper:             make(chan struct{}),
		objectMetadataCache: NewMetadataResolver(cacheSize),
		omitLookup:          omitLookup,
		fn:                  fn,
		maxEventAgeSeconds:  time.Second * time.Duration(MaxEventAgeSeconds),
		counters:            counters,
		dynamicClient:       dynamic.NewForConfigOrDie(config),
		clientset:           clientset,
	}

	for _, informer := range w.informers {
		informer.AddEventHandler(w)
		informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
			w.counters.WatchErrors.Inc()
		})
	}

	slog.Info("watcher namespaces configured", "namespaces", namespaces)

	return w
}

func parseNamespaces(namespaceCSV string) []string {
	if strings.TrimSpace(namespaceCSV) == "" {
		return []string{""}
	}

	seen := make(map[string]struct{})
	namespaces := make([]string, 0)

	for _, ns := range strings.Split(namespaceCSV, ",") {
		namespace := strings.TrimSpace(ns)
		if namespace == "" {
			continue
		}
		if _, exists := seen[namespace]; exists {
			continue
		}
		seen[namespace] = struct{}{}
		namespaces = append(namespaces, namespace)
	}

	if len(namespaces) == 0 {
		return []string{""}
	}

	return namespaces
}

func (e *Watcher) OnAdd(obj interface{}, isInInitialList bool) {
	ev := obj.(*corev1.Event)
	e.onEvent(ev)
}

func (e *Watcher) OnUpdate(oldObj, newObj interface{}) {
	ev := newObj.(*corev1.Event)
	e.onEvent(ev)
}

func (e *Watcher) isEventDiscarded(ev *corev1.Event) bool {
	eventAge, timeUsedForAge := getEventAge(ev)
	if eventAge > e.maxEventAgeSeconds {
		if timeUsedForAge.After(startUpTime) {
			slog.Warn("Event discarded as being older than maxEventAgeSeconds",
				"eventAge", eventAge.String(),
				"eventNamespace", ev.Namespace,
				"eventName", ev.Name,
				"eventReason", ev.Reason,
				"maxEventAgeSeconds", e.maxEventAgeSeconds.String())
			e.counters.EventsDiscarded.Inc()
		}
		return true
	}
	return false
}

// getEventAge returns the age of the event and the time used for age calculation.
// Uses the greater of creationTimestamp and lastTimestamp, because in practice
// lastTimestamp is not always greater than creationTimestamp despite event aggregation.
func getEventAge(ev *corev1.Event) (time.Duration, time.Time) {
	timestamp := ev.CreationTimestamp.Time

	if ev.LastTimestamp.Time.After(timestamp) {
		timestamp = ev.LastTimestamp.Time
	}

	return time.Since(timestamp), timestamp
}

func (e *Watcher) onEvent(ev *corev1.Event) {
	if e.isEventDiscarded(ev) {
		return
	}

	slog.Debug("Received event",
		"msg", ev.Message,
		"namespace", ev.Namespace,
		"reason", ev.Reason,
		"involvedObject", ev.InvolvedObject.Name)

	e.counters.EventsProcessed.Inc()

	enriched := &event.EnrichedEvent{
		Event: *ev.DeepCopy(),
	}
	enriched.Event.ManagedFields = nil

	if e.omitLookup {
		enriched.InvolvedObject.ObjectReference = *ev.InvolvedObject.DeepCopy()
	} else {
		objectMetadata, err := e.objectMetadataCache.GetObjectMetadata(&ev.InvolvedObject, e.clientset, e.dynamicClient, e.counters)
		if err != nil {
			if errors.IsNotFound(err) {
				enriched.InvolvedObject.Deleted = true
				slog.Error("Object not found, likely deleted", "error", err)
			} else {
				slog.Error("Failed to get object metadata", "error", err)
			}
			enriched.InvolvedObject.ObjectReference = *ev.InvolvedObject.DeepCopy()
		} else {
			enriched.InvolvedObject.Labels = objectMetadata.Labels
			enriched.InvolvedObject.Annotations = objectMetadata.Annotations
			enriched.InvolvedObject.OwnerReferences = objectMetadata.OwnerReferences
			enriched.InvolvedObject.ObjectReference = *ev.InvolvedObject.DeepCopy()
			enriched.InvolvedObject.Deleted = objectMetadata.Deleted
		}
	}

	e.fn(enriched)
}

func (e *Watcher) OnDelete(obj interface{}) {
}

func (e *Watcher) Start() {
	for _, informer := range e.informers {
		e.wg.Add(1)
		go func(i cache.SharedInformer) {
			defer e.wg.Done()
			i.Run(e.stopper)
		}(informer)
	}
}

func (e *Watcher) Stop() {
	close(e.stopper)
	e.wg.Wait()
}

func (e *Watcher) setStartUpTime(time time.Time) {
	startUpTime = time
}
