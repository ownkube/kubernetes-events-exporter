package routing

import (
	"regexp"

	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

func matchString(pattern, s string) bool {
	matched, _ := regexp.MatchString(pattern, s)
	return matched
}

type Filter struct {
	Labels      map[string]string
	Annotations map[string]string
	Message     string
	APIVersion  string `yaml:"apiVersion"`
	Kind        string
	Namespace   string
	Reason      string
	Type        string
	MinCount    int32 `yaml:"minCount"`
	Component   string
	Host        string
	Receiver    string
}

func (r *Filter) MatchesEvent(ev *event.EnrichedEvent) bool {
	rules := [][2]string{
		{r.Message, ev.Message},
		{r.APIVersion, ev.InvolvedObject.APIVersion},
		{r.Kind, ev.InvolvedObject.Kind},
		{r.Namespace, ev.Namespace},
		{r.Reason, ev.Reason},
		{r.Type, ev.Type},
		{r.Component, ev.Source.Component},
		{r.Host, ev.Source.Host},
	}

	for _, v := range rules {
		rule := v[0]
		value := v[1]
		if rule != "" {
			matches := matchString(rule, value)
			if !matches {
				return false
			}
		}
	}

	if r.Labels != nil && len(r.Labels) > 0 {
		for k, v := range r.Labels {
			if val, ok := ev.InvolvedObject.Labels[k]; !ok {
				return false
			} else {
				matches := matchString(v, val)
				if !matches {
					return false
				}
			}
		}
	}

	if r.Annotations != nil && len(r.Annotations) > 0 {
		for k, v := range r.Annotations {
			if val, ok := ev.InvolvedObject.Annotations[k]; !ok {
				return false
			} else {
				matches := matchString(v, val)
				if !matches {
					return false
				}
			}
		}
	}

	if ev.Count < r.MinCount {
		return false
	}

	return true
}
