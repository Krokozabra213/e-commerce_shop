# Развёртывание e-commerce_shop в Kubernetes

Helm-чарты для микросервисов `e-commerce_shop`. Всё, что нужно для запуска в
кластере: инфраструктура, сервисы, миграции, ingress, HPA/PDB/NetworkPolicy и
observability.

```
deploy/helm/
├── README.md                      ← этот файл
├── Makefile                       ← lint / template / validate / install
├── deploy.sh                      ← установка «одной командой» (3 релиза по порядку)
├── infra/                         ← чарт: PostgreSQL ×4, Redis, MongoDB, Kafka, Schema Registry
├── ecommerce-shop/                ← чарт: 6 сервисов + миграции goose
├── observability/                 ← чарт: otel-collector, tempo, loki, prometheus, grafana
└── scripts/
    ├── build-images.sh            ← сборка образов сервисов и миграций
    ├── validate-render.sh         ← lint + render + проверки всех чартов/профилей
    └── validate-config-contract.sh← конфиги из ConfigMap прогоняются через реальный Go-код
```

---

## 1. Быстрый старт (minikube / kind)

```bash
# 0) из корня репозитория
cd /path/to/e-commerce_shop

# 1) собрать образы (10 штук: 6 сервисов + 4 образа миграций)
REGISTRY=ghcr.io PROJECT=<your-org> TAG=1.0.0 PUSH=1 deploy/helm/scripts/build-images.sh

#    для kind/minikube без реестра:
deploy/helm/scripts/build-images.sh          # локальные образы
kind load docker-image $(deploy/helm/scripts/build-images.sh --print-tags)

# 2) установить всё
deploy/helm/deploy.sh                        # ENV=dev по умолчанию

# 3) проверить
kubectl get pods,svc,ingress -n ecommerce
```

`deploy.sh` делает три вещи **по порядку**:

| # | Релиз | Что ставит |
|---|-------|-----------|
| 1 | `ecommerce-infra` | Postgres ×4, Redis, MongoDB, Kafka (KRaft), Schema Registry, Job создания топиков |
| 2 | `observability` | otel-collector, tempo, loki, prometheus, grafana |
| 3 | `ecommerce` | 6 сервисов + Job'ы миграций goose |

Через `make`:

```bash
cd deploy/helm
make lint          # helm lint всех чартов
make validate      # lint + render + проверка контракта конфигов через Go
make install       # namespace + infra + observability + app
make status
make logs SVC=api-gateway
```

---

## 2. Почему три отдельных релиза, а не один umbrella-чарт

Первоначальный сгенерированный чарт был одним релизом со всей
инфраструктурой внутри. Это принципиально ломает миграции:

* Миграции выполняются **Helm-хуком** и должны видеть живую БД.
* `pre-install`-хуки выполняются **до** создания ресурсов релиза, поэтому БД
  из того же релиза ещё не существует.
* `post-install`-хуки выполняются **после** создания подов приложения —
  значит, поды стартуют на ещё не мигрированной схеме.

Разделив инфраструктуру и приложение на разные релизы, мы получаем корректный
порядок: `infra` готов → `pre-upgrade`-хук миграций применяет схему → только
потом обновляются поды приложения.

Дополнительный плюс: в production инфраструктуру почти всегда заменяют на
managed-сервисы (RDS/ElastiCache/Atlas/MSK). Тогда `infra`-чарт просто не
ставится (`WITH_INFRA=0`), а в `ecommerce-shop/values-prod.yaml` указываются
внешние адреса — приложение об этом ничего не знает.

---

## 3. Как сервисы читают конфигурацию (главное, что нужно понимать)

Это ядро всего чарта. Код каждого сервиса:

```go
// services/<svc>/internal/config/config.go
func Init() (*Config, error) {
    _ = godotenv.Load()                                  // .env в контейнере отсутствует
    env := os.Getenv("ENV"); if env == "" { env = "dev" }
    configFile := fmt.Sprintf("configs/%s.yaml", env)    // ПУТЬ ОТНОСИТЕЛЬНО CWD
    if _, err := os.Stat(configFile); os.IsNotExist(err) {
        return nil, fmt.Errorf("config file not found: %s (ENV=%s)", configFile, env)
    }
    var cfg Config
    if err := cleanenv.ReadConfig(configFile, &cfg); err != nil { return nil, err }
    return &cfg, cfg.Validate()
}
```

