{{- /*
Общие (не selector) метки для всех ресурсов чарта.
*/ -}}
{{- define "infra.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: ecommerce-infra
{{- end }}

{{- /*
Метки конкретного компонента.
*/ -}}
{{- define "infra.componentLabels" -}}
{{ include "infra.labels" .root }}
app.kubernetes.io/component: {{ .component }}
{{- with .role }}
ecommerce.io/role: {{ . }}
{{- end }}
{{- end }}

{{- /*
Selector-метки пода; менять нельзя — selector иммутабелен.
*/ -}}
{{- define "infra.selectorLabels" -}}
app.kubernetes.io/name: {{ .root.Chart.Name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{- /*
Полное имя образа с учётом global.imageRegistry.
*/ -}}
{{- define "infra.image" -}}
{{- $registry := .root.Values.global.imageRegistry | default "" -}}
{{- $tag := .image.tag | default .root.Chart.AppVersion | toString -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" (trimSuffix "/" $registry) .image.repository $tag -}}
{{- else -}}
{{- printf "%s:%s" .image.repository $tag -}}
{{- end -}}
{{- end }}

{{- /*
imagePullPolicy с учётом global.
*/ -}}
{{- define "infra.imagePullPolicy" -}}
{{- .Values.global.imagePullPolicy | default "IfNotPresent" -}}
{{- end }}

{{- /*
Блок imagePullSecrets.
*/ -}}
{{- define "infra.imagePullSecrets" -}}
{{- with .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{- /*
Блок resources из values (пустой объект -> ничего не рендерим).
*/ -}}
{{- define "infra.resources" -}}
{{- with . -}}
resources:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{- /*
Имя Secret'а с паролями PostgreSQL.
*/ -}}
{{- define "infra.postgresql.secretName" -}}
{{- if .Values.postgresql.auth.existingSecret -}}
{{- .Values.postgresql.auth.existingSecret -}}
{{- else -}}
{{- .Values.postgresql.auth.secretName -}}
{{- end -}}
{{- end }}

{{- /*
init-контейнер, который ждёт доступности TCP-порта зависимости.
*/ -}}
{{- define "infra.waitFor" -}}
{{- /* waitImage — полная ссылка на образ, global.imageRegistry не применяется. */ -}}
{{- $image := .root.Values.global.waitImage | default "busybox:1.36" -}}
{{- $attempts := div (int (.timeoutSeconds | default 300)) 3 -}}
- name: wait-for-{{ .name }}
  image: {{ $image | quote }}
  imagePullPolicy: {{ include "infra.imagePullPolicy" .root }}
  command:
    - /bin/sh
    - -c
    - |
      set -eu
      i=0
      until nc -z {{ .host }} {{ .port }}; do
        i=$((i + 1))
        if [ "$i" -ge {{ $attempts }} ]; then
          echo "ERROR: timeout waiting for {{ .host }}:{{ .port }}" >&2
          exit 1
        fi
        echo "waiting for {{ .host }}:{{ .port }} ($i/{{ $attempts }})..."
        sleep 3
      done
      echo "{{ .host }}:{{ .port }} is reachable"
  resources:
    requests:
      cpu: 10m
      memory: 16Mi
    limits:
      cpu: 100m
      memory: 64Mi
  securityContext:
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: true
    capabilities:
      drop:
        - ALL
{{- end }}
