package output

import (
	"context"

	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type InMemoryConfig struct {
	Ref *InMemory
}

type InMemory struct {
	Events []*event.EnrichedEvent
	Config *InMemoryConfig
}

func (i *InMemory) Send(ctx context.Context, ev *event.EnrichedEvent) error {
	i.Events = append(i.Events, ev)
	return nil
}

func (i *InMemory) Close() {
	// No-op
}
