package event

import (
	"encoding/json"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EnrichedEvent struct {
	corev1.Event       `json:",inline"`
	ClusterName        string            `json:"clusterName,omitempty"`
	ClusterEnvironment string            `json:"clusterEnvironment,omitempty"`
	InvolvedObject     EnrichedObjectRef `json:"involvedObject"`
}

func (e EnrichedEvent) DeDot() EnrichedEvent {
	c := e
	c.Labels = dedotMap(e.Labels)
	c.Annotations = dedotMap(e.Annotations)
	c.InvolvedObject.Labels = dedotMap(e.InvolvedObject.Labels)
	c.InvolvedObject.Annotations = dedotMap(e.InvolvedObject.Annotations)
	return c
}

func dedotMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return in
	}
	ret := make(map[string]string, len(in))
	for key, value := range in {
		nKey := strings.ReplaceAll(key, ".", "_")
		ret[nKey] = value
	}
	return ret
}

type EnrichedObjectRef struct {
	corev1.ObjectReference `json:",inline"`
	Labels                 map[string]string       `json:"labels,omitempty"`
	Annotations            map[string]string       `json:"annotations,omitempty"`
	OwnerReferences        []metav1.OwnerReference `json:"ownerReferences,omitempty"`
	Deleted                bool                    `json:"deleted"`
}

func (e *EnrichedEvent) ToJSON() []byte {
	b, _ := json.Marshal(e)
	return b
}

func (e *EnrichedEvent) GetTimestampMs() int64 {
	timestamp := e.FirstTimestamp.Time
	if timestamp.IsZero() {
		timestamp = e.EventTime.Time
	}

	return timestamp.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

func (e *EnrichedEvent) GetTimestampISO8601() string {
	timestamp := e.FirstTimestamp.Time
	if timestamp.IsZero() {
		timestamp = e.EventTime.Time
	}

	layout := "2006-01-02T15:04:05.000Z"
	return timestamp.Format(layout)
}
