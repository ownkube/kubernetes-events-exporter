package routing

import (
	"github.com/ownkube/kubernetes-events-exporter/internal/dispatch"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type Router struct {
	Route RoutingRule
	Rcvr  dispatch.Dispatcher
}

func (r *Router) ProcessEvent(ev *event.EnrichedEvent) {
	r.Route.ProcessEvent(ev, r.Rcvr)
}
