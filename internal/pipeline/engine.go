package pipeline

import (
	"log/slog"
	"reflect"

	"github.com/ownkube/kubernetes-events-exporter/internal/config"
	"github.com/ownkube/kubernetes-events-exporter/internal/dispatch"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
	"github.com/ownkube/kubernetes-events-exporter/internal/routing"
)

type Pipeline struct {
	Route    routing.RoutingRule
	Registry dispatch.Dispatcher
}

func NewPipeline(cfg *config.AppConfig, registry dispatch.Dispatcher) *Pipeline {
	for _, v := range cfg.Receivers {
		handler, err := v.GetHandler()
		if err != nil {
			slog.Error("Cannot initialize sink", "error", err, "name", v.Name)
			panic("cannot initialize sink: " + v.Name + ": " + err.Error())
		}

		slog.Info("Registering sink",
			"name", v.Name,
			"type", reflect.TypeOf(handler).String())

		registry.Register(v.Name, handler)
	}

	return &Pipeline{
		Route:    cfg.Route,
		Registry: registry,
	}
}

func (e *Pipeline) OnEvent(ev *event.EnrichedEvent) {
	e.Route.ProcessEvent(ev, e.Registry)
}

func (e *Pipeline) Stop() {
	slog.Info("Closing sinks")
	e.Registry.Close()
	slog.Info("All sinks closed")
}
