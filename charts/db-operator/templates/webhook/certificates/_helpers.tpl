{{/*
  A helper for managing certificates
*/}}

{{- define "db-operator.webhook.certificateName" -}}
{{ printf "%s-certificate" (include "db-operator.webhook.name" $)}}
{{- end }}

{{- define "db-operator.webhook.shortSvcUrl" -}}
{{- include "db-operator.webhook.name" $ }}.{{ $.Release.Namespace }}.svc
{{- end }}

{{- define "db-operator.webhook.fullSvcUrl" -}}
{{- include "db-operator.webhook.name" $ }}.{{ $.Release.Namespace }}.svc.{{ $.Values.clusterDomain }}
{{- end }}

{{- define "db-operator.webhook.selfSignedCert" -}}
{{- $shortSvcUrl := include "db-operator.webhook.shortSvcUrl" $ }}
{{- $fullSvcUrl := include "db-operator.webhook.fullSvcUrl " $ }}
{{- $cert := genSelfSignedCert $shortSvcUrl (list) (list $fullSvcUrl) 365 | toYaml -}}
{{- $cert }}
{{- end }}
