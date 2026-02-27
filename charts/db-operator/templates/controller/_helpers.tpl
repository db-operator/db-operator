{{/*
Controller templates
*/}}

{{/*
Controller specific labels
*/}}
{{- define "db-operator.controller.specificLabels" -}}
app.kubernetes.io/component: "controller"
{{- end -}}

{{/*
Controller selector labels
*/}}
{{- define "db-operator.controller.selectorLabels" -}}
{{ include "db-operator.selectorLabels" . }}
{{ include "db-operator.controller.specificLabels" . }}
{{- end -}}

{{/*
Controller labels
*/}}
{{- define "db-operator.controller.labels" -}}
{{ include "db-operator.labels" . }}
{{ include "db-operator.controller.specificLabels" . }}
{{- end -}}

{{/*
Expand the name of the chart.
*/}}
{{- define "db-operator.controller.name" -}}
{{- printf "db-operator-controller" -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "db-operator.controller.serviceAccountName" -}}
{{- if .Values.controller.serviceAccount.create -}}
    {{ default (include "db-operator.controller.name" .) .Values.controller.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.controller.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Controller extra args
*/}}
{{- define "db-operator.controller.args" -}}
{{- $args := list -}}
{{- $args = append $args (printf "--zap-log-level=%s" .Values.controller.logLevel) -}}
{{- with .Values.Controller }}
{{- range .extraArgs -}}
{{- $args = append $args . -}}
{{- end -}}
{{- end -}}
{{ join "," $args }}
{{- end -}}
