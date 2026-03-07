package output

import (
	"bytes"
	"encoding/json"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/ownkube/kubernetes-events-exporter/internal/event"
)

func RenderTemplate(ev *event.EnrichedEvent, text string) (string, error) {
	tmpl, err := template.New("template").Funcs(sprig.TxtFuncMap()).Parse(text)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, ev)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func ConvertLayoutTemplate(layout map[string]interface{}, ev *event.EnrichedEvent) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range layout {
		m, err := convertTemplate(value, ev)
		if err != nil {
			return nil, err
		}
		result[key] = m
	}
	return result, nil
}

func convertTemplate(value interface{}, ev *event.EnrichedEvent) (interface{}, error) {
	switch v := value.(type) {
	case string:
		rendered, err := RenderTemplate(ev, v)
		if err != nil {
			return nil, err
		}

		return rendered, nil
	case map[interface{}]interface{}:
		strKeysMap := make(map[string]interface{})
		for k, v := range v {
			res, err := convertTemplate(v, ev)
			if err != nil {
				return nil, err
			}
			strKeysMap[k.(string)] = res
		}
		return strKeysMap, nil
	case map[string]interface{}:
		strKeysMap := make(map[string]interface{})
		for k, v := range v {
			res, err := convertTemplate(v, ev)
			if err != nil {
				return nil, err
			}
			strKeysMap[k] = res
		}
		return strKeysMap, nil
	case []interface{}:
		listConf := make([]interface{}, len(v))
		for i := range v {
			t, err := convertTemplate(v[i], ev)
			if err != nil {
				return nil, err
			}
			listConf[i] = t
		}
		return listConf, nil
	}
	return nil, nil
}

func SerializeEventWithLayout(layout map[string]interface{}, ev *event.EnrichedEvent) ([]byte, error) {
	var toSend []byte
	if layout != nil {
		res, err := ConvertLayoutTemplate(layout, ev)
		if err != nil {
			return nil, err
		}

		toSend, err = json.Marshal(res)
		if err != nil {
			return nil, err
		}
	} else {
		toSend = ev.ToJSON()
	}
	return toSend, nil
}