Отсюда три жёстких требования к манифестам:

1. **Файл `configs/<ENV>.yaml` обязан существовать** — иначе сервис падает на
   старте. Поэтому чарт рендерит свой ConfigMap и монтирует его в `/app/configs`,
   а `ENV` задаётся переменной окружения.
2. **`ENV` и имя ключа ConfigMap'а должны совпадать.** Чарт рендерит ключ
   `{{ .Values.global.environment }}.yaml` и ставит ту же строку в `ENV`.
3. **Переменные окружения переопределяют YAML** (cleanenv: сначала YAML, потом
   env). Именно так передаются секреты — они не попадают в ConfigMap.

### Семантика cleanenv, которую нужно знать точно

Проверено по исходникам `cleanenv@v1.5.0`:

| Правило | Следствие |
|---|---|
| env-переменная читается **только** для полей с тегом `env:"..."` | поля без тега (`OutboxConfig`, `RateLimitRule`, `KafkaConsumerConfig.SessionTimeout`, …) настраиваются **только** через YAML |
| имя переменной = `env-prefix` + значение `env`-тега, **без** разделителя и без изменения регистра | `RedisConfig.Addr` + префикс `REDIS_` → `REDIS_ADDR`; `PostgresConfig.DBName` → `POSTGRES_DBNAME` (не `POSTGRES_DB_NAME`!) |
| `env-required` падает, только если поле пустое **и** в YAML, **и** в env | YAML из ConfigMap'а «закрывает» required-поля; секреты закрываются env-переменными |
| `env-prefix` накапливается по вложенным структурам | `OAUTH_` + `GOOGLE_` + `CLIENT_ID` → `OAUTH_GOOGLE_CLIENT_ID` |

Поэтому в чарте **вся несекретная конфигурация лежит в ConfigMap** (естественный
YAML, никаких уродливых имён вида `ORDER_CREATED_CONSUMERBROKERS`), а через env
передаются только секреты.

### Таблица: секреты → env-переменная → ключ Secret'а

| Сервис(ы) | env-переменная | ключ в Secret'е |
|---|---|---|
| все | `APP_SECRET` | `app-secret` |
| auth | `POSTGRES_PASSWORD` | `postgres-auth-password` |
| user | `POSTGRES_PASSWORD` | `postgres-user-password` |
| inventory | `POSTGRES_PASSWORD` | `postgres-inventory-password` |
| order | `POSTGRES_PASSWORD` | `postgres-order-password` |
| api-gateway, auth | `REDIS_PASSWORD` | `redis-password` |
| product | `MONGO_URI` | `mongodb-uri` |
| auth | `OAUTH_GOOGLE_CLIENT_ID` / `_CLIENT_SECRET` / `_REDIRECT_URL` | `oauth-google-*` |
| auth | `OAUTH_GITHUB_CLIENT_ID` / `_CLIENT_SECRET` / `_REDIRECT_URL` | `oauth-github-*` |
| auth | (файл) `AUTH_JWT_PRIVATE_KEY_PATH` | `jwt-private-key` → `/etc/ecommerce/certs/jwt-private.pem` |

Особые случаи, найденные при разборе кода:

* `MongoDBConfig.URI` помечен `yaml:"-"` — URI **невозможно** задать из YAML,
  только через env `MONGO_URI`.
* `SchemaRegistryConfig.URL` не имеет env-тега — задаётся только из YAML
  (`schema_registry.url`).
* `OAuthProviderConfig.ClientID/ClientSecret/RedirectURL` помечены
  `env-required` и валидируются **всегда**, даже при `oauth.*.enabled=false`.
  Поэтому, если OAuth выключен, в Secret'е всё равно должны быть непустые
  значения-заглушки (по умолчанию `disabled`) — см. §11, п. 1.
