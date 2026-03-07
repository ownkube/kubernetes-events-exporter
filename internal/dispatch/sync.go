package dispatch

import (
	"context"
	"log/slog"

	"github.com/ownkube/kubernetes-events-exporter/internal/event"
	"github.com/ownkube/kubernetes-events-exporter/internal/output"
)

type SyncDispatcher struct {
	reg map[string]output.Handler
}

func (s *SyncDispatcher) SendEvent(name string, ev *event.EnrichedEvent) {
	err := s.reg[name].Send(context.Background(), ev)
	if err != nil {
		slog.Debug("Cannot send event", "error", err, "sink", name, "event", string(ev.UID))
	}
}

func (s *SyncDispatcher) Register(name string, handler output.Handler) {
	if s.reg == nil {
		s.reg = make(map[string]output.Handler)
	}

	s.reg[name] = handler
}

func (s *SyncDispatcher) Close() {
	for name, handler := range s.reg {
		slog.Info("Closing sink", "sink", name)
		handler.Close()
	}
}
