{{/*
Expand the name of the chart.
*/}}
{{- define "db-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "db-operator.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "db-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "db-operator.name" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Arguments builder
*/}}
{{- define "db-operator.args" -}}
{{- $args := list -}}
{{- $args = append $args (printf "--zap-log-level=%s" .Values.controller.logLevel) -}}
{{- if .Values.checkForChanges -}}
{{- $args = append $args "--check-for-changes" -}}
{{- end -}}
{{- /* controller-specific extraArgs */ -}}
{{- range .Values.controller.extraArgs -}}
{{- $args = append $args . -}}
{{- end -}}
{{ join "," $args }}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "db-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
DB Operator labels
*/}}
{{- define "db-operator.labels" -}}
helm.sh/chart: {{ include "db-operator.chart" . }}
{{ include "db-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "db-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "db-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Build an image out of object like that:
image:
  registry: ghcr.io
  repositoru: db-operator/db-operator
  tag: latest
Might be used to make it easier to configure mirroring
*/}}
{{- define "db-operator.imageBootsrap" -}}
{{- $image := "" }}
{{- if .image.registry }}
{{- $image = printf "%s/" .image.registry }}
{{- end }}
{{- $tag := printf "%s" .chart.AppVersion }}
{{- if .image.tag }}
{{- $tag = .image.tag }}
{{- end }}
{{- printf "%s%s:%s" $image .image.repository $tag }}
{{- end }}
