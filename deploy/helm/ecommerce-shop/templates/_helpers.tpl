{{- /*
Общие (не selector!) метки.
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
Использование: include "ecommerce-shop.componentLabels" (dict "root" . "name" "auth-service")
*/ -}}
{{- define "ecommerce-shop.componentLabels" -}}
{{ include "ecommerce-shop.labels" .root }}
app.kubernetes.io/component: {{ .name }}
{{- end }}

{{- /*
Selector-метки пода. Иммутабельны — менять нельзя.
*/ -}}
{{- define "ecommerce-shop.selectorLabels" -}}
app.kubernetes.io/name: {{ .root.Chart.Name }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .name }}
{{- end }}

{{- /*
Возвращает "true", если включён хотя бы один сервис. Иначе пустую строку.
Использование: {{- if include "ecommerce-shop.anyServiceEnabled" . }}
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
Полное имя образа с учётом global.imageRegistry.
Использование: include "ecommerce-shop.image" (dict "root" . "image" $svc.image)

Тег выбирается по цепочке: image.tag -> global.imageTag -> .Chart.AppVersion.
global.imageTag позволяет переопределить тег СРАЗУ для всех сервисов и Job'ов
миграций:
  --set global.imageTag=1.2.3
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
Имя ConfigMap'а сервиса -> содержит <environment>.yaml.
Использование: include "ecommerce-shop.configMapName" (dict "root" . "name" "auth-service")
*/ -}}
{{- define "ecommerce-shop.configMapName" -}}
{{- printf "%s-config" .name -}}
{{- end }}

{{- /*
init-контейнер ожидания зависимости.
Использование: include "ecommerce-shop.waitContainer" (dict "root" . "item" $item)
Поддерживает TCP-проверку (nc -z) и HTTP-проверку (wget), если задан item.path.
*/ -}}
{{- define "ecommerce-shop.waitContainer" -}}
{{- $root := .root -}}
{{- $item := .item -}}
{{- /* waitImage — ПОЛНАЯ ссылка на образ, global.imageRegistry к ней НЕ применяется:
       в приватных реестрах busybox обычно лежит по другому пути (library/busybox),
       чем образы приложения, и авто-префикс давал бы ImagePullBackOff. */ -}}
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
Секция telemetry для configs/<env>.yaml.
Одинакова для всех сервисов, отличается только service_name.
Использование: include "ecommerce-shop.telemetryBlock" (dict "root" . "name" "auth-service")
ВАЖНО: имена ключей должны совпадать с yaml-тегами infracfg.TelemetryConfig.
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
Использование: include "ecommerce-shop.allowedOrigins" (dict "root" . "svc" $svc)
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
Использование: include "ecommerce-shop.outboxBlock" (dict "root" . "key" "user_created_outbox")
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
Использование: include "ecommerce-shop.kafkaConsumerBlock" (dict "root" . "key" "orderCreatedConsumer" "groupID" "inventory-service" "topic" "order.created" "dlqTopic" "order.created.dlq")
*/ -}}
{{- define "ecommerce-shop.kafkaConsumerBlock" -}}
{{ .key }}:
  brokers:
    {{- /* brokers — строка через запятую, поэтому splitList, а не одна строка:
           иначе несколько брокеров превращаются в один невалидный адрес. */}}
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
Использование: include "ecommerce-shop.postgresBlock" (dict "root" . "inst" "auth")
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