* `RateLimitConfig` и `OutboxConfig` настраиваются только из YAML.

### Проверка контракта конфигов (без кластера)

```bash
deploy/helm/scripts/validate-config-contract.sh                  # values-dev.yaml
deploy/helm/scripts/validate-config-contract.sh deploy/helm/ecommerce-shop/values-prod.yaml
```

Скрипт рендерит чарт, вытаскивает конфиги из ConfigMap'ов и **прогоняет их через
настоящий `config.Init()`** каждого сервиса (компилирует временный Go-тест),
подставляя те же env-переменные, что и в кластере. Это ловит опечатки в
YAML-ключах, забытые required-поля и неверные имена env-переменных.

---

## 4. Соглашение об именах (важно!)

**Service'ы называются коротко и не зависят от имени релиза:**

```
auth-service:8100   inventory-service:8400   redis:6379   kafka:29092
```

Это не косметика: конфиги в `files/configs/*.yaml` ссылаются друг на друга
именно такими именами, как и `infra/observability`-конфиги
(`kafka:29092`, `http://schema-registry:8081`, `tempo:4317`).

Следствия:

* **Один релиз каждого чарта на namespace.** Нельзя поставить два релиза
  `ecommerce-shop` в один namespace — Service'ы столкнутся.
* Если имена сервисов нужны другие — правьте `dependencies.services.*`
  (и `dependencies.*` для инфраструктуры) в values, но тогда придётся
  синхронизировать это с реальными Service'ами.

`Deployment`, `Service`, `Ingress`, `HPA`, `PDB` сервиса имеют одинаковое имя
(`auth-service`). `ConfigMap` → `<svc>-config`, `Secret` → `<release>-secrets`,
`ServiceAccount` → `<release>-app`, `Job` миграций → `<svc>-migrate`.

В infra- и observability-чартах у каждого StatefulSet'а есть **дополнительный
headless-сервис** `<name>-headless` (`clusterIP: None`,
`publishNotReadyAddresses: true`). Он нужен как governing service для
StatefulSet и для per-pod DNS (`<pod>.<name>-headless`); обычный ClusterIP-сервис
с тем же базовым именем остаётся точкой входа для клиентов.

---

## 5. Значения (values): что где настраивать

### `ecommerce-shop` — ключевые секции

| Ключ | Назначение |
|---|---|
| `global.imageRegistry` | **только хост** реестра (`registry.example.com`). Путь/проект — в `image.repository` (`ecommerce/api-gateway`) |
| `global.imagePullSecrets` | pull-секреты реестра |
| `global.environment` | `ENV`/`APP_ENV` и имя ключа ConfigMap'а (`dev.yaml`) |
| `global.waitTimeoutSeconds` | сколько init-контейнеры ждут зависимости |
| `dependencies.*` | адреса Postgres/Redis/Mongo/Kafka/Schema Registry |
| `dependencies.services.*` | адреса сервисов друг для друга (используются в конфигах) |
| `observability.*` | секция `telemetry` в конфигах (endpoint, sample rate, …) |
| `logging.*` | секция `slog` (level/format/add_source) |
| `rateLimiter.*` | секция `rate_limiter` (только YAML, env-тегов нет) |
| `kafkaConsumer.*`, `kafkaProducer.*`, `outbox.*` | общие параметры Kafka/outbox |
| `auth.*` | JWT/OAuth/email-verification для auth-service |
| `secrets.*` | создание Secret'а чартом (`create`) или использование существующего (`existingSecret`) |
| `defaults.*` | ресурсы, securityContext, probes по умолчанию (сервис может переопределить) |
| `migrations.*` | режим/хук/лимиты Job'ов миграций |
| `ingress.*` | мастер-выключатель, className, аннотации, TLS |
| `networkPolicy.*` | включение, default-deny, доп. правила |
| `services.<name>.*` | per-service: replicas, image, ports, waitFor, database, secrets-флаги, resources, autoscaling, pdb, ingress, tolerations… |

