{{- /*
Helpers for the "observability" chart.

Имена Service'ов не префиксуются именем релиза: конфиги ссылаются на короткие
DNS-имена `tempo:4317`, `loki:3100`, `prometheus:9090`; каждый компонент имеет
fullnameOverride, равный имени компонента. В одном namespace допустим только
один релиз этого чарта.
*/ -}}

{{- /*
Chart name and version, used for the helm.sh/chart label.
*/ -}}
{{- define "observability.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- /*
Common labels for every resource.
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
*/ -}}
{{- define "observability.componentLabels" -}}
{{ include "observability.labels" .root }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- /*
Selector labels for a single component; immutable.
*/ -}}
{{- define "observability.selectorLabels" -}}
app.kubernetes.io/name: {{ .root.Chart.Name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- /*
Short, stable, release-independent resource name for a component.
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
Checksum of a component ConfigMap payload; forces a rollout on config change.
*/ -}}
{{- define "observability.configChecksum" -}}
{{- printf "%s" .config | sha256sum -}}
{{- end -}}

{{- /*
Image pull secrets block, rendered only when configured.
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
Enabled components list for NOTES.txt.
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
