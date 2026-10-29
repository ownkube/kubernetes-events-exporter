{{/*
Expand the name of the chart.
*/}}
{{- define "kubernetes-events-exporter.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "kubernetes-events-exporter.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kubernetes-events-exporter.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "kubernetes-events-exporter.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kubernetes-events-exporter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubernetes-events-exporter.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service account name
*/}}
{{- define "kubernetes-events-exporter.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kubernetes-events-exporter.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Leader election is on when either the top-level leaderElection.enabled or
config.leaderElection.enabled is true. Renders "true" or nothing.
*/}}
{{- define "kubernetes-events-exporter.leaderElection" -}}
{{- $cfg := .Values.config.leaderElection | default dict }}
{{- if or .Values.leaderElection.enabled $cfg.enabled }}true{{ end }}
{{- end }}

{{/*
Exporter config as written to the ConfigMap, with the top-level
leaderElection.enabled folded into config.leaderElection.
*/}}
{{- define "kubernetes-events-exporter.config" -}}
{{- $cfg := deepCopy .Values.config }}
{{- if include "kubernetes-events-exporter.leaderElection" . }}
{{- $le := $cfg.leaderElection | default dict }}
{{- $_ := set $le "enabled" true }}
{{- $_ := set $cfg "leaderElection" $le }}
{{- end }}
{{- toYaml $cfg }}
{{- end }}
