{{- define "githook.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "githook.fullname" -}}
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

{{- define "githook.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "githook.labels" -}}
helm.sh/chart: {{ include "githook.chart" . }}
{{ include "githook.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "githook.selectorLabels" -}}
app.kubernetes.io/name: {{ include "githook.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "githook.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "githook.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "githook.configVolumeName" -}}
{{- printf "%s-config" (include "githook.fullname" .) -}}
{{- end -}}

{{- define "githook.configResourceName" -}}
{{- if .Values.config.existingSecret -}}
  {{- .Values.config.existingSecret -}}
{{- else -}}
  {{- if .Values.config.createSecret -}}
    {{- printf "%s-config" (include "githook.fullname" .) -}}
  {{- else -}}
    {{- required "config.existingSecret is required when config.createSecret=false" .Values.config.existingSecret -}}
  {{- end -}}
{{- end -}}
{{- end -}}
