{{/*
Webhook templates
*/}}

{{/*
Webhook specific labels
*/}}
{{- define "db-operator.webhook.specificLabels" -}}
app.kubernetes.io/component: "webhook"
{{- end -}}

{{/*
Webhook selector labels
*/}}
{{- define "db-operator.webhook.selectorLabels" -}}
{{ include "db-operator.selectorLabels" . }}
{{ include "db-operator.webhook.specificLabels" . }}
{{- end -}}

{{/*
Webhook labels
*/}}
{{- define "db-operator.webhook.labels" -}}
{{ include "db-operator.labels" . }}
{{ include "db-operator.webhook.specificLabels" . }}
{{- end -}}

{{/*
Expand the name of the chart.
*/}}
{{- define "db-operator.webhook.name" -}}
{{- printf "db-operator-webhook" -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "db-operator.webhook.serviceAccountName" -}}
{{- if .Values.webhook.serviceAccount.create -}}
    {{ default (include "db-operator.webhook.name" .) .Values.webhook.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.webhook.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Webhook extra args
*/}}
{{- define "db-operator.webhook.args" -}}
{{- $args := list -}}
{{- $args = append $args (printf "--zap-log-level=%s" .Values.webhook.logLevel) -}}
{{- $args = append $args "--webhook" -}}
{{- with .Values.webhook }}
{{- range .extraArgs -}}
{{- $args = append $args . -}}
{{- end -}}
{{- end -}}
{{ join "," $args }}
{{- end -}}
