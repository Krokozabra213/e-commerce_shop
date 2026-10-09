# Развёртывание e-commerce_shop в Kubernetes

Helm-чарты для микросервисов `e-commerce_shop`. Всё, что нужно для запуска в
кластере: инфраструктура, сервисы, миграции, ingress, HPA/PDB/NetworkPolicy и
observability.

Короткая памятка по всему циклу (проверки перед пушем, запуск после
клонирования, переключение профилей и версий) — [`CHECKLIST.md`](../../CHECKLIST.md)
в корне репозитория.

```
deploy/helm/
├── README.md                      ← этот файл (устройство чартов)
├── DEPLOY-RUNBOOK.md              ← пошаговый запуск: GHCR + k3s + секреты
├── Makefile                       ← lint / template / validate / install
├── deploy.sh                      ← установка «одной командой» (3 релиза по порядку)
├── infra/                         ← чарт: PostgreSQL ×4, Redis, MongoDB, Kafka, Schema Registry
├── ecommerce-shop/                ← чарт: 6 сервисов + миграции goose
├── observability/                 ← чарт: otel-collector, tempo, loki, prometheus, grafana
└── scripts/
    ├── build-images.sh            ← сборка образов сервисов и миграций
    ├── create-app-secret.sh       ← Secret из services/*/.env + ключа JWT
    ├── create-ghcr-pull-secret.sh ← pull-секрет для приватных пакетов GHCR
    ├── create-tls-secret.sh       ← самоподписанный TLS-сертификат для ingress
    ├── set-oauth-redirect.sh      ← переключить OAuth-redirect localhost ↔ домен
    ├── bump-version.sh            ← обновить версию (appVersion) в одном месте
    └── validate-config.sh         ← helm template dev/prod + проверка YAML
```

---

## 1. Быстрый старт (k3s)

Нужны `docker`, `kubectl`, `helm` и k3s. Из корня репозитория:

```bash
# 1) k3s как systemd-служба — без Traefik (иначе он займёт 80/443)
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik" sh -
mkdir -p ~/.kube && sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config && sudo chown "$(id -u):$(id -g)" ~/.kube/config

# 2) ingress-nginx
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx && helm repo update
helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx -n ingress-nginx --create-namespace -f deploy/cluster/ingress-nginx-values.yaml --set controller.config.hsts=false --wait --timeout 10m

# 3) секреты приложения (приватный ключ JWT генерируется автоматически) и TLS
REDIRECT_BASE=http://localhost deploy/helm/scripts/create-app-secret.sh
HOST=api.ecommerce.local deploy/helm/scripts/create-tls-secret.sh
kubectl apply -f deploy/cluster/ingress-localhost.yaml

# 4) инфраструктура
helm upgrade --install ecommerce-infra deploy/helm/infra -n ecommerce --create-namespace -f deploy/helm/infra/values-dev.yaml --wait --timeout 20m

# 5) приложение (образы из GHCR — собирать ничего не нужно)
helm upgrade --install ecommerce deploy/helm/ecommerce-shop -n ecommerce -f deploy/helm/ecommerce-shop/values-dev.yaml -f deploy/helm/ecommerce-shop/values-local-tls.yaml -f deploy/helm/ecommerce-shop/values-ghcr.yaml --set secrets.create=false --set secrets.existingSecret=ecommerce-shop-secrets --wait --timeout 15m
```

Образы уже опубликованы как `ghcr.io/krokozabra213/<сервис>:<appVersion>` и
доступны публично, поэтому pull-секрет не требуется.

### 1.1 Проверка

ServiceLB k3s выдаёт ingress-контроллеру IP машины, поэтому 80/443 слушаются
прямо на хосте — `port-forward` не нужен:

```bash
kubectl -n ecommerce get pods,svc,ingress

curl -s http://localhost/healthz
curl -sk https://api.ecommerce.local/healthz
# Swagger UI: http://localhost/swagger/index.html
```

`/etc/hosts` должен содержать `127.0.0.1 api.ecommerce.local` — TLS-сертификат
выпущен именно на это имя.

Для итераций по коду соберите образы и импортируйте их в containerd k3s
(`make -C deploy/helm k3s-load` делает то же самое):

```bash
deploy/helm/scripts/build-images.sh
deploy/helm/scripts/build-images.sh --print-tags | xargs -I{} sh -c 'docker save {} | sudo k3s ctr images import -'
```

Разбор профилей и версий — в [`CHECKLIST.md`](../../CHECKLIST.md), подробные
сценарии и устранение проблем — в [DEPLOY-RUNBOOK.md](DEPLOY-RUNBOOK.md).