`defaults` применяются, только если у сервиса ключ **не задан** (проверка через
`hasKey`, поэтому `false`/`0`/`""` не «съедаются» как в sprig `default`).

### `infra` — ключевые секции

`postgresql` (в т.ч. `instances.{auth,inventory,order,user}`), `redis`,
`mongodb`, `kafka` (+`topics.list`), `schemaRegistry`. У всех есть
`enabled`, `persistence.{enabled,size,storageClass}`, `resources`, `auth`.

### `observability` — ключевые секции

`otel-collector`, `tempo`, `loki`, `prometheus`, `grafana` — каждый со своими
`enabled`, `persistence`, `resources`. Подробности — в
`deploy/helm/observability/README.md`.

---

## 6. Секреты

### dev / staging (по умолчанию)

```bash
# ключ генерируется автоматически, если его нет (deploy.sh)
deploy/helm/deploy.sh
```

Secret создаётся чартом из `secrets.values` и передаётся так:

```bash
helm upgrade --install ecommerce deploy/helm/ecommerce-shop \
  -n ecommerce -f deploy/helm/ecommerce-shop/values-dev.yaml \
  --set-file secrets.values.jwtPrivateKey=services/auth-service/certs/jwt-private.pem
```

> `helm lint`/`helm template` по умолчанию **падают**, если не передан
> `secrets.values.jwtPrivateKey`. Это сделано намеренно (fail-fast вместо
> непонятного CrashLoopBackOff у auth-service). Для lint используйте:
> `make lint` из `deploy/helm` — он подставляет `--set-file` автоматически.

### production

```bash
kubectl -n ecommerce create secret generic ecommerce-shop-secrets \
  --from-literal=app-secret="$(openssl rand -hex 32)" \
  --from-literal=postgres-auth-password="$(openssl rand -hex 24)" \
  --from-literal=postgres-user-password="$(openssl rand -hex 24)" \
  --from-literal=postgres-inventory-password="$(openssl rand -hex 24)" \
  --from-literal=postgres-order-password="$(openssl rand -hex 24)" \
  --from-literal=redis-password="$(openssl rand -hex 24)" \
  --from-literal=mongodb-uri="mongodb://user:pass@mongo:27017/products?authSource=admin" \
  --from-literal=jwt-private-key="$(cat services/auth-service/certs/jwt-private.pem)" \
  --from-literal=oauth-google-client-id="..." \
  --from-literal=oauth-google-client-secret="..." \
  --from-literal=oauth-google-redirect-url="https://api.example.com/api/v1/auth/oauth/google/callback" \
  --from-literal=oauth-github-client-id="..." \
  --from-literal=oauth-github-client-secret="..." \
  --from-literal=oauth-github-redirect-url="https://api.example.com/api/v1/auth/oauth/github/callback"
```

и в values: `secrets.create=false`, `secrets.existingSecret=ecommerce-shop-secrets`.

Чарт **специально падает** (`fail`), если:
* `secrets.create=false`, но `secrets.existingSecret` пуст;
* `global.environment=prod` и `secrets.create=true` (обходится
  `secrets.allowCreateInProduction=true`);
* `secrets.create=true`, но `secrets.values.jwtPrivateKey` пуст, а auth-service включён.

Для инфраструктуры аналогично: `postgresql-credentials` (ключи
`auth-password`, `inventory-password`, `order-password`, `user-password`),
`redis-credentials`, `mongodb-credentials`.

> В репозитории есть `services/*/.env` с реальными OAuth-секретами (Google/
> GitHub). Они не попадают в git (`.gitignore`, `.gitleaks.toml`), но если эти
> значения когда-либо были опубликованы — их следует отозвать и перевыпустить.
> В чарт они не переносятся: см. подстановки-заглушки `disabled`.

---

## 7. Миграции goose

* Образы миграций: `services/<svc>/Migrate.Dockerfile` → `<registry>/<project>/<svc>-migrate:<tag>`.
* Запускаются **Job'ами** (по одному на сервис с БД): auth, user, inventory, order.
* Хуки по умолчанию: `post-install,pre-upgrade`, вес `-5`,
  `hook-delete-policy: before-hook-creation` (Job иммутабелен, поэтому
  пересоздаётся).
