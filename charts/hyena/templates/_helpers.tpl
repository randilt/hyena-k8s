{{/*
Expand the name of the chart.
*/}}
{{- define "hyena.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "hyena.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "hyena.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "hyena.labels" -}}
helm.sh/chart: {{ include "hyena.chart" . }}
{{ include "hyena.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "hyena.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hyena.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Share server selector labels
*/}}
{{- define "hyena.shareServer.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hyena.name" . }}-share-server
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: share-server
{{- end }}

{{/*
Demo app selector labels
*/}}
{{- define "hyena.demoApp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hyena.name" . }}-demo-app
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: demo-app
{{- end }}

{{/*
Create the name of the service account to use for demo app
*/}}
{{- define "hyena.serviceAccountName" -}}
{{- if .Values.demoApp.serviceAccount.create }}
{{- default (printf "%s-demo-app" (include "hyena.fullname" .)) .Values.demoApp.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.demoApp.serviceAccount.name }}
{{- end }}
{{- end }}