`deploy.sh` ставит релизы по порядку:

| # | Релиз | Что ставит |
|---|-------|-----------|
| 1 | `ecommerce-infra` | Postgres ×4, Redis, MongoDB, Kafka (KRaft), Schema Registry, Job создания топиков |
| 2 | `observability` | otel-collector, tempo, loki, prometheus, grafana |
| 3 | `ecommerce` | 6 сервисов + Job'ы миграций goose |

Через `make`:

```bash
cd deploy/helm
make lint          # helm lint всех чартов
make validate      # lint + render + проверка YAML (dev и prod)
make k3s-load      # собрать образы и импортировать их в containerd k3s
make install       # namespace + infra + observability + app
make status
make logs SVC=api-gateway
```

`make install` ставит всё с профилем `values-dev.yaml` и образами из узла,
поэтому ему должен предшествовать `make k3s-load`. Для образов из GHCR
используйте команды из шага 5 выше.

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
  значения-заглушки (по умолчанию `disabled`).
* `RateLimitConfig` и `OutboxConfig` настраиваются только из YAML.

### Проверка рендера (без кластера)

```bash
deploy/helm/scripts/validate-config.sh        # профили dev и prod
make -C deploy/helm validate                  # то же + helm lint + template всех чартов
```

Скрипт рендерит чарт приложения (`helm template`) для `dev` и `prod` и проверяет,
что весь вывод — корректный YAML. Это ловит синтаксические ошибки в шаблонах и
values.

Что он **не** проверяет (упрощено осознанно): схемы Kubernetes (kubeconform),
контракт конфигов через реальный `config.Init()` и чарты `infra`/`observability`.
Ошибки в них поймает `make -C deploy/helm lint`/`template` (они покрывают все три
чарта). Если понадобится вернуть полные проверки — они есть в истории git.

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

### Профили

| Файл | Назначение |
|---|---|
| `values.yaml` | базовые значения по умолчанию (накладывается всегда) |
| `values-dev.yaml` | локальная разработка: http, по 1 реплике, debug-логи |
| `values-ghcr.yaml` | оверлей: образы из GHCR (стеком к dev) |
| `values-local-tls.yaml` | локально как в проде: HTTPS (самоподписанный) + 3 реплики + PDB |
| `values-server.yaml` | один сервер (k3s): HTTPS через cert-manager + 3 реплики + инфра в кластере |
| `values-prod.yaml` | prod: managed-БД, HPA, cert-manager, NetworkPolicy |

Профили накладываются слева направо (`-f base -f overlay`), подробности и
команды — в [DEPLOY-RUNBOOK.md](DEPLOY-RUNBOOK.md).

### `ecommerce-shop` — ключевые секции

| Ключ | Назначение |
|---|---|
| `global.imageRegistry` | **только хост** реестра (`registry.example.com`). Путь/проект — в `image.repository` (`krokozabra213/api-gateway`) |
| `global.imagePullSecrets` | pull-секреты реестра (подставляются и в Deployment'ы, и в Job'ы миграций) |
| `global.imageTag` | тег сразу для всех сервисов и Job'ов миграций (пусто = `.Chart.AppVersion`); отдельный сервис — `services.<name>.image.tag` |
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

### Перенос реальных секретов из `services/*/.env`

Файлы `.env` внутри контейнеров **не читаются** (`godotenv.Load()` не находит
файл), поэтому OAuth-креды и любые нестандартные значения нужно положить в
Secret. Для этого есть скрипт:

```bash
DRY_RUN=1 deploy/helm/scripts/create-app-secret.sh   # отчёт: что и откуда
deploy/helm/scripts/create-app-secret.sh             # создать Secret

helm upgrade --install ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  --set secrets.create=false \
  --set secrets.existingSecret=ecommerce-shop-secrets
```