* Пароль передаётся через `GOOSE_DBSTRING` с `$(POSTGRES_PASSWORD)` —
  Kubernetes сам подставляет значение из `secretKeyRef`, пароль не попадает в
  манифест.
* init-контейнер (`busybox`, `nc -z`) ждёт доступности БД, `backoffLimit: 3`.

**Почему `post-install`, а не `pre-install`:** Job читает пароль из Secret'а
чарта, а `pre-install`-хуки выполняются до создания Secret'а. Цена — при самой
первой установке поды приложения стартуют раньше, чем применится схема. Для
нового кластера это безопасно (трафик ещё не идёт, `/readyz` проверяет только
доступность БД, а не схему). Если строгий порядок нужен и на первой установке:
создайте Secret заранее (`secrets.create=false`) и выставьте
`migrations.hook="pre-install,pre-upgrade"` (так сделано в `values-prod.yaml`).

`Migrate.Dockerfile` пинит goose `v3.22.1`. CLI `goose up` **не** использует
advisory-lock (в v3.27 появился только в Go-API с `WithSessionLocker`), поэтому
миграции намеренно запускаются одним Job'ом, а не init-контейнером каждого пода:
при `replicaCount > 1` init-контейнеры гонялись бы наперегонки.

---

## 8. Kafka

Топики создаются Job'ом `kafka-topics` (хук `post-install,post-upgrade`).
Список — `infra.kafka.topics.list`, полный набор из `infra/kafka/topics.go` и
`dlqTopic` в конфигах сервисов:

```
user.created[.dlq]                  order.created[.dlq]
order.cancel-inventory[.dlq]        inventory.reserved[.dlq]
inventory.reservation-failed[.dlq]  order.create-payment
payment.succeded                    payment.failed
```

> **Найденный баг в `docker-compose.yaml`:** в `kafka-init` создавались только
> 10 топиков из 13 — отсутствовали `order.create-payment`, `payment.succeded`,
> `payment.failed`, в которые публикуется `order-service`
> (`internal/service/order/handle_*.go`). Работало только за счёт
> `auto.create.topics.enable=true` в Confluent-образе. В compose добавлены все 13.

Kafka — single-node KRaft. `CLUSTER_ID` **нельзя менять** для уже
отформатированного PVC (брокер не стартует). Для HA используйте Strimzi и
`kafka.enabled=false` (тогда для Schema Registry задайте
`schemaRegistry.kafkaBootstrapServers`).

Controller quorum адресуется через per-pod headless DNS
(`1@kafka-0.kafka-headless:9093`), а не через ClusterIP: у ClusterIP нет
endpoints, пока под не Ready, поэтому брокер не смог бы собрать quorum и
никогда не стал бы Ready.

---

## 9. Метрики и observability

**Важно:** сервисы **не отдают `/metrics`** — они пушат OTLP в otel-collector
(`TELEMETRY_ENDPOINT`). Поэтому Prometheus скрейпит
`otel-collector:8889` (prometheus-exporter коллектора), а не поды приложения.
`ServiceMonitor` не используется.

Эндпоинты health: `/healthz` (liveness + startup) и `/readyz` (readiness).
`/readyz` проверяет зависимости сервиса (Postgres/Mongo/Redis/Kafka/Schema
Registry) — при недоступности сервис выпадает из балансировки.

`observability.enabled=false` (dev) → в конфигах `telemetry.enabled: false`.
Включение при недоступном коллекторе не ломает старт
(`grpc.NewClient` ленивый), но трейсы/логи не будут экспортироваться.

---

## 10. NetworkPolicy

Выключены по умолчанию. При `networkPolicy.enabled=true` создаётся
namespace-wide `default-deny-ingress` + правила:

* вход в `api-gateway` — откуда угодно (ingress-controller живёт в другом namespace);
* вход в остальные сервисы — от любых подов чарта (`part-of=ecommerce-shop`);
* вход в инфраструктуру — от приложения и от самой инфраструктуры
  (Kafka ↔ Schema Registry);
