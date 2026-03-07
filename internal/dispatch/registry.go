package dispatch

import (
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
	"github.com/ownkube/kubernetes-events-exporter/internal/output"
)

type Dispatcher interface {
	SendEvent(string, *event.EnrichedEvent)
	Register(string, output.Handler)
	Close()
}
