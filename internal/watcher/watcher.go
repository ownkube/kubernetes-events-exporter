package watcher

import (
	"log/slog"
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
	informer            cache.SharedInformer
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
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithNamespace(namespace))
	informer := factory.Core().V1().Events().Informer()

	w := &Watcher{
		informer:            informer,
		stopper:             make(chan struct{}),
		objectMetadataCache: NewMetadataResolver(cacheSize),
		omitLookup:          omitLookup,
		fn:                  fn,
		maxEventAgeSeconds:  time.Second * time.Duration(MaxEventAgeSeconds),
		counters:            counters,
		dynamicClient:       dynamic.NewForConfigOrDie(config),
		clientset:           clientset,
	}

	informer.AddEventHandler(w)
	informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		w.counters.WatchErrors.Inc()
	})

	return w
}

func (e *Watcher) OnAdd(obj interface{}) {
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
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.informer.Run(e.stopper)
	}()
}

func (e *Watcher) Stop() {
	close(e.stopper)
	e.wg.Wait()
}

func (e *Watcher) setStartUpTime(time time.Time) {
	startUpTime = time
}