* вход в observability — от любых подов namespace (OTLP, tempo/loki, scrape).

Egress-правила не задаются, поэтому DNS не ломается. Требуется CNI с поддержкой
NetworkPolicy (Calico/Cilium/Antrea); в kind/minikube по умолчанию их нет — там
включение ничего не даст, но и не сломает. Свои источники трафика добавляйте
через `networkPolicy.extraIngressRules`.

---

## 11. Что было исправлено и что осталось на код

### 11.1 Исправлено при подготовке чарта

| # | Проблема | Где было | Что сделано |
|---|---|---|---|
| 1 | **Приватный ключ JWT попадал в образ auth-service** — `COPY . .` + `COPY certs` при сборке из корня репозитория | `services/auth-service/Dockerfile` | копирование `certs/` убрано (ключ монтируется из Secret'а), добавлен корневой `.dockerignore` |
| 2 | **Отрендеренные манифесты с реальным ключом** попадали в `deploy/helm/build/` (не в .gitignore) | `scripts/validate-render.sh` | рендер идёт с фиктивным ключом, `build/` и `deploy/.tools` добавлены в `.gitignore` |
| 3 | **Несколько Kafka-брокеров склеивались в один адрес**: строка `a:9092,b:9092` попадала в YAML как один элемент списка | `ecommerce-shop/templates/_helpers.tpl` | `splitList ","` — прод-профиль с 3 брокерами теперь валиден (иначе `/readyz` никогда бы не прошёл) |
| 4 | **Job'ы миграций не имели `imagePullSecrets`** → в prod с приватным реестром падал весь install | `ecommerce-shop/templates/migrations.yaml`, `infra/templates/kafka-topics.yaml` | добавлены `global.imagePullSecrets` |
| 5 | **`waitImage` получал префикс реестра без проекта** (`registry.example.com/busybox:1.36`) | `_helpers.tpl` обоих чартов | `waitImage` — полная ссылка, автопрефикс убран |
| 6 | **`kafka.topics.partitions` в prod не работал** (перекрывался `partitions: 3` у каждого топика) | `infra/values*.yaml` | per-topic `partitions` убраны, применяется верхнеуровневый (в prod = 6) |
| 7 | **KRaft controller quorum шёл через ClusterIP** — endpoints пусты, пока под не Ready → брокер не мог стать Ready | `infra/templates/kafka.yaml` | voter адресуется через per-pod headless DNS (`kafka-0.kafka-headless:9093`) + `publishNotReadyAddresses` |
| 8 | **StatefulSet'ы ссылались на обычный ClusterIP-сервис** как на governing service (не было headless) | `infra` (4) и `observability` (4) | добавлены `<svc>-headless` (`clusterIP: None`), `spec.serviceName` переведён на них |
| 9 | **Нет `seccompProfile`** → поды отклонялись бы в namespace с Pod Security Standard `restricted` | `ecommerce-shop`, `infra` | `seccompProfile: RuntimeDefault` (+ `runAsNonRoot: true` для infra) |
| 10 | **`pdb.minAvailable: 0` молча превращался в 1** (sprig `default` считает 0 пустым) | `ecommerce-shop/templates/pdb.yaml` | разбор через `hasKey` |
| 11 | **Schema Registry нельзя было направить на внешний Kafka** при `kafka.enabled=false` | `infra/templates/schema-registry.yaml` | добавлен `schemaRegistry.kafkaBootstrapServers`, init-контейнер ожидания включается только при `kafka.enabled=true` |
| 12 | **`secrets.create=false` без `existingSecret`** молча ссылался на несуществующий Secret | `ecommerce-shop` | `templates/validate.yaml` c `fail` и понятным текстом |
| 13 | **prod с `secrets.create=true`** (пароли из values) | `ecommerce-shop` | fail-fast, обходится `secrets.allowCreateInProduction=true` |
| 14 | **nil-pointer при отсутствии необязательных блоков** (`autoscaling`/`pdb`/`ingress`/`kafka.topics`) | несколько шаблонов | `| default dict` + `required` с внятными сообщениями |
| 15 | **`api-gateway` gRPC-адрес inventory был неверным**: `auth-service:44100` | `services/api-gateway/configs/dev.yaml` | исправлено на `inventory-service:44100` (у auth-service нет gRPC-сервера) |
| 16 | **`services/auth-service/certs/.gitkeep` отсутствовал** → `COPY certs` падал в свежем клоне | репозиторий | `.gitkeep` добавлен |
| 17 | **В `docker-compose.yaml` не хватало 3 Kafka-топиков**, в которые публикуется order-service | `docker-compose.yaml` | добавлены `order.create-payment`, `payment.succeded`, `payment.failed` (см. §8) |
| 18 | Bash: пустой массив при `set -u` ломал `ENV=prod ./deploy.sh` на bash < 4.4; `DRY_RUN` создавал файл ключа | `deploy.sh` | `${arr[@]+"${arr[@]}"}`, генерация ключа пропускается при `DRY_RUN=1` |

### 11.2 Рекомендации по коду (в чартах не обходится)

1. **OAuth-поля обязательны даже при выключенном OAuth.**
   `OAuthProviderConfig.{ClientID,ClientSecret,RedirectURL}` помечены
   `env-required` безусловно, поэтому при `oauth.*.enabled=false` нужны
   непустые заглушки (по умолчанию `disabled`). *Рекомендация:* проверять
   обязательность только при `enabled=true`.

2. **`api-gateway` не вызывает `JWTClient.Validate()`.** В
   `services/api-gateway/internal/config/config.go` `Validate()` проверяет
   только `Logger` и `Redis`; `jwt.publicKeyPath`/`jwksEndpoint` не валидируются.
   Сейчас не мешает (ключ берётся у auth-service по HTTP), но валидация мертва.

3. **`auth-service` `/readyz` не возвращает 500 при недоступном Redis.** В
   `internal/features/health/handler.go` ошибка Redis выставляет
   `c.Status(500)`, но финальный `c.Status(statusCode).JSON(...)` перезаписывает
   его на 200.

4. **`Migrate.Dockerfile` пинит goose v3.22.1**, локально установлен v3.27.2.
   Возможно, стоит зафиксировать одну версию во всех местах.

5. **`notification-service` — пустой модуль** (только `go.mod` и пустой
   `Makefile`), но перечислен в `go.work`. В чарт не включён.

6. **`ENV` vs `APP_ENV`.** Файл конфига выбирается по «голому» `ENV`, а
   `AppConfig.ENV` читается из `APP_ENV`. Чарт выставляет обе переменные —
   стоит унифицировать в коде.

## 12. Устранение неполадок

| Симптом | Причина / что делать |
|---|---|
| `config file not found: configs/dev.yaml` | ConfigMap не смонтирован или `global.environment` не совпадает с ключом. Проверьте `kubectl get cm <svc>-config -o yaml` |
| Pod в CrashLoopBackOff, `field "..." is required but the value is not provided` | не передана env-переменная из Secret'а либо опечатка в имени. Сверьтесь с §3 |
| `open .env: no such file or directory` в логах | **не ошибка**: `godotenv.Load()` не находит `.env` в контейнере и просто логирует |
| `auth-service`: `failed to load private key` | пустой/битый `secrets.values.jwtPrivateKey` либо не смонтирован Secret. Проверьте `AUTH_JWT_PRIVATE_KEY_PATH` и `kubectl get secret ... -o jsonpath='{.data.jwt-private-key}' \| base64 -d \| head -1` |
| `errored, unable to authenticate` у Kafka-консьюмеров | топики не созданы (Job `kafka-topics`), либо `dependencies.kafka.brokers` неверны |
| Schema Registry бесконечно перезапускается | Kafka ещё не готова; init-контейнер ждёт порт, проверьте `kubectl logs deploy/schema-registry -c wait-for-kafka` |
| `unsupported image` / образ не найден | `global.imageRegistry` — **только хост**; путь уже в `image.repository` |
| Миграции не запускаются | это хуки: `helm upgrade` их выполняет, `helm template` — только рендерит. Смотрите `kubectl get jobs`, `helm get hooks <release>` |
| Postgres-под не стартует после смены `CLUSTER_ID`/`PGDATA` | PVC уже отформатирован; пересоздайте PVC (`kubectl delete pvc <name>`) |

---

## 13. Что проверено и что нет

Проверено (воспроизводимо, без кластера):

* `helm lint` всех трёх чартов — 0 ошибок.
* `helm template` всех чартов в профилях `values.yaml`, `values-dev.yaml`,
  `values-prod.yaml` — весь вывод валидный YAML, `metadata.namespace` нигде не
  захардкожен.
* Выключение всех компонентов/сервисов каждого чарта даёт пустой рендер;
  отключение каждого сервиса по отдельности рендерится без ошибок.
* **Контракт конфигов:** все 6 отрендеренных `<env>.yaml` (dev и prod) успешно
  загружаются настоящим `config.Init()` соответствующего сервиса с теми же
  env-переменными, что задаёт Deployment. Плюс негативный тест (испорченный
  ключ → падение) подтверждает, что проверка не «пустая».
* `docker`-образы `postgres:16-alpine` содержат `pg_ctl`/`pg_isready`
  (используются в `preStop`/пробах).
* **Схемы Kubernetes (kubeconform):** 186 ресурсов во всех профилях трёх
  чартов — `Invalid: 0, Errors: 0`. Именно эта проверка поймала дублирующийся
  ключ `app.kubernetes.io/component` в метках Job'ов миграций (PyYAML такое
  молча проглатывает, поэтому в `validate-render.sh` добавлен детектор дублей).
