# observability

Standalone Helm-чарт стека наблюдаемости для E-Commerce Shop:

```
Go-сервисы ──OTLP(4317/4318)──▶ otel-collector ──▶ tempo   (трейсы)
                                      │
                                      ├──────────▶ loki    (логи)
                                      │
                                      └─ prometheus exporter :8889 ──▶ prometheus ──▶ grafana
                                         (метрики сервисов)              (скрейп)
                                         внутренние метрики :8888 ───────┘
```

| Компонент        | Образ                                            | Назначение                          |
| ---------------- | ------------------------------------------------ | ----------------------------------- |
| `otel-collector` | `otel/opentelemetry-collector-contrib:0.149.0`   | приём OTLP, конвертация метрик      |
| `tempo`          | `grafana/tempo:2.7.2`                            | хранилище трейсов                   |
| `loki`           | `grafana/loki:3.5.0`                             | хранилище логов                     |
| `prometheus`     | `prom/prometheus:v3.1.0`                         | сбор и хранение метрик              |
| `grafana`        | `grafana/grafana:11.6.0`                         | UI + provisioning datasource'ов     |

Версии зафиксированы (pinned) в `values.yaml` и совпадают с `docker-compose.yaml`
для otel-collector / tempo / loki / grafana.

---

## 1. Установка

```bash
# dev (kind/minikube, без PVC, анонимный Grafana)
/snap/helm/545/helm upgrade --install obs deploy/helm/observability \
  -n observability --create-namespace \
  -f deploy/helm/observability/values-dev.yaml

# prod
kubectl -n observability create secret generic grafana-admin \
  --from-literal=admin-password="$(openssl rand -base64 24)"
/snap/helm/545/helm upgrade --install obs deploy/helm/observability \
  -n observability --create-namespace \
  -f deploy/helm/observability/values-prod.yaml
```

Namespace **нигде не хардкодится** и `metadata.namespace` не выставляется —
используйте `-n <namespace>`, Helm проставит namespace сам.

---

## 2. ⚠️ Стабильные (не префиксованные) имена Service'ов

Конфиги перенесены из `infra/observability/*.yaml` **без изменения
endpoint'ов**, а они ссылаются на сервисы по коротким DNS-именам:
`tempo:4317`, `http://loki:3100/otlp`, `http://tempo:3200`, `http://loki:3100`,
`http://prometheus:9090`.

Поэтому имена Service'ов обязаны оставаться короткими и одинаковыми в любом
релизе/namespace:

| Ресурс              | Имя (по умолчанию)   |
| ------------------- | -------------------- |
| Service/Deployment  | `otel-collector`     |
| Service/StatefulSet | `tempo`, `loki`, `prometheus`, `grafana` |
| ConfigMap           | `<component>-config`, `grafana-datasources` |
| ServiceAccount      | `<component>`        |

Реализовано через собственный `fullnameOverride` **у каждого компонента**:

```yaml
tempo:
  fullnameOverride: tempo   # по умолчанию = имя компонента
```

Имя релиза в имена ресурсов НЕ добавляется (`observability.componentName` не
использует `.Release.Name`). Последствия, которые нужно понимать:

* в одном namespace может жить только один релиз чарта. Для второго релиза
  переопределите `fullnameOverride` у **всех** компонентов **и** endpoint'ы в
  `*.config` / `grafana.datasources` (иначе они будут указывать на первый релиз);
* `helm uninstall` удалит ресурсы по этим коротким именам — не ставьте чарт в
  namespace, где уже есть объекты с такими именами;
* PVC, созданные через `volumeClaimTemplates`, при `helm uninstall` **не
  удаляются** (стандартное поведение StatefulSet) — чистите вручную.

---

## 3. Значения (values)

### Общие для каждого компонента

| Ключ                                   | Описание                                                        |
| -------------------------------------- | --------------------------------------------------------------- |
| `<c>.enabled`                          | вкл/выкл компонент (по умолчанию `true` у всех)                  |
| `<c>.fullnameOverride`                 | короткое стабильное имя ресурсов (см. §2)                        |
| `<c>.image.{repository,tag,pullPolicy}`| образ                                                            |
| `<c>.replicaCount`                     | реплики (для tempo/loki/prometheus/grafana — только `1`, см. §5) |
| `<c>.serviceAccount.{create,name,annotations}` | ServiceAccount, `automountServiceAccountToken: false`   |
| `<c>.resources.{requests,limits}`      | requests/limits                                                  |
| `<c>.podSecurityContext`, `<c>.securityContext` | security-контексты                                      |
| `<c>.podAnnotations`, `<c>.imagePullSecrets`, `<c>.nodeSelector`, `<c>.tolerations`, `<c>.affinity`, `<c>.extraEnv` | стандартные настройки пода |
| `<c>.persistence.{enabled,storageClass,accessMode,size,annotations}` | PVC (кроме otel-collector)          |
| `<c>.config`                           | содержимое конфига (inline, попадает в ConfigMap)                |

### Специфичные

