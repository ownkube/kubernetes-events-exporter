package routing

import (
	"github.com/ownkube/kubernetes-events-exporter/internal/dispatch"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

type RoutingRule struct {
	Drop   []Filter
	Match  []Filter
	Routes []RoutingRule
}

func (r *RoutingRule) ProcessEvent(ev *event.EnrichedEvent, registry dispatch.Dispatcher) {
	for _, v := range r.Drop {
		if v.MatchesEvent(ev) {
			return
		}
	}

	matchesAll := true
	for _, rule := range r.Match {
		if rule.MatchesEvent(ev) {
			if rule.Receiver != "" {
				registry.SendEvent(rule.Receiver, ev)
			}
		} else {
			matchesAll = false
		}
	}

	if matchesAll {
		for _, subRoute := range r.Routes {
			subRoute.ProcessEvent(ev, registry)
		}
	}
}
