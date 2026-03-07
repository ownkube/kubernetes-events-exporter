package dispatch

import (
	"context"
	"log/slog"
	"sync"

	"github.com/ownkube/kubernetes-events-exporter/internal/event"
	"github.com/ownkube/kubernetes-events-exporter/internal/observability"
	"github.com/ownkube/kubernetes-events-exporter/internal/output"
)

type AsyncDispatcher struct {
	ch           map[string]chan event.EnrichedEvent
	exitCh       map[string]chan interface{}
	wg           *sync.WaitGroup
	Counters     *observability.Counters
}

func (r *AsyncDispatcher) SendEvent(name string, ev *event.EnrichedEvent) {
	ch := r.ch[name]
	if ch == nil {
		slog.Error("There is no channel", "name", name)
	}

	go func() {
		ch <- *ev
	}()
}

func (r *AsyncDispatcher) Register(name string, receiver output.Handler) {
	if r.ch == nil {
		r.ch = make(map[string]chan event.EnrichedEvent)
		r.exitCh = make(map[string]chan interface{})
	}

	ch := make(chan event.EnrichedEvent)
	exitCh := make(chan interface{})

	r.ch[name] = ch
	r.exitCh[name] = exitCh

	if r.wg == nil {
		r.wg = &sync.WaitGroup{}
	}
	r.wg.Add(1)

	go func() {
	Loop:
		for {
			select {
			case ev := <-ch:
				slog.Debug("sending event to sink", "sink", name, "event", ev.Message)
				err := receiver.Send(context.Background(), &ev)
				if err != nil {
					r.Counters.SendErrors.Inc()
					slog.Debug("Cannot send event", "error", err, "sink", name, "event", ev.Message)
				}
			case <-exitCh:
				slog.Info("Closing the sink", "sink", name)
				break Loop
			}
		}
		receiver.Close()
		slog.Info("Closed", "sink", name)
		r.wg.Done()
	}()
}

func (r *AsyncDispatcher) Close() {
	for _, ec := range r.exitCh {
		ec <- 1
	}
	r.wg.Wait()
}