| Ключ                                        | Описание |
| ------------------------------------------- | -------- |
| `otel-collector.{otlpGrpcPort,otlpHttpPort,internalMetricsPort,prometheusPort,healthPort}` | порты 4317/4318/8888/8889/13133 |
| `prometheus.retention`                      | `--storage.tsdb.retention.time` (15d / 6h dev / 30d prod) |
| `grafana.adminPassword`                     | пароль admin; пусто → генерируется `randAlphaNum 24` |
| `grafana.adminUser`                         | логин admin (по умолчанию `admin`) |
| `grafana.existingSecret` / `existingSecretKey` | готовый Secret с паролем (приоритетнее `adminPassword`) |
| `grafana.anonymous.{enabled,orgRole}`       | анонимный доступ (dev `true`/`Admin`, prod `false`) |
| `grafana.{logLevel,defaultTheme,featureToggles}` | env Grafana из compose |
| `grafana.ingress.{enabled,className,annotations,host,path,pathType,tls}` | Ingress (default `false`) |

### Переключатели

```bash
# отключить один компонент
--set loki.enabled=false

# отключить всё (рендерится пустой манифест)
--set grafana.enabled=false --set tempo.enabled=false --set loki.enabled=false \
--set prometheus.enabled=false --set otel-collector.enabled=false
```

При отключении компонента не создаётся **ни один** его объект (SA, ConfigMap,
Secret, workload, Service, Ingress). Исключение — `prometheus.config` содержит
статичные таргеты `otel-collector:8889/8888`: если отключить только коллектор,
Prometheus поднимется, но эти таргеты будут `down`. Уберите их или переопределите
`prometheus.config` целиком.

---

## 4. UID / securityContext (что выбрано и почему)

| Компонент        | runAsUser/Group | fsGroup | Обоснование |
| ---------------- | --------------- | ------- | ----------- |
| `otel-collector` | 10001           | 10001   | официальный distroless-образ `otel/opentelemetry-collector-contrib` работает от uid 10001 |
| `tempo`          | 10001           | 10001   | официальный образ `grafana/tempo` — uid 10001 |
| `loki`           | 10001           | 10001   | официальный образ `grafana/loki` — uid 10001 |
| `grafana`        | 472             | 472     | официальный образ `grafana/grafana` — uid/gid 472 |
| `prometheus`     | 65534           | 65534   | официальный образ `prom/prometheus` — `nobody` (65534) |

Для всех: `runAsNonRoot: true`, `seccompProfile: RuntimeDefault`, в контейнере
`allowPrivilegeEscalation: false` + `capabilities.drop: [ALL]`.

`fsGroup` выставлен равным uid, чтобы kubelet выставил групповое владение PVC и
запись в том работала.

`readOnlyRootFilesystem`:

* `otel-collector` — `true` (образ ничего не пишет);
* `tempo`, `loki`, `prometheus`, `grafana` — `false`. Эти процессы пишут
  данные в PVC (`/var/tempo`, `/loki`, `/prometheus`, `/var/lib/grafana`), но
  могут обращаться и к `/tmp`; с `true` возможны падения. Если ваш runtime это
  позволяет — включите `readOnlyRootFilesystem: true` и добавьте `emptyDir` на
  `/tmp`.

ServiceAccount создаётся для каждого компонента с
`automountServiceAccountToken: false` — ни один компонент не обращается к
Kubernetes API.

---

## 5. Персистентность

`tempo`, `loki`, `prometheus`, `grafana` — **StatefulSet** с
`volumeClaimTemplates` (том `data`):

| Компонент  | mountPath          | size по умолчанию | dev     | prod   |
| ---------- | ------------------ | ----------------- | ------- | ------ |
| tempo      | `/var/tempo`       | 10Gi              | emptyDir| 50Gi   |
| loki       | `/loki`            | 10Gi              | emptyDir| 50Gi   |
| prometheus | `/prometheus`      | 20Gi              | emptyDir| 100Gi  |
| grafana    | `/var/lib/grafana` | 5Gi               | emptyDir| 10Gi   |

* `persistence.enabled: false` → вместо PVC монтируется `emptyDir` (данные
  теряются при пересоздании пода);
* `persistence.storageClass: ""` → storageClass по умолчанию в кластере;
* `accessMode` по умолчанию `ReadWriteOnce`.

`otel-collector` — stateless `Deployment` без PVC.

> `replicaCount > 1` для tempo/loki/prometheus/grafana не поддерживается
> текущими конфигами: используется filesystem-хранилище, а у Loki
> `common.ring.instance_addr: 127.0.0.1`. Оставлено значение `1`.

---

## 6. Изменения, внесённые в конфиги из docker-compose

`tempo.yaml` и `loki.yaml` перенесены **без изменений** (проверено сравнением
YAML-деревьев с `infra/observability/*.yaml`). В `otel-collector` добавлено
только необходимое для работы в k8s:

1. `extensions.health_check.endpoint: 0.0.0.0:13133` и
   `service.extensions: [health_check]` — http-проба `/` для liveness/readiness
   (в compose healthcheck у коллектора отсутствовал).
