{{- /*
================================================================================
Helpers for the "observability" chart.
==============================================================================

ВАЖНО (IMPORTANT): этот чарт НЕ добавляет имя релиза к именам сервисов.

Конфиги, перенесённые из docker-compose (infra/observability/*.yaml), ссылаются
на DNS-имена сервисов жёстко: `tempo:4317`, `loki:3100`, `prometheus:9090`.
Поэтому имена Service'ов обязаны оставаться короткими и стабильными:
`otel-collector`, `tempo`, `loki`, `prometheus`, `grafana`.

Каждый компонент имеет свой `fullnameOverride` (по умолчанию равен имени
компонента). Меняйте его только если понимаете, что делаете: при этом нужно
поправить и конфиги (endpoint'ы) в values.yaml.

Следствие: в одном namespace нельзя ставить два релиза этого чарта без
переопределения `fullnameOverride` у всех компонентов.
*/ -}}

{{- /*
Chart name and version, used for the helm.sh/chart label.
*/ -}}
{{- define "observability.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- /*
Common labels for every resource.
Usage: {{- include "observability.labels" . | nindent 4 }}
*/ -}}
{{- define "observability.labels" -}}
helm.sh/chart: {{ include "observability.chart" . }}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- /*
Labels for a single component.
Usage: {{- include "observability.componentLabels" (dict "root" . "component" "tempo") | nindent 4 }}
*/ -}}
{{- define "observability.componentLabels" -}}
{{ include "observability.labels" .root }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- /*
Selector labels for a single component. Must stay immutable.
Usage: {{- include "observability.selectorLabels" (dict "root" . "component" "tempo") | nindent 6 }}
*/ -}}
{{- define "observability.selectorLabels" -}}
app.kubernetes.io/name: {{ .root.Chart.Name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- /*
Short, stable, release-independent resource name for a component.
Resolution order: .Values.<component>.fullnameOverride -> <component>.
Usage: {{- include "observability.componentName" (dict "root" . "component" "tempo") }}
*/ -}}
{{- define "observability.componentName" -}}
{{- $name := .component -}}
{{- $cfg := index .root.Values .component -}}
{{- if and $cfg $cfg.fullnameOverride -}}
{{- $name = $cfg.fullnameOverride -}}
{{- end -}}
{{- $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- /*
ServiceAccount name for a component.
Usage: {{- include "observability.serviceAccountName" (dict "root" . "component" "tempo") }}
*/ -}}
{{- define "observability.serviceAccountName" -}}
{{- $cfg := index .root.Values .component -}}
{{- if and $cfg $cfg.serviceAccount $cfg.serviceAccount.name -}}
{{- $cfg.serviceAccount.name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "observability.componentName" . -}}
{{- end -}}
{{- end -}}

{{- /*
Fully qualified image reference: repository:tag.
Usage: {{- include "observability.image" (dict "root" . "component" "tempo") }}
*/ -}}
{{- define "observability.image" -}}
{{- $cfg := index .root.Values .component -}}
{{- $registry := (index .root.Values "global").imageRegistry | default "" -}}
{{- $tag := $cfg.image.tag | toString -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" (trimSuffix "/" $registry) $cfg.image.repository $tag -}}
{{- else -}}
{{- printf "%s:%s" $cfg.image.repository $tag -}}
{{- end -}}
{{- end -}}

{{- /*
Checksum of a component ConfigMap payload, used to force a rollout on config change.
Usage: {{- include "observability.configChecksum" (dict "root" . "component" "tempo" "config" $c.config) }}
*/ -}}
{{- define "observability.configChecksum" -}}
{{- printf "%s" .config | sha256sum -}}
{{- end -}}

{{- /*
Image pull secrets block, rendered only when configured.
Usage: {{- include "observability.imagePullSecrets" (dict "root" . "component" "tempo") | nindent 6 }}
*/ -}}
{{- define "observability.imagePullSecrets" -}}
{{- $cfg := index .root.Values .component -}}
{{- /* Приоритет: imagePullSecrets компонента, иначе global.imagePullSecrets. */ -}}
{{- $secrets := $cfg.imagePullSecrets | default (index .root.Values "global").imagePullSecrets -}}
{{- with $secrets -}}
imagePullSecrets:
{{- toYaml . | nindent 2 }}
{{- end -}}
{{- end -}}

{{- /*
Version of the chart, exposed for NOTES.txt.
*/ -}}
{{- define "observability.notesEnabledList" -}}
{{- $components := list "otel-collector" "tempo" "loki" "prometheus" "grafana" -}}
{{- $enabled := list -}}
{{- range $components -}}
{{- if (index $.Values .).enabled -}}
{{- $enabled = append $enabled . -}}
{{- end -}}
{{- end -}}
{{- join ", " $enabled -}}
{{- end -}}
