{{- /*
Общие (не selector) метки.
*/ -}}
{{- define "ecommerce-shop.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: ecommerce-shop
{{- with .Values.global.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{- /*
Метки сервиса.
*/ -}}
{{- define "ecommerce-shop.componentLabels" -}}
{{ include "ecommerce-shop.labels" .root }}
app.kubernetes.io/component: {{ .name }}
{{- end }}

{{- /*
Selector-метки пода (иммутабельны).
*/ -}}
{{- define "ecommerce-shop.selectorLabels" -}}
app.kubernetes.io/name: {{ .root.Chart.Name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .name }}
{{- end }}

{{- /*
Возвращает "true", если включён хотя бы один сервис, иначе пустую строку.
*/ -}}
{{- define "ecommerce-shop.anyServiceEnabled" -}}
{{- $any := false -}}
{{- range $name, $svc := .Values.services -}}
{{- if $svc.enabled -}}{{- $any = true -}}{{- end -}}
{{- end -}}
{{- if $any -}}true{{- end -}}
{{- end }}

{{- /*
Полное имя релиза (для не-DNS ресурсов: SA, ConfigMap, Secret, Job).
*/ -}}
{{- define "ecommerce-shop.fullname" -}}
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
{{- end }}

{{- /*
Имя ServiceAccount'а.
*/ -}}
{{- define "ecommerce-shop.serviceAccountName" -}}
{{- printf "%s-app" (include "ecommerce-shop.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{- /*
Полное имя образа с учётом global.imageRegistry; тег: image.tag -> global.imageTag -> .Chart.AppVersion.
*/ -}}
{{- define "ecommerce-shop.image" -}}
{{- $registry := .root.Values.global.imageRegistry | default "" -}}
{{- $tag := .image.tag | default "" -}}
{{- if not $tag -}}{{- $tag = .root.Values.global.imageTag | default "" -}}{{- end -}}
{{- if not $tag -}}{{- $tag = .root.Chart.AppVersion -}}{{- end -}}
{{- $tag = $tag | toString -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" (trimSuffix "/" $registry) .image.repository $tag -}}
{{- else -}}
{{- printf "%s:%s" .image.repository $tag -}}
{{- end -}}
{{- end }}

{{- /*
imagePullPolicy с учётом global.
*/ -}}
{{- define "ecommerce-shop.imagePullPolicy" -}}
{{- .Values.global.imagePullPolicy | default "IfNotPresent" -}}
{{- end }}

{{- /*
Имя Secret'а с секретами приложения.
*/ -}}
{{- define "ecommerce-shop.secretName" -}}
{{- if .Values.secrets.existingSecret -}}
{{- .Values.secrets.existingSecret -}}
{{- else if .Values.secrets.name -}}
{{- .Values.secrets.name -}}
{{- else -}}
{{- printf "%s-secrets" (include "ecommerce-shop.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end }}

{{- /*
Имя ConfigMap'а сервиса (содержит <environment>.yaml).
*/ -}}
{{- define "ecommerce-shop.configMapName" -}}
{{- printf "%s-config" .name -}}
{{- end }}

{{- /*
init-контейнер ожидания зависимости; TCP-проверка (nc -z) или HTTP-проверка (wget), если задан item.path.
*/ -}}
{{- define "ecommerce-shop.waitContainer" -}}
{{- $root := .root -}}
{{- $item := .item -}}
{{- /* waitImage — полная ссылка на образ, global.imageRegistry к ней не применяется. */ -}}
{{- $image := $root.Values.global.waitImage | default "busybox:1.36" -}}
{{- $timeout := $root.Values.global.waitTimeoutSeconds | default 600 -}}
{{- $attempts := div (int $timeout) 3 -}}
- name: wait-for-{{ $item.name }}
  image: {{ $image | quote }}
  imagePullPolicy: {{ include "ecommerce-shop.imagePullPolicy" $root }}
  command:
    - /bin/sh
    - -c
    - |
      set -eu
      i=0
      {{- if $item.path }}
      until wget -q -O /dev/null --timeout=3 "http://{{ $item.host }}:{{ $item.port }}{{ $item.path }}"; do
      {{- else }}
      until nc -z {{ $item.host }} {{ $item.port }}; do
      {{- end }}
        i=$((i + 1))
        if [ "$i" -ge {{ $attempts }} ]; then
          echo "ERROR: timeout waiting for {{ $item.host }}:{{ $item.port }}{{ $item.path }}" >&2
          exit 1
        fi
        echo "waiting for {{ $item.host }}:{{ $item.port }}{{ $item.path }} ($i/{{ $attempts }})..."
        sleep 3
      done
      echo "{{ $item.host }}:{{ $item.port }}{{ $item.path }} is reachable"
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

{{- /*
Блок resources.
*/ -}}
{{- define "ecommerce-shop.resources" -}}
{{- with . -}}
resources:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{- /*
Секция telemetry для configs/<env>.yaml; одинакова для всех сервисов, отличается только service_name.
*/ -}}
{{- define "ecommerce-shop.telemetryBlock" -}}
{{- $obs := .root.Values.observability -}}
telemetry:
  enabled: {{ $obs.enabled }}
  logs: {{ $obs.logs }}
  metrics: {{ $obs.metrics }}
  traces: {{ $obs.traces }}
  endpoint: {{ $obs.otel.endpoint | quote }}
  insecure: {{ $obs.otel.insecure }}
  service_name: {{ .name | quote }}
  service_version: {{ .root.Chart.AppVersion | quote }}
  environment: {{ .root.Values.global.environment | quote }}
  tracesSampleRate: {{ $obs.tracesSampleRate }}
  logToStdout: {{ $obs.logToStdout }}
  sampleLogs: {{ $obs.sampleLogs }}
{{- end }}

{{- /*
Список allowedOrigins сервиса (с фолбэком на defaults.allowedOrigins).
*/ -}}
{{- define "ecommerce-shop.allowedOrigins" -}}
{{- $origins := .root.Values.defaults.allowedOrigins -}}
{{- if hasKey .svc "allowedOrigins" -}}
{{- $origins = .svc.allowedOrigins -}}
{{- end -}}
{{- range $origins }}
  - {{ . | quote }}
{{- end }}
{{- end }}

{{- /*
Секция outbox для configs/<env>.yaml.
*/ -}}
{{- define "ecommerce-shop.outboxBlock" -}}
{{- $o := .root.Values.outbox -}}
{{ .key }}:
  batchSize: {{ $o.batchSize }}
  pollInterval: {{ $o.pollInterval }}
  maxAttempts: {{ $o.maxAttempts }}
  errorBackoff: {{ $o.errorBackoff }}
  dbTimeout: {{ $o.dbTimeout }}
  sendTimeout: {{ $o.sendTimeout }}
  lease: {{ $o.lease }}
{{- end }}

{{- /*
Секция kafka consumer для configs/<env>.yaml.
*/ -}}
{{- define "ecommerce-shop.kafkaConsumerBlock" -}}
{{ .key }}:
  brokers:
    {{- /* brokers — строка через запятую, поэтому splitList. */}}
    {{- range (splitList "," .root.Values.dependencies.kafka.brokers) }}
    - {{ trim . | quote }}
    {{- end }}
  groupID: {{ .groupID | quote }}
  topic: {{ .topic | quote }}
  dlqTopic: {{ .dlqTopic | quote }}
  sessionTimeout: {{ .root.Values.kafkaConsumer.sessionTimeout }}
  heartbeatInterval: {{ .root.Values.kafkaConsumer.heartbeatInterval }}
  maxPollInterval: {{ .root.Values.kafkaConsumer.maxPollInterval }}
  StartOffsetEarliest: {{ .root.Values.kafkaConsumer.startOffsetEarliest }}
{{- end }}

{{- /*
Секция kafka producer для configs/<env>.yaml.
*/ -}}
{{- define "ecommerce-shop.kafkaProducerBlock" -}}
kafka_producer:
  brokers:
    {{- range (splitList "," .root.Values.dependencies.kafka.brokers) }}
    - {{ trim . | quote }}
    {{- end }}
  clientID: {{ .name | quote }}
  dialTimeout: {{ .root.Values.kafkaProducer.dialTimeout }}
  writeTimeout: {{ .root.Values.kafkaProducer.writeTimeout }}
  maxRetries: {{ .root.Values.kafkaProducer.maxRetries }}
  retryBackoff: {{ .root.Values.kafkaProducer.retryBackoff }}
{{- end }}

{{- /*
Секция postgres (без пароля — он приходит из Secret).
*/ -}}
{{- define "ecommerce-shop.postgresBlock" -}}
{{- $pg := index .root.Values.dependencies.postgres .inst -}}
postgres:
  host: {{ $pg.host | quote }}
  port: {{ $pg.port | quote }}
  user: {{ $pg.username | quote }}
  dbName: {{ $pg.database | quote }}
  sslMode: {{ $pg.sslMode | quote }}
  maxConns: 20
  minConns: 5
  maxConnLifetime: 1h
  maxConnIdleTime: 30m
  healthCheckPeriod: 1m
  connectTimeout: 10s
{{- end }}