2. `exporters.prometheus`: `endpoint: 0.0.0.0:8889`,
   `enable_open_metrics: true`,
   `resource_to_telemetry_conversion.enabled: true` — Go-сервисы пушат OTLP и
   **не** отдают `/metrics`, поэтому Prometheus скрейпит именно этот exporter,
   а `resource_to_telemetry_conversion` превращает resource-атрибуты
   (`service.name`, `deployment.environment`) в labels метрик.
3. Новая pipeline `metrics` (receivers `otlp`, processors
   `memory_limiter`+`batch`, exporters `[prometheus]`).
4. `service.telemetry.metrics.address: 0.0.0.0:8888` — собственные метрики
   коллектора (в compose порт публиковался, но адрес не задавался).

Датасорсы Grafana: Tempo и Loki перенесены без изменений (сравнение поэлементное),
добавлен третий datasource `Prometheus` (`uid: prometheus`,
`url: http://prometheus:9090`). `uid`'ы `tempo`/`loki` сохранены — на них
ссылаются `tracesToLogsV2`, `lokiSearch` и `derivedFields`.

`isDefault` оставлен у Tempo (как в compose).

Строка `url: "$${__value.raw}"` в derived fields оставлена дословно: `$$` —
это экранирование `$` для provisioning Grafana, менять нельзя.

---

## 7. Grafana

* пароль admin берётся из Secret:
  * `grafana.existingSecret` (ключ `grafana.existingSecretKey`, по умолчанию
    `admin-password`) — рекомендуется для prod;
  * иначе создаётся Secret `grafana-admin` из `grafana.adminPassword`; если
    значение пустое — генерируется случайный `randAlphaNum 24`. **Важно:** без
    явного `adminPassword`/`existingSecret` пароль меняется при каждом
    `helm upgrade`. Забрать текущий:
    ```bash
    kubectl -n <ns> get secret grafana-admin \
      -o jsonpath='{.data.admin-password}' | base64 -d
    ```
* анонимный доступ: `grafana.anonymous.enabled` (dev `true` + роль `Admin`,
  как в compose; prod `false`). При включении также выставляется
  `GF_AUTH_DISABLE_LOGIN_FORM=true`.
* прочие env из compose: `GF_USERS_DEFAULT_THEME=dark`, `GF_LOG_LEVEL`,
  `GF_FEATURE_TOGGLES_ENABLE=traceQLStreaming`.
* datasource'ы монтируются из ConfigMap `grafana-datasources` в
  `/etc/grafana/provisioning/datasources` (read-only).
* Ingress — опционально (`grafana.ingress.enabled`, default `false`), в
  `values-prod.yaml` показан включённый пример.

---

## 8. Пробы

| Компонент        | liveness / readiness            | Порт  |
| ---------------- | ------------------------------- | ----- |
| `otel-collector` | `GET /`                         | 13133 |
| `tempo`          | `GET /ready`                    | 3200  |
| `loki`           | `GET /ready`                    | 3100  |
| `grafana`        | `GET /api/health`               | 3000  |
| `prometheus`     | `GET /-/healthy`, `GET /-/ready`| 9090  |

Дополнительно при изменении ConfigMap меняется аннотация
`checksum/config` (или `checksum/datasources` у Grafana) — поды
перезапускаются автоматически.

---

## 9. Валидация

Выполнено локально (`helm` v4.3.0, `/snap/helm/545/helm`):

```bash
/snap/helm/545/helm lint deploy/helm/observability
# ==> Linting deploy/helm/observability
# [INFO] Chart.yaml: icon is recommended
# 1 chart(s) linted, 0 chart(s) failed

/snap/helm/545/helm template obs deploy/helm/observability -f deploy/helm/observability/values-dev.yaml
/snap/helm/545/helm template obs deploy/helm/observability -f deploy/helm/observability/values-prod.yaml
/snap/helm/545/helm template obs deploy/helm/observability \
  --set grafana.enabled=false --set tempo.enabled=false --set loki.enabled=false \
  --set prometheus.enabled=false --set otel-collector.enabled=false
```

Все команды завершились с кодом `0`, без ошибок. Дополнительно проверено:
каждый `--set <component>.enabled=false` по отдельности; отсутствие
`metadata.namespace` во всех отрендеренных объектах; совпадение
tempo/loki/grafana-datasources с исходными compose-конфигами.

> Локальный `helm` в PATH сломан — всегда используйте `/snap/helm/545/helm`.

---

## 10. Что НЕ проверено (нужен живой кластер)

Запуск в кластере не выполнялся (только `lint`/`template`). При деплое
проверьте:

* совместимость uid/gid из §4 с вашим runtime (если образ собран иначе —
  переопределите `*.podSecurityContext`);
* работу проб: `kubectl -n <ns> get pods`, `kubectl -n <ns> describe pod ...`;
* что StorageClass из `persistence.storageClass` существует и поддерживает
  `ReadWriteOnce`;
* что Grafana стартует с PVC и `fsGroup: 472`;
* сквозной путь данных: OTLP от Go-сервисов → Tempo/Loki, и что Prometheus
  видит таргеты `otel-collector:8889`/`8888` в состоянии `up`
  (`http://prometheus:9090/targets`);
* ingress (в prod включён) и наличие TLS-секрета `grafana-tls`.