Приоритет значения: env-переменная → `services/auth-service/.env` → умолчание
(совпадает с `infra/values-dev.yaml`). Скрипт не печатает значения секретов.
Пути OAuth-колбэков и требования Google/GitHub — в
[DEPLOY-RUNBOOK.md §4.5](DEPLOY-RUNBOOK.md#45-oauth-redirect-url-и-требования-провайдеров).

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
NetworkPolicy (Calico/Cilium/Antrea); если в кластере такого CNI нет, включение
ничего не даст, но и не сломает. Свои источники трафика добавляйте
через `networkPolicy.extraIngressRules`.

---

## 11. Устранение неполадок

| Симптом | Причина / что делать |
|---|---|
| `config file not found: configs/dev.yaml` | ConfigMap не смонтирован или `global.environment` не совпадает с ключом. Проверьте `kubectl get cm <svc>-config -o yaml` |
| Pod в CrashLoopBackOff, `field "..." is required but the value is not provided` | не передана env-переменная из Secret'а либо опечатка в имени. Сверьтесь с §3 |
| `open .env: no such file or directory` в логах | **не ошибка**: `godotenv.Load()` не находит `.env` в контейнере и просто логирует |
| `auth-service`: `failed to load private key` | пустой/битый `secrets.values.jwtPrivateKey` либо не смонтирован Secret. Проверьте `AUTH_JWT_PRIVATE_KEY_PATH` и `kubectl get secret ... -o jsonpath='{.data.jwt-private-key}' \| base64 -d \| head -1` |
| `errored, unable to authenticate` у Kafka-консьюмеров | топики не созданы (Job `kafka-topics`), либо `dependencies.kafka.brokers` неверны |
| Kafka в `CrashLoopBackOff`: exit code 1 за секунду, в логах только `Running in KRaft mode...` и `port is deprecated. Please use KAFKA_ADVERTISED_LISTENERS instead.` | service links: Service с именем `kafka` подбрасывает в под `KAFKA_PORT=tcp://...`, и Confluent-образ принимает это за конфиг брокера. В pod-спеке должен быть `enableServiceLinks: false` |
| Schema Registry бесконечно перезапускается | Kafka ещё не готова; init-контейнер ждёт порт, проверьте `kubectl logs deploy/schema-registry -c wait-for-kafka` |
| `unsupported image` / образ не найден | `global.imageRegistry` — **только хост**; путь уже в `image.repository` |
| Миграции не запускаются | это хуки: `helm upgrade` их выполняет, `helm template` — только рендерит. Смотрите `kubectl get jobs`, `helm get hooks <release>` |
| Postgres-под не стартует после смены `CLUSTER_ID`/`PGDATA` | PVC уже отформатирован; пересоздайте PVC (`kubectl delete pvc <name>`) |

---

## 12. Область проверки и известные ограничения

Что покрыто проверками, а что осталось непроверенным — чтобы тестировщик
понимал, куда смотреть в первую очередь.

**Покрыто:**

* `helm lint` и `helm template` всех трёх чартов в профилях dev, prod и server —
  вывод валидный YAML;
* установка на k3s: инфраструктура (4 x PostgreSQL, Redis, MongoDB,
  Kafka (KRaft), Schema Registry) и приложение (6 сервисов + 4 Job'а миграций)
  поднимаются, поды проходят пробы при `readOnlyRootFilesystem: true` и
  `runAsNonRoot: 65534`;
* ingress -> `api-gateway`: `/healthz` -> 200, `/readyz` -> OK; HTTPS с TLS 1.3,
  TLS 1.2 отклоняется, http редиректится на https (DEPLOY-RUNBOOK.md §11);
* JWT: приватный ключ из Secret смонтирован, `POST /api/v1/auth/login`
  возвращает RS256-токен;
* сквозной сценарий: `register` -> outbox -> Kafka -> `user-service` создал
  пользователя -> `login` -> `GET /users/me` -> создание категории и товара
  (MongoDB) -> публикация -> товар виден в каталоге с остатками из
  `inventory-service` (gRPC);
* Swagger UI и same-origin запросы из него (DEPLOY-RUNBOOK.md §14).

**Не покрыто:**

* заказ (`order-service`) и резерв товара под нагрузкой;
* наблюдаемость: OTLP -> Tempo/Loki, скрейп otel-collector, дашборды Grafana
  (в dev-профиле observability выключена);
* NetworkPolicy в реальном CNI (в кластере по умолчанию CNI без поддержки policy);
* HA-профили (`replicaCount > 1`, `autoscaling.enabled`) и prod с внешними БД.

Команды для проверки:

```bash
# проверка рендера без кластера (dev + prod)
deploy/helm/scripts/validate-config.sh
make -C deploy/helm validate                # то же + helm lint + template всех чартов

# полноценная установка на k3s — см. §1 или DEPLOY-RUNBOOK.md
kubectl get pods -n ecommerce -w

# server-side проверка манифестов без установки
deploy/helm/deploy.sh DRY_RUN=1             # печатает команды
helm template ecommerce deploy/helm/ecommerce-shop -f deploy/helm/ecommerce-shop/values-dev.yaml \
  --set-file secrets.values.jwtPrivateKey=services/auth-service/certs/jwt-private.pem \
  | kubectl apply --dry-run=server -f -
```

---

## 13. Удаление

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