* Проверки «отрицательных» сценариев: fail-fast на пустом ключе JWT,
  `secrets.create=false` без `existingSecret`, `prod + secrets.create=true`,
  `migration.enabled` без `database`; отсутствие необязательных блоков
  (`autoscaling`/`pdb`/`ingress`/`kafka.topics` = `null`).

**Не проверено — нужен живой кластер:**

* реальный старт подов, прохождение проб, поведение `readOnlyRootFilesystem`
  и `runAsNonRoot: 65534` для каждого образа;
* фактические uid/gid, требуемые образами infra (`postgres` 999, `redis` 999,
  `mongo` 999, `cp-kafka` 1000) на конкретном CSI-драйвере;
* корректность схем Kubernetes (kubeconform) и admission-политики кластера;
* сквозной сценарий: регистрация пользователя → заказ → резерв товара;
* наблюдаемость: OTLP → Tempo/Loki, скрейп otel-collector, дашборды Grafana;
* NetworkPolicy в реальном CNI.

Команды для проверки в вашем кластере:

```bash
# схемы Kubernetes без кластера
go install github.com/yannh/kubeconform/cmd/kubeconform@latest
deploy/helm/scripts/validate-render.sh      # подхватит kubeconform автоматически

# полноценная установка на minikube
minikube start --cpus 4 --memory 6144
deploy/helm/deploy.sh
kubectl get pods -n ecommerce -w

# server-side проверка манифестов без установки
deploy/helm/deploy.sh DRY_RUN=1             # печатает команды
helm template ecommerce deploy/helm/ecommerce-shop -f deploy/helm/ecommerce-shop/values-dev.yaml \
  --set-file secrets.values.jwtPrivateKey=services/auth-service/certs/jwt-private.pem \
  | kubectl apply --dry-run=server -f -
```

---

## 14. Удаление

```bash
helm uninstall ecommerce -n ecommerce
helm uninstall observability -n ecommerce
helm uninstall ecommerce-infra -n ecommerce
```

> **PVC не удаляются** вместе с релизом (созданы через `volumeClaimTemplates`).
> Удалите вручную, если данные больше не нужны:
> `kubectl delete pvc -n ecommerce -l app.kubernetes.io/part-of=ecommerce-infra`

`secrets.create=true` удаляет Secret вместе с релизом; Secret, созданный вручную
(`existingSecret`), останется — удалите его отдельно.
