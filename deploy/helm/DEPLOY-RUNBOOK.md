# Runbook: образы в GHCR и запуск в k3s

Пошаговая инструкция «от нуля до работающего кластера» для чартов из
`deploy/helm/`. Дополняет [README.md](README.md) (там — устройство чартов и
разбор кода сервисов); здесь — конкретные команды и подводные камни.

Короткая памятка по всему циклу (проверки перед пушем, запуск после
клонирования, переключение профилей и версий) — [`CHECKLIST.md`](../../CHECKLIST.md)
в корне репозитория.

Оглавление:

* [0. Что именно деплоится](#0-что-именно-деплоится)
* [1. Проверка инструментов](#1-проверка-инструментов)
* [2. Локальный кластер k3s](#2-локальный-кластер-k3s)
* [3. Образы: сборка и публикация в GHCR](#3-образы-сборка-и-публикация-в-ghcr)
* [4. Секреты, ключи и переменные окружения](#4-секреты-ключи-и-переменные-окружения)
* [5. Доступ к сервисам снаружи](#5-доступ-к-сервисам-снаружи)
* [6. Smoke-тест](#6-smoke-тест)
* [7. Обновление версии](#7-обновление-версии)
* [8. Удаление](#8-удаление)
* [9. Частые ошибки](#9-частые-ошибки)
* [10. CI: сборка в GHCR и выкатка](#10-ci-сборка-в-ghcr-и-выкатка)
* [11. Единая точка входа, HTTPS и только TLS 1.3](#11-единая-точка-входа-https-и-только-tls-13)
* [12. OAuth через единую точку входа](#12-oauth-через-единую-точку-входа)
* [13. Развёртывание на удалённом сервере (k3s)](#13-развёртывание-на-удалённом-сервере-k3s)
* [14. Swagger UI: где лежит и где ловушка с CORS](#14-swagger-ui-где-лежит-и-где-ловушка-с-cors)
* [15. Реальные OAuth-креды: где взять и как положить](#15-реальные-oauth-креды-где-взять-и-как-положить)

---

## 0. Что именно деплоится

Три Helm-релиза в **одном** namespace (по умолчанию `ecommerce`), ставятся
строго по порядку:

| # | Релиз | Чарт | Что внутри |
|---|-------|------|-----------|
| 1 | `ecommerce-infra` | `infra/` | PostgreSQL ×4, Redis, MongoDB, Kafka (KRaft), Schema Registry, Job создания топиков |
| 2 | `observability` | `observability/` | otel-collector, tempo, loki, prometheus, grafana (в dev по умолчанию **не** ставится) |
| 3 | `ecommerce` | `ecommerce-shop/` | 6 сервисов + 4 Job'а миграций goose |

Образов **10**: 6 сервисов + 4 образа миграций.

| Service (DNS-имя) | Порт | Образ | БД |
|---|---|---|---|
| `api-gateway` | 4000 | `krokozabra213/api-gateway` | — (+ Redis) |
| `auth-service` | 8100 | `krokozabra213/auth-service` | `auth-postgres` |
| `user-service` | 8200 | `krokozabra213/user-service` | `user-postgres` |
| `product-service` | 8300 (gRPC 44400) | `krokozabra213/product-service` | MongoDB |
| `inventory-service` | 8400 (gRPC 44100) | `krokozabra213/inventory-service` | `inventory-postgres` |
| `order-service` | 8600 | `krokozabra213/order-service` | `order-postgres` |

Два факта, из которых следует всё остальное:

1. **Service'ы называются коротко** (`auth-service`, а не
   `ecommerce-auth-service`) — конфиги ссылаются друг на друга именно такими
   именами. Поэтому **один релиз `ecommerce` на namespace**.
2. **Конфиг сервиса = ConfigMap + Secret.** Несекретные значения чарт рендерит
   из `ecommerce-shop/files/configs/<svc>.yaml` в ConfigMap и монтирует в
   `/app/configs/<env>.yaml`; секреты приходят env-переменными из Secret'а.
   Файлы `services/*/.env` в кластере **не читаются** (см. §4.1).

---

## 1. Проверка инструментов

```bash
docker version          # нужен рабочий docker
kubectl version --client
helm version
openssl version         # для проверки/генерации ключа JWT
```

> В этом репозитории `kubectl` и `helm` установлены как **snap**. Snap-приложения
> иногда падают с `cannot create transient scope: DBus error ...`, если команда
> запускается вне нормальной пользовательской сессии (например из CI или
> неинтерактивного шелла). В обычном терминале они работают. Если ловите эту
> ошибку — поставьте обычные бинарники:
> ```bash
> mkdir -p ~/.local/bin
> curl -fsSL https://get.helm.sh/helm-v3.16.4-linux-amd64.tar.gz | tar xz -C /tmp
> mv /tmp/linux-amd64/helm ~/.local/bin/
> ```

Все команды ниже выполняются **из корня репозитория** (`e-commerce_shop/`).

Проверить чарты без кластера (полезно перед первым запуском):

```bash
make -C deploy/helm lint        # helm lint всех трёх чартов
make -C deploy/helm validate    # + render + прогон конфигов через реальный config.Init()
```

---

## 2. Локальный кластер k3s

k3s ставится как systemd-служба, Traefik отключён (иначе он занимает 80/443),
а ingress-nginx ставится отдельным чартом. `kubectl port-forward` для доступа
снаружи **не нужен никогда**: встроенный ServiceLB выдаёт ingress-контроллеру
IP машины, поэтому 80/443 слушают прямо на хосте.

### 2.1 Чек-лист со свежего клона

Проверено на копии репозитория без всех gitignored-файлов (то есть как после
`git clone`): **дополнительно готовить ничего не нужно**. Чего в клоне нет и
почему это не мешает:

| Чего нет | Почему | Что происходит |
|---|---|---|
| `services/*/.env` | в `.gitignore` | скрипт берёт умолчания, OAuth-креды становятся заглушкой `disabled` — поды стартуют, вход через Google/GitHub не работает |
| `services/auth-service/certs/jwt-private.pem` | в `services/auth-service/.gitignore` | `deploy.sh` и `create-app-secret.sh` **сгенерируют** его сами (RSA-2048) |
| собранных образов | это артефакты | тянутся готовыми из GHCR (§3), для итераций собираются локально (§2.6) |

Все команды ниже выполняются **из корня репозитория** (`e-commerce_shop/`).

### 2.2 Установка k3s и kubeconfig

Traefik отключён на этапе установки: иначе он займёт 80/443, а чарт
рассчитывает на `ingressClassName: nginx`.

```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik" sh -

sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
sudo chown "$(id -u):$(id -g)" ~/.kube/config
kubectl get nodes
```

Стек из 4 × PostgreSQL + Redis + MongoDB + Kafka + Schema Registry и 6 сервисов
занимает около 6 ГБ RAM — на машине с 16 ГБ остаётся запас. Отдельного лимита,
как у виртуальной машины, здесь нет: k3s использует память хоста.

`/etc/hosts` для локального запуска: строка `127.0.0.1 api.ecommerce.local` —
корректна (k3s слушает 80/443 на самом хосте).

### 2.3 ingress-nginx

`hsts=false` нужен локально из-за самоподписанного сертификата.

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx --create-namespace \
  -f deploy/cluster/ingress-nginx-values.yaml \
  --set controller.config.hsts=false \
  --wait --timeout 10m

kubectl -n ingress-nginx get svc ingress-nginx-controller   # EXTERNAL-IP = IP машины
```

`EXTERNAL-IP` сразу становится IP машины (ServiceLB), поэтому 80/443 слушаются
на самом хосте — проброс портов и туннели не нужны.

### 2.4 Секреты

```bash
DRY_RUN=1 deploy/helm/scripts/create-app-secret.sh   # посмотреть, что и откуда возьмётся
REDIRECT_BASE=http://localhost deploy/helm/scripts/create-app-secret.sh

HOST=api.ecommerce.local deploy/helm/scripts/create-tls-secret.sh
kubectl apply -f deploy/cluster/ingress-localhost.yaml
```

`create-app-secret.sh` создаёт Secret `ecommerce-shop-secrets`: пароли БД/Redis,
собранный `MONGO_URI`, приватный ключ JWT из
`services/auth-service/certs/jwt-private.pem` и **реальные** OAuth-креды из
`services/auth-service/.env`. Подробности — §4.2.

`create-tls-secret.sh` выпускает самоподписанный сертификат для
`api.ecommerce.local`, а `ingress-localhost.yaml` публикует тот же
`api-gateway` на `http://localhost` (нужно для локального OAuth, §12.6).

Проверить, что ключ JWT действительно создан:

```bash
ls -l services/auth-service/certs/jwt-private.pem
kubectl -n ecommerce get secret ecommerce-shop-secrets -o jsonpath='{.data.jwt-private-key}' \
  | base64 -d | openssl pkey -noout && echo "ключ корректен"
```

### 2.5 Установка инфраструктуры и приложения

```bash
# 1/2 инфраструктура
helm upgrade --install ecommerce-infra deploy/helm/infra \
  --namespace ecommerce --create-namespace \
  -f deploy/helm/infra/values-dev.yaml \
  --wait --timeout 15m

# 2/2 приложение: HTTPS-профиль + готовые образы из GHCR
helm upgrade --install ecommerce deploy/helm/ecommerce-shop \
  --namespace ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  -f deploy/helm/ecommerce-shop/values-local-tls.yaml \
  -f deploy/helm/ecommerce-shop/values-ghcr.yaml \
  --set secrets.create=false \
  --set secrets.existingSecret=ecommerce-shop-secrets \
  --set 'migrations.hook=pre-install\,pre-upgrade' \
  --wait --timeout 15m
```

Образы в GHCR публичные, тянутся анонимно — pull-секрет не нужен.

Разбор двух неочевидных флагов:

* `secrets.create=false --set secrets.existingSecret=...` — чарт **не** создаёт
  Secret, а берёт готовый. Так реальные OAuth-секреты и пароли не попадают ни в
  values, ни в историю Helm-релизов. Если эту пару не задать, чарт упадёт с
  понятным сообщением (fail-fast), а не создаст «тихо» неверный Secret.
* `migrations.hook=pre-install,pre-upgrade` — миграции выполняются **до**
  создания подов приложения. Так можно, только если Secret уже существует
  (мы его создали в §2.4). По умолчанию хук `post-install` — тогда на первой
  установке поды стартуют раньше миграций (безопасно для пустого кластера, но
  менее строго). Обратите внимание на экранирование запятой: `\,` — иначе Helm
  воспримет её как разделитель аргументов `--set`.

**Альтернатива «в одну команду»** (если OAuth не нужен): `deploy/helm/deploy.sh`
ставит infra и приложение сам и даёт чарту создать Secret. Но тогда OAuth-креды
будут заглушками `disabled` (в `values-dev.yaml`), а образы должны быть уже
загружены в containerd k3s (§2.6).

### 2.6 Локальная сборка образов (итерации по коду, без GHCR)

```bash
deploy/helm/scripts/build-images.sh
deploy/helm/scripts/build-images.sh --print-tags | xargs -I{} sh -c 'docker save {} | sudo k3s ctr images import -'
```

Сборка идёт из **корня репозитория** (context = `.`): `go.work` объединяет
модули `api`, `infra`, `services/*`, и каждый Dockerfile копирует весь
workspace. Первый билд долгий — тянет зависимости Go.

### 2.7 Проверка

```bash
kubectl -n ecommerce get pods                 # все ли Running
kubectl -n ecommerce get jobs                 # *-migrate должны быть Complete
kubectl -n ecommerce get svc,ingress
kubectl -n ecommerce get events --sort-by=.lastTimestamp | tail -20

curl -s  http://localhost/healthz             # через ingress-localhost
curl -sk https://api.ecommerce.local/healthz  # самоподписанный сертификат
```

Ожидаемо: `kafka-0`, `schema-registry`, 4 × `*-postgres-0`, `redis-0`,
`mongodb-0`, 6 подов приложения и 4 завершённых Job'а миграций.

---

## 3. Образы: сборка и публикация в GHCR

Сборка и публикация десяти образов (6 сервисов + 4 Job'а миграций) в GitHub
Container Registry. Локальная установка с готовыми образами — в §2.

### 3.1 Personal Access Token

GitHub → **Settings → Developer settings → Personal access tokens → Tokens
(classic)** → Generate new token. Нужны scopes:

* `write:packages` — чтобы пушить;
* `read:packages` — чтобы тянуть (нужен, если пакеты приватные);
* `delete:packages` — опционально.

Сохраните токен: `export CR_PAT=ghp_...`

> `read:packages` нужен, только если пакеты приватные. Если запушите и тянете
> одним и тем же токеном — `write:packages` включает чтение.

### 3.2 Логин в GHCR

```bash
export CR_PAT=ghp_ВАШ_ТОКЕН
echo "$CR_PAT" | docker login ghcr.io -u krokozabra213 --password-stdin
```

Имя пользователя — ваш GitHub-логин. В GHCR **все** имена в нижнем регистре:
`krokozabra213`, а не `Krokozabra213`.

### 3.3 Сборка и push

```bash
REGISTRY=ghcr.io PUSH=1 deploy/helm/scripts/build-images.sh
```

`PROJECT` по умолчанию уже `krokozabra213`, поэтому имена получаются ровно
такие, какие ждёт чарт:

```
ghcr.io/krokozabra213/api-gateway:v1.0.0
ghcr.io/krokozabra213/auth-service:v1.0.0
...
ghcr.io/krokozabra213/order-service-migrate:v1.0.0
```

> **Почему это важно.** Итоговый образ — `<global.imageRegistry>/<image.repository>:<tag>`.
> В `ecommerce-shop/values.yaml` репозитории имеют вид `krokozabra213/<svc>`.
> Если бы там остался сгенерированный префикс `ecommerce/`, при
> `imageRegistry: ghcr.io` получилось бы `ghcr.io/ecommerce/<svc>` — namespace
> чужой организации, и `docker push` вернул бы `denied`. Держите `PROJECT` в
> `build-images.sh` и префикс в `values.yaml` синхронными.

### 3.4 Приватные пакеты и pull-секрет

После первого push пакеты создаются **приватными**. Выбран именно этот вариант,
поэтому нужен pull-секрет. Два варианта существуют, но ниже — только приватный
(публичный: GitHub → профиль → **Packages** → пакет → **Package settings →
Change visibility → Public**, тогда секрет не нужен вовсе).

**Что уже готово в проекте — править ничего не надо.** Имя секрета `ghcr-pull`
уже прописано в `global.imagePullSecrets` у трёх профилей и подставляется в
**все 10 workload'ов** (6 Deployment'ов + 4 Job'а миграций):

| Профиль | Секрет |
|---|---|
| `values-ghcr.yaml` (dev + GHCR) | `ghcr-pull` |
| `values-server.yaml` (один сервер) | `ghcr-pull` |
| `values-prod.yaml` (managed-БД) | `ghcr-pull` |
| `values-dev.yaml` (локально, без реестра) | не нужен — образы лежат в узле |

Инфраструктурному чарту секрет не нужен: `postgres`, `redis`, `mongo`,
`cp-kafka`, `cp-schema-registry`, `busybox` тянутся с Docker Hub публично
(`infra/values*.yaml` → `imagePullSecrets: []`).

**Шаг 1. Создать PAT.** GitHub → **Settings → Developer settings → Personal
access tokens → Tokens (classic)** → Generate new token → scope
**`read:packages`**. Для пакетов в организации дополнительно нужны права на неё
и авторизация токена для SSO.

**Шаг 2. Создать секрет в кластере** (скрипт идемпотентный: повторный запуск
просто обновит токен):

```bash
CR_PAT=ghp_xxx deploy/helm/scripts/create-ghcr-pull-secret.sh

# посмотреть, что будет сделано, ничего не создавая
CR_PAT=ghp_xxx DRY_RUN=1 deploy/helm/scripts/create-ghcr-pull-secret.sh

# другой namespace / имя секрета
CR_PAT=ghp_xxx NAMESPACE=shop SECRET_NAME=my-pull \
  deploy/helm/scripts/create-ghcr-pull-secret.sh
```

Скрипт сначала мягко проверяет токен в GHCR (403 — это нормально, если пакет ещё
не запушен), затем создаёт `kubernetes.io/dockerconfigjson`.

> **Порядок важен:** секрет должен существовать **до** создания подов. Если
> выкатить чарт раньше, поды уйдут в `ImagePullBackOff` — kubelet не найдёт
> указанный `imagePullSecret`.

**Шаг 3. Выкатить чарт** (секрет уже в values, дополнительных флагов не нужно):

```bash
helm upgrade --install ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  -f deploy/helm/ecommerce-shop/values-ghcr.yaml \
  --set secrets.create=false --set secrets.existingSecret=ecommerce-shop-secrets \
  --wait --timeout 15m
```

**Шаг 4. Проверить, что образы реально тянутся из GHCR.**

```bash
kubectl -n ecommerce get pods
kubectl -n ecommerce get events --sort-by=.lastTimestamp | grep -i 'pull' | tail

# какие образы у подов
kubectl -n ecommerce get pods -o jsonpath='{range .items[*]}{.spec.containers[0].image}{"\n"}{end}' | sort -u

# секрет на месте и указывает на ghcr.io
kubectl -n ecommerce get secret ghcr-pull -o jsonpath='{.type}{"\n"}'
kubectl -n ecommerce get secret ghcr-pull -o jsonpath='{.data.\.dockerconfigjson}' \
  | base64 -d | python3 -c 'import sys,json;print(list(json.load(sys.stdin)["auths"]))'
```

**Ротация токена:** создать новый PAT → снова запустить скрипт (он перезапишет
секрет) → `kubectl -n ecommerce rollout restart deploy` (секрет читается
kubelet'ом при создании пода, уже запущенные поды его не перечитывают).

**Удаление:** секрет создан вручную, `helm uninstall` его не тронет:

```bash
kubectl -n ecommerce delete secret ghcr-pull
```

> `imagePullSecrets` из `global` подставляются и в Deployment'ы, и в Job'ы
> миграций: без этого в приватном реестре Job'ы не могут забрать образ и
> install падает.

### 3.5 Установка

Оверлей `values-ghcr.yaml` добавляет только `imageRegistry: ghcr.io` и
`imagePullSecrets` (для публичных пакетов его можно не подключать — §2.5):

```bash
helm upgrade --install ecommerce deploy/helm/ecommerce-shop \
  --namespace ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  -f deploy/helm/ecommerce-shop/values-ghcr.yaml \
  --set secrets.create=false \
  --set secrets.existingSecret=ecommerce-shop-secrets \
  --set 'migrations.hook=pre-install\,pre-upgrade' \
  --wait --timeout 15m
```

Убедитесь, что образы реально тянутся из GHCR, а не подхватились локальные:

```bash
kubectl -n ecommerce get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].image}{"\n"}{end}'
```

### 3.6 Если нужно обновить образ

```bash
REGISTRY=ghcr.io TAG=v1.2.3 PUSH=1 deploy/helm/scripts/build-images.sh

helm upgrade ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  -f deploy/helm/ecommerce-shop/values-ghcr.yaml \
  --set secrets.create=false --set secrets.existingSecret=ecommerce-shop-secrets \
  --set global.imageTag=v1.2.3 \
  --wait --timeout 15m
```

`global.imageTag` переопределяет тег сразу у **всех** сервисов и Job'ов
миграций (отдельный сервис — `--set services.<name>.image.tag=...`; пустой тег
= `.Chart.AppVersion`).

---

## 4. Секреты, ключи и переменные окружения

### 4.1 Почему `services/*/.env` в кластере не работает

`config.Init()` каждого сервиса делает `godotenv.Load()` — внутри контейнера
файла `.env` нет (`**/.env` исключён корневым `.dockerignore`), и вызов просто
логирует ошибку. Дальше:

```
ENV=<env>  ->  читается configs/<env>.yaml  (монтируется из ConfigMap)
                         ↓
             cleanenv: сначала YAML, потом env-переменные (env перекрывает YAML)
```

Отсюда разделение: **несекретное** — в ConfigMap, **секретное** — в Secret и
дальше в env-переменные. Три требования, которые из этого следуют (и которые
чарт уже выполняет): ключ ConfigMap'а обязан совпадать со значением `ENV`,
файл `configs/<env>.yaml` обязан существовать, а имена env-переменных — точно
совпадать с тегами `env:"..."` в Go (без разделителей:
`PostgresConfig.DBName` → `POSTGRES_DBNAME`).

### 4.2 Что делает `create-app-secret.sh`

`deploy/helm/scripts/create-app-secret.sh` собирает Secret из локальных
источников. Приоритет значения: **env-переменная → `services/auth-service/.env`
→ умолчание**, совпадающее с `infra/values-dev.yaml`.

```bash
DRY_RUN=1 deploy/helm/scripts/create-app-secret.sh      # отчёт без создания
NAMESPACE=shop deploy/helm/scripts/create-app-secret.sh  # другой namespace
REDIRECT_BASE=http://localhost:8100 deploy/helm/scripts/create-app-secret.sh
```

Скрипт **не печатает значения секретов** — только ключ и источник
(`.env` / `default` / `env`). Применение идемпотентно
(`kubectl create --dry-run=client | kubectl apply`), можно запускать повторно
после смены `.env`.

### 4.3 Ключи Secret'а

| Ключ в Secret'е | env-переменная | Кто получает |
|---|---|---|
| `app-secret` | `APP_SECRET` | все сервисы |
| `postgres-auth-password` | `POSTGRES_PASSWORD` | auth-service |
| `postgres-user-password` | `POSTGRES_PASSWORD` | user-service |
| `postgres-inventory-password` | `POSTGRES_PASSWORD` | inventory-service |
| `postgres-order-password` | `POSTGRES_PASSWORD` | order-service |
| `redis-password` | `REDIS_PASSWORD` | api-gateway, auth-service |
| `mongodb-uri` | `MONGO_URI` | product-service |
| `jwt-private-key` | (файл) `AUTH_JWT_PRIVATE_KEY_PATH` | auth-service, `/etc/ecommerce/certs/jwt-private.pem` |
| `oauth-google-*` | `OAUTH_GOOGLE_CLIENT_ID/_SECRET/_REDIRECT_URL` | auth-service |
| `oauth-github-*` | `OAUTH_GITHUB_CLIENT_ID/_SECRET/_REDIRECT_URL` | auth-service |

Два важных свойства:

* **`MONGO_URI` нельзя задать через YAML** — поле помечено `yaml:"-"`,
  только env. Адрес базы при этом лежит отдельно в
  `configs/product-service.yaml` (`mongo.database: products`), поэтому в URI
  базы нет, и драйвер использует `authSource=admin` — как и создаёт инфра-чарт.
* **OAuth-поля обязательны всегда.** `OAuthProviderConfig.{ClientID,ClientSecret,RedirectURL}`
  помечены `env-required` и валидируются даже при `oauth.*.enabled=false`.
  Пустое значение → под падает на старте с
  `field "..." is required but the value is not provided`. Поэтому скрипт
  подставляет непустые заглушки `disabled`, если реальных кредов нет.

### 4.4 Приватный ключ JWT

* Лежит в `services/auth-service/certs/jwt-private.pem`, в git не попадает.
* В образ **не копируется**: `auth-service/Dockerfile` больше не копирует
  `certs/`, а корневой `.dockerignore` исключает `**/*.pem`. Иначе ключ утёк бы
  вместе с образом, и любой, кто скачал образ, мог бы подделывать JWT.
* В кластер приходит Secret'ом и монтируется **только** в auth-service как
  `/etc/ecommerce/certs/jwt-private.pem` (`defaultMode: 0440`).
* api-gateway ключ не получает: он забирает публичный ключ у auth-service по
  HTTP.

Проверить, что ключ в Secret'е корректный:

```bash
kubectl -n ecommerce get secret ecommerce-shop-secrets \
  -o jsonpath='{.data.jwt-private-key}' | base64 -d | openssl pkey -noout && echo "ключ OK"
```

Если ключа нет — сгенерировать (только для dev):

```bash
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
  -out services/auth-service/certs/jwt-private.pem
chmod 600 services/auth-service/certs/jwt-private.pem
```

### 4.5 OAuth: redirect URL и требования провайдеров

Роуты есть и в auth-service, и в api-gateway:

```
GET /api/v1/auth/oauth/<provider>/login
GET /api/v1/auth/oauth/<provider>/callback
```

`redirectURL` в конфиге должен **посимвольно** совпадать с тем, что
зарегистрировано в OAuth-приложении Google/GitHub. Ваш
`services/auth-service/.env` содержит:

```
http://localhost:8100/api/v1/auth/oauth/google/callback
http://localhost:8100/api/v1/auth/oauth/github/callback
```

**Самый простой рабочий вариант** — оставить эти значения как есть и
пробросить порт auth-service на localhost:

```bash
kubectl -n ecommerce port-forward svc/auth-service 8100:8100
```

Тогда OAuth-флоу идёт напрямую в auth-service, и **ничего не нужно
перерегистрировать** у провайдеров:

```bash
open "http://localhost:8100/api/v1/auth/oauth/google/login"    # macOS
xdg-open "http://localhost:8100/api/v1/auth/oauth/google/login" # Linux
```

**Вариант через ingress** (`http://api.ecommerce.local/...`) требует:

1. добавить новый callback в настройки OAuth-приложения;
2. **TLS для Google**: Google принимает `http://` только для `localhost`, для
   любого другого хоста нужен `https://`. То есть `http://api.ecommerce.local/...`
   Google отклонит — понадобится самоподписанный сертификат в
   `ingress.tls` и доверие к нему в браузере. GitHub `http` допускает, но
   надёжнее тоже TLS.

Поэтому для локального запуска рекомендуется port-forward.

Проверить, что креды реально доехали до пода (значения не печатаются):

```bash
kubectl -n ecommerce exec deploy/auth-service -- sh -c \
  'test -n "$OAUTH_GOOGLE_CLIENT_ID" && echo "client_id задан" ; \
   test -f "$AUTH_JWT_PRIVATE_KEY_PATH" && echo "ключ JWT смонтирован"'
```

### 4.6 Произвольные переменные окружения и файлы

Если сервису нужна переменная, которой нет в Secret'е, в values у каждого
сервиса есть `extraEnv` / `extraEnvFrom` (и `extraVolumes` /
`extraVolumeMounts` для файлов):

```bash
helm upgrade ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  --set secrets.create=false --set secrets.existingSecret=ecommerce-shop-secrets \
  --set services.order-service.extraEnv[0].name=FEATURE_X \
  --set services.order-service.extraEnv[0].value=1
```

Так же передаётся всё, что не имеет env-тегов в Go и живёт только в YAML
(`RateLimitConfig`, `OutboxConfig`) — но его правильнее выносить в values и
рендерить через `files/configs/*.yaml`.

### 4.7 Production-путь (для сведения)

В prod чарт **специально падает**, если `global.environment=prod` и
`secrets.create=true` — чтобы пароли не уехали в values. Правильно так:

```bash
kubectl -n ecommerce create secret generic ecommerce-shop-secrets \
  --from-literal=app-secret="$(openssl rand -hex 32)" \
  --from-literal=postgres-auth-password="$(openssl rand -hex 24)" \
  ... \
  --from-file=jwt-private-key=services/auth-service/certs/jwt-private.pem

helm upgrade --install ecommerce deploy/helm/ecommerce-shop \
  -n ecommerce -f deploy/helm/ecommerce-shop/values-prod.yaml   # там уже create=false
```

Не забудьте синхронизировать пароли с инфраструктурой: её Secret'ы
(`postgresql-credentials`, `redis-credentials`, `mongodb-credentials`) должны
содержать те же значения, иначе приложение не подключится.

---

## 5. Доступ к сервисам снаружи

Наружу опубликован только `api-gateway` (host `api.ecommerce.local`). k3s
слушает 80/443 на самом хосте, поэтому достаточно добавить запись в `/etc/hosts`
и обращаться по имени — проброс портов не нужен:

```bash
echo "127.0.0.1 api.ecommerce.local" | sudo tee -a /etc/hosts

curl -i https://api.ecommerce.local/healthz     # 200 от api-gateway (сертификат самоподписанный)
curl -i https://api.ecommerce.local/readyz      # 200, когда зависимости доступны
curl -i http://localhost/healthz                # http-вход через ingress-localhost
```

Отладочный доступ к внутренним сервисам — без ingress:

```bash
kubectl -n ecommerce port-forward svc/api-gateway 4000:4000
kubectl -n ecommerce port-forward svc/auth-service 8100:8100
kubectl -n ecommerce port-forward svc/grafana 3000:3000     # если observability включён
```

---

## 6. Smoke-тест

`api-gateway` проксирует `/api/v1/auth`, `/api/v1/users`, `/api/v1/products`,
`/api/v1/categories`, `/api/v1/orders`, `/api/v1/inventory`.

```bash
BASE=https://api.ecommerce.local   # самоподписанный сертификат, поэтому curl -k

# 1) регистрация (email + password >= 8 символов)
curl -sk -X POST "$BASE/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"Passw0rd123"}'

# 2) логин -> access_token
TOKEN=$(curl -sk -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"Passw0rd123"}' | sed -E 's/.*"access_token":"([^"]+)".*/\1/')

# 3) защищённый эндпоинт
curl -sk "$BASE/api/v1/users/me" -H "Authorization: Bearer $TOKEN"

# 4) публичный каталог
curl -sk "$BASE/api/v1/products"
```

**Про роли.** При регистрации пользователь получает `ROLE_USER`. Операции
`rolesMW` (создание товаров/категорий, управление заказами) требуют более
высокой роли, а выдать её через API нельзя — эндпоинт выдачи ролей сам
защищён `rolesMW` (замкнутый круг на пустой БД). Роль выдаётся прямо в БД
user-service:

```bash
kubectl -n ecommerce exec -it user-postgres-0 -- \
  psql -U postgres -d postgres -c \
  "INSERT INTO user_roles (user_id, role)
   SELECT id, 'ROLE_ADMIN' FROM users WHERE email='test@example.com'
   ON CONFLICT DO NOTHING;"
```

---

## 7. Обновление версии

```bash
# пересобрать и (если есть реестр) запушить
REGISTRY=ghcr.io TAG=v1.2.3 PUSH=1 deploy/helm/scripts/build-images.sh

# обновить релиз
helm upgrade ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  -f deploy/helm/ecommerce-shop/values-ghcr.yaml \
  --set secrets.create=false --set secrets.existingSecret=ecommerce-shop-secrets \
  --set global.imageTag=v1.2.3 \
  --set 'migrations.hook=pre-install\,pre-upgrade' \
  --wait --timeout 15m
```

Миграции при `helm upgrade` выполняются хуком **до** обновления подов — схема
приезжает раньше кода.

**Не переиспользуйте тег.** Соберите образы с новым тегом и передайте его
релизу — тогда rollout произойдёт сам, без остановки подов:

```bash
TAG=v1.0.1 deploy/helm/scripts/build-images.sh
helm upgrade ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  -f deploy/helm/ecommerce-shop/values-local-tls.yaml \
  -f deploy/helm/ecommerce-shop/values-ghcr.yaml \
  --set secrets.create=false --set secrets.existingSecret=ecommerce-shop-secrets \
  --set global.imageTag=v1.0.1 --wait
```

Для локальной итерации без реестра импортируйте собранные образы в containerd
k3s (§2.6). В GHCR вопроса подмены нет вообще: теги неизменяемы, и kubelet
тянет новый образ по новому тегу.

Убедиться, что под взял именно новый образ (пример для api-gateway — смотрим
спеку, вшитую в образ):

```bash
kubectl -n ecommerce exec deploy/api-gateway -c api-gateway -- \
  sh -c 'sed -n "/^servers:/,+2p" /app/api/openapi.yaml'
```

---

## 8. Удаление

```bash
helm uninstall ecommerce -n ecommerce
helm uninstall observability -n ecommerce        # если ставили
helm uninstall ecommerce-infra -n ecommerce

# PVC релизом НЕ удаляются — удалите вручную:
kubectl delete pvc -n ecommerce -l app.kubernetes.io/part-of=ecommerce-infra
```

Secret `ecommerce-shop-secrets` создан вручную (`existingSecret`) — при
`helm uninstall` он останется, удалите отдельно:

```bash
kubectl -n ecommerce delete secret ecommerce-shop-secrets ghcr-pull
```

Полная очистка кластера: `sudo /usr/local/bin/k3s-uninstall.sh`.

---

## 9. Частые ошибки

| Симптом | Причина / что делать |
|---|---|
| `helm push` → `denied: requested access to the resource is denied` | токен без `write:packages` либо `PROJECT`/`image.repository` не совпадают с вашим GHCR-namespace |
| поды в `ImagePullBackOff`, `ghcr.io/krokozabra213/...` | пакеты приватные, а pull-секрета нет или он неверный. Проверьте `kubectl -n ecommerce get secret ghcr-pull`, пересоздайте с `--docker-password="$CR_PAT"` |
| Swagger UI: `Failed to fetch` / намёк на CORS на каждый запрос | в спеке `servers` указан абсолютный адрес (`http://localhost:4000`), а UI открыт по другому адресу. Лечится относительным `url: /` (§14) |
| `config file not found: configs/dev.yaml` | ConfigMap не смонтирован или `global.environment` не совпадает с ключом ConfigMap'а |
| `field "..." is required but the value is not provided` | не доехала env-переменная из Secret'а: сверьте имена по §4.3 (например `POSTGRES_DBNAME`, а не `POSTGRES_DB_NAME`) |
| `secrets.create=false, но secrets.existingSecret не задан` | fail-fast чарта: добавьте `--set secrets.existingSecret=...` или создайте Secret скриптом |
| `secrets.values.jwtPrivateKey пуст, а auth-service включён` | используйте скрипт §4.2 либо `--set-file secrets.values.jwtPrivateKey=...` |
| `open .env: no such file or directory` в логах | **не ошибка**: `godotenv.Load()` не находит `.env` внутри контейнера |
| `failed to load private key` (auth-service) | пустой/битый `jwt-private-key` в Secret'е либо не смонтирован том |
| Job'ы миграций `Error`, `unable to authenticate` | топики/БД ещё не готовы либо пароль в Secret'е не совпадает с инфраструктурным |
| `errored, unable to authenticate` у Kafka-консьюмеров | не выполнился Job `kafka-topics` либо неверные `dependencies.kafka.brokers` |
| Kafka (или Schema Registry) в `CrashLoopBackOff`: exit code 1 за секунду, в логах только `Running in KRaft mode...` и `port is deprecated` | service links: Service с именем `kafka` подбрасывает в под `KAFKA_PORT=tcp://...`, а Confluent-образ считает любую `KAFKA_*` конфигом брокера. В pod-спеке должен быть `enableServiceLinks: false` |
| schema-registry перезапускается | Kafka ещё не готова; смотрите `kubectl logs deploy/schema-registry -c wait-for-kafka` |
| Postgres не стартует после смены `CLUSTER_ID`/`PGDATA` | PVC уже отформатирован: `kubectl delete pvc <name>` (данные пропадут) |
| `cannot create transient scope: DBus error` | snap-версия `helm`/`kubectl` вне пользовательской сессии — поставьте обычные бинарники (§1) |
| OAuth: `redirect_uri_mismatch` | `redirectURL` не совпадает с зарегистрированным у провайдера посимвольно (схема, хост, порт, путь) |

---

## 10. CI: сборка в GHCR и выкатка

Готовый workflow — `.github/workflows/deploy.yaml`. Он дополняет `ci.yaml`
(там линт/тесты) и делает две вещи:

| job | когда | что |
|---|---|---|
| `images` | push тега `v*` или ручной запуск | собирает 10 образов и пушит в GHCR |
| `deploy` | **только** ручной запуск с галочкой «deploy» | `helm upgrade` в кластер |

Тег вычисляется автоматически:

* push тега `v1.2.3` → образы `ghcr.io/krokozabra213/<svc>:v1.2.3`;
* ручной запуск без тега → `sha-<короткий-хеш>`;
* ручной запуск с тегом → ровно этот тег.

Имена сервисов в манифестах править не нужно: они собираются как
`<global.imageRegistry>/<image.repository>:<global.imageTag>`, где
`image.repository` = `krokozabra213/<svc>` уже лежит в `values.yaml`, а реестр
и тег job `deploy` подставляет через `--set`. Подробнее — §10.1.

Что настроить один раз в репозитории:

1. **Settings → Actions → General → Workflow permissions** — «Read and write
   permissions» (в workflow права выданы явно, но при read-only на уровне
   репозитория push пакетов может блокироваться).
2. **Repository secret `KUBE_CONFIG`** для job `deploy`:
   ```bash
   base64 -w0 ~/.kube/config      # целиком, одной строкой
   ```
3. **Pull-секрет в кластере** (если пакеты приватные) — примеры в §3.4.
   Первый push всегда создаёт пакеты приватными.

Что workflow **не** делает: не создаёт Secret'ы приложения — там OAuth-креды и
приватный ключ JWT, они кладутся в кластер отдельно (§4.2). Как перенести это в
CI — в комментарии в конце `deploy.yaml` (и почему это стоит делать осознанно).

> Время: 10 образов × сборка Go-workspace — заметно. Если станет долго,
> переходите на matrix-стратегию (по образу на job) и кэш
> `docker/build-push-action`; сейчас сборка идёт последовательно одним job'ом.

### 10.1 Как имена образов попадают в манифесты

`image.repository` в `values.yaml` уже содержит владельца GHCR
(`krokozabra213/auth-service` и т.д.), а реестр и тег задаются снаружи. Поэтому
при выкатке достаточно двух флагов:

```bash
helm upgrade --install ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-server.yaml \
  --set global.imageRegistry=ghcr.io \
  --set global.imageTag=v1.2.3 \
  --wait --timeout 20m
```

`global.imageTag` применяется и к сервисам, и к Job'ам миграций (все они идут
через один хелпер `ecommerce-shop.image`). Приоритет тега:
`services.<name>.image.tag` → `global.imageTag` → `.Chart.AppVersion`.

Проверить, во что отрендерится, не трогая кластер:

```bash
helm template ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-server.yaml \
  --set global.imageTag=v1.2.3 | grep -E '^\s+image:' | sort -u
```

Альтернатива (GitOps): job может записывать тег прямо в values-файл и
коммитить его — тогда кластер синхронизируется Argo CD/Flux. Для одного
сервера `--set` из workflow проще и не создаёт лишних коммитов.

---

## 11. Единая точка входа, HTTPS и только TLS 1.3

### 11.1 Что такое «единая точка входа» здесь

Наружу смотрит **ровно один** Ingress — на `api-gateway`. Остальные пять
сервисов имеют только `ClusterIP` и доступны исключительно внутри namespace.
Проверить:

```bash
kubectl -n ecommerce get ingress                  # ровно один: api-gateway
kubectl -n ecommerce get svc                      # все типы ClusterIP
```

**Отдельный nginx ставить не нужно.** `ingress-nginx` controller — это и есть
nginx (в нашем случае v1.14.3 на OpenSSL 3), он терминирует TLS и проксирует в
`api-gateway`. Второй nginx перед ним был бы лишним слоем без пользы.

### 11.2 Локально: HTTPS с самоподписанным сертификатом

```bash
# 1) сертификат (ключ ложится в deploy/.tools/tls — gitignored)
HOST=api.ecommerce.local deploy/helm/scripts/create-tls-secret.sh

# 2) профиль: HTTPS + 3 реплики каждого сервиса + PDB + распределение по узлам
helm upgrade --install ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  -f deploy/helm/ecommerce-shop/values-local-tls.yaml \
  --set secrets.create=false \
  --set secrets.existingSecret=ecommerce-shop-secrets \
  --set 'migrations.hook=pre-install\,pre-upgrade' \
  --wait --timeout 15m

# 3) имя в /etc/hosts — для k3s правильный адрес именно 127.0.0.1
echo "127.0.0.1 api.ecommerce.local" | sudo tee -a /etc/hosts
```

> Для OAuth этот адрес всё равно не подойдёт — Google не принимает непубличные
> TLD. Локальный OAuth делается через `localhost` (§12.6).

Профиль `values-local-tls.yaml`:

* включает `ingress.tls` с Secret'ом `ecommerce-tls`;
* `force-ssl-redirect` — http отдаёт `308` на https;
* по 3 реплики каждому сервису + `PodDisruptionBudget(maxUnavailable: 1)`;
* `defaults.spreadAcrossNodes: true` — реплики не собираются на одном узле
  (`ScheduleAnyway`, поэтому одноузловой k3s не остаётся без подов).

### 11.3 «Только TLS 1.3» — это настройка контроллера, а не Ingress'а

У `ingress-nginx` **нет** аннотации для управления версиями протокола: это
ключ `ssl-protocols` в ConfigMap самого контроллера. То есть настройка
кластерная, а не пер-ингрессная.

```bash
kubectl -n ingress-nginx patch configmap ingress-nginx-controller \
  --type merge -p '{"data":{"ssl-protocols":"TLSv1.3"}}'
```

Контроллер сам перечитает ConfigMap и перезагрузит nginx. Для чистого
развёртывания на сервере то же самое задано в
`deploy/cluster/ingress-nginx-values.yaml`.

> Про шифры: ключ `ssl-ciphers` в nginx действует **только** на TLS ≤ 1.2.
> Наборы TLS 1.3 задаются OpenSSL и по умолчанию безопасны
> (`TLS_AES_256_GCM_SHA384`, `TLS_CHACHA20_POLY1305_SHA256`,
> `TLS_AES_128_GCM_SHA256`) — трогать их не нужно.

### 11.4 Как проверить, что TLS 1.3 работает и TLS 1.2 отключён

```bash
# HTTPS вообще
curl -k -s -o /dev/null -w '%{http_code}\n' https://api.ecommerce.local/healthz

# http -> https
curl -s -o /dev/null -D - http://api.ecommerce.local/healthz | grep -i location

# только 1.3
echo | openssl s_client -connect 127.0.0.1:443 -servername api.ecommerce.local -tls1_3 2>/dev/null \
  | grep -E 'Protocol|Cipher'

# TLS 1.2 должен НЕ договориться — nginx отвечает "tlsv1 alert protocol version"
# и оставляет "Cipher is (NONE)". Проверяем именно это, а не текст handshake
# failure: формулировка отличается между версиями OpenSSL.
echo | openssl s_client -connect 127.0.0.1:443 -servername api.ecommerce.local -tls1_2 2>&1 \
  | grep -qE 'alert protocol version|Cipher is \(NONE\)' \
  && echo "TLS 1.2 отклонён" || echo "TLS 1.2 ПРИНЯТ — проверьте ConfigMap"

# то же через curl: с --tls-max 1.2 запрос должен упасть
curl -k -s -o /dev/null -w '%{http_code}\n' --tlsv1.2 --tls-max 1.2 \
  https://api.ecommerce.local/healthz   # ожидаем 000
```

Проверено локально в k3s: `TLS 1.3 → TLS_AES_256_GCM_SHA384`, TLS 1.2
отклоняется, http отдаёт `308` на `https://api.ecommerce.local/...`.

### 11.5 Три реплики: на что смотреть

```bash
kubectl -n ecommerce get deploy              # у всех 6 сервисов 3/3
kubectl -n ecommerce get pdb
kubectl -n ecommerce get pods -o wide        # распределение по узлам
```

Нюанс HPA: если у сервиса `autoscaling.enabled: true`, поле `replicas` в
Deployment'е **не выставляется** — количеством управляет HPA. В
`values-local-tls.yaml` и `values-server.yaml` HPA выключен и заданы ровно
`replicaCount: 3`; в `values-prod.yaml` у `api-gateway` и `product-service` HPA
включён с `minReplicas: 3`, то есть 3 реплики — это гарантированный минимум.
Если по ТЗ нужно жёстко 3 — выключите `autoscaling.enabled`.

---

## 12. OAuth через единую точку входа

### 12.1 Как это работает в коде (важно понять до настройки)

`GET /api/v1/auth/oauth/<provider>/login` возвращает **JSON, а не 302-редирект**:

```json
{
  "auth_url": "https://accounts.google.com/o/oauth2/auth?client_id=...&redirect_uri=...&state=...",
  "state": "..."
}
```

То есть браузер туда надо отправить **на клиенте** (фронтенд берёт `auth_url` и
делает `window.location = auth_url`). Если открыть этот URL руками, вы увидите
JSON, и это не ошибка.

Дальше Google возвращает браузер на `redirect_uri`, который собран из
`OAUTH_<PROVIDER>_REDIRECT_URL` (значение из Secret'а) — и он указывает ровно
на единую точку входа:

```
https://api.ecommerce.local/api/v1/auth/oauth/google/callback
```

### 12.2 Что обязательно сделать в консолях провайдеров

Этот точный URL должен быть **посимвольно** зарегистрирован в OAuth-приложении:

* Google Cloud Console → APIs & Services → Credentials → OAuth client ID →
  Authorized redirect URIs;
* GitHub → Settings → Developer settings → OAuth Apps → Authorization callback URL.

Ваши текущие `.env` содержат `http://localhost:8100/...` — этот адрес
продолжает работать только если ходить в auth-service напрямую через
port-forward (то есть мимо единой точки входа). Для схемы через ingress нужно
**добавить** https-адрес.

> Google принимает `http://` только для `localhost`. Для любого другого хоста
> обязателен `https://` — поэтому переход на ingress-адрес обязан быть https,
> и это как раз то, что даёт TLS 1.3 из §11.

### 12.3 Где взять адрес и как его поменять

Скрипт умеет переписать оба redirect URL разом:

```bash
KUBECTL=kubectl REDIRECT_BASE=https://api.ecommerce.local \
  deploy/helm/scripts/create-app-secret.sh
```

После смены Secret'а поды надо пересоздать — значения подставляются через
`secretKeyRef`, поэтому сам по себе Secret не триггерит rollout:

```bash
kubectl -n ecommerce rollout restart deploy/auth-service
```

### 12.4 Почему 3 реплики не ломают OAuth

`state` (защита от CSRF) хранится в **Redis**, а не в памяти пода: одна
реплика создаёт `oauth:state:<state>` с TTL, а callback может прийти на любую
другую — валидация пройдёт. После запроса `/login` ключ виден в
Redis, а в endpoints `auth-service` три пода.

Будь это in-memory map, при `replicaCount: 3` OAuth падал бы через раз с
«Невалидный state токен» — но здесь всё корректно.

### 12.5 Что останется проверить вручную

Полный флоу требует реального логина в аккаунте Google/GitHub в браузере.
Автоматически проверяется только построение `auth_url` и наличие `state` в
Redis. Отдельно учтите: с самоподписанным сертификатом браузер покажет
предупреждение при возврате на `https://api.ecommerce.local/...` — надо один
раз нажать «Дополнительно → Перейти». С настоящим сертификатом этого не будет.

### 12.6 Локально без домена: только `localhost`

**`api.ecommerce.local` для Google не существует.** `.local` зарезервирован
[RFC 6762](https://datatracker.ietf.org/doc/html/rfc6762) под mDNS и не является
публичным TLD — поэтому консоль отвечает
`Invalid Origin: must end with a public top-level domain`.

Плюс легко перепутать поля: сообщение про **Origin** относится к полю
*Authorized JavaScript origins* (оно для чисто браузерных приложений и требует
публичный домен). Нашему серверному флоу нужен **Authorized redirect URIs** —
и туда Google [разрешает](https://developers.google.com/identity/protocols/oauth2/web-server)
адреса локальной машины, в документации у них везде пример
`http://localhost:8080`.

Рабочая локальная схема — **через тот же nginx**, а не мимо него:

```bash
# 1) отдельный Ingress для хоста localhost (основной остаётся https-only)
kubectl apply -f deploy/cluster/ingress-localhost.yaml

# 2) туннель в ingress-контроллер: http://localhost:8080 -> nginx -> api-gateway
kubectl -n ingress-nginx port-forward svc/ingress-nginx-controller 8080:80

# 3) переключить redirect и перезапустить auth-service
deploy/helm/scripts/set-oauth-redirect.sh http://localhost:8080
```

Путь трафика получается ровно как в проде:
`браузер → nginx (ingress-controller) → Service api-gateway → один из 3 подов`.

**Чего делать не надо.** `kubectl port-forward svc/api-gateway 8080:4000`
тоже даст `http://localhost:8080`, но это **отладочный туннель прямо в под**:
он не проходит через ingress и не отражает реальную схему. Для быстрой
проверки кода он удобен, но тестировать на нём «единую точку входа» смысла нет.

Почему отдельный Ingress: host в Ingress обязан быть DNS-именем — Kubernetes
отклоняет `127.0.0.1` («must be a DNS name, not an IP address»). Если клиент
идёт по IP, ему нужен явный заголовок:
`curl -H 'Host: localhost' http://127.0.0.1:8080/healthz`.

Почему http, а не https: Google принимает `http` только для `localhost`, а
самоподписанный сертификат в браузере добавляет лишний шаг с предупреждением.
Для основного домена (`api.ecommerce.local`, §11) схема остаётся https-only
с TLS 1.3 — настройки в ingress-nginx действуют пер-ингресс.

Скрипт напечатает точные адреса для консолей провайдеров. В браузере дальше
работает штатный флоу: фронтенд берёт `auth_url` из ответа `/login` и
открывает его.

### 12.7 Переключение localhost ↔ домен

Когда появится домен — одна команда, регистр и путь менять не нужно:

```bash
deploy/helm/scripts/set-oauth-redirect.sh https://api.вашдомен.ru
```

Скрипт делает две вещи: пересобирает Secret с новыми `OAUTH_*_REDIRECT_URL` и
перезапускает `auth-service` (значения приходят через `secretKeyRef`, поэтому
Secret сам по себе rollout не запускает). Дальше остаётся добавить новый
callback в консоли Google/GitHub — старый localhost-адрес можно оставить, они
не конфликтуют.

---

## 13. Развёртывание на удалённом сервере (k3s)

### 13.1 k3s без Traefik

Установка k3s одинакова на локальной машине и на сервере — команды в §2.2.
Traefik отключён обязательно: он занял бы 80/443, а чарт рассчитывает на
`ingressClassName: nginx`.

Если k3s уже стоит с Traefik:

```bash
kubectl -n kube-system scale deploy traefik --replicas=0
kubectl -n kube-system delete svc traefik
```

### 13.2 ingress-nginx

Ставится так же, как локально (§2.3). После установки `EXTERNAL-IP` у
`ingress-nginx-controller` должен стать IP сервера (k3s даёт его через встроенный
ServiceLB). Если там `<pending>` — Traefik всё ещё держит порты.

### 13.3 DNS и firewall

* A-запись домена → внешний IP сервера;
* открыть `80/tcp` (нужен Let's Encrypt для HTTP-01 и для редиректа) и
  `443/tcp`;
* `ufw allow 80,443/tcp` (или правила облачной security group).

Проверка до выпуска сертификата: `dig +short api.вашдомен.ru` должен вернуть IP
сервера, а `curl -I http://api.вашдомен.ru` — что-то ответить (404 от nginx — норма).

### 13.4 cert-manager и сертификат Let's Encrypt

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
kubectl -n cert-manager wait --for=condition=Available deploy --all --timeout=5m

# ЗАМЕНИТЕ email в файле, при необходимости сначала на letsencrypt-staging
kubectl apply -f deploy/cluster/cluster-issuer-letsencrypt.yaml

kubectl get clusterissuer
```

### 13.5 Образы: GHCR

Образы в GHCR публичные и тянутся анонимно — pull-секрет на сервере не нужен.
Сборка и push — §3.3. Если пакеты всё-таки приватные, секрет создаётся
скриптом: `CR_PAT=... deploy/helm/scripts/create-ghcr-pull-secret.sh`
(подробно §3.4).

### 13.6 Секреты, инфраструктура и выбор «managed vs в кластере»

Есть два прод-варианта. Они отличаются **только тем, где живут БД**, а не
«режимом» приложения — `global.environment` в обоих `prod`:

| Вариант | Где БД | Чарт `infra` | Профиль приложения |
|---|---|---|---|
| **Всё в кластере** (БД подами в namespace `ecommerce`) | Postgres ×4, Redis, MongoDB, Kafka, Schema Registry | **ставится** | `values-server.yaml` + `values-prod-onprem.yaml` |
| Managed (RDS/Atlas/MSK) | внешние сервисы | **не ставится** | `values-prod.yaml` (там реальные адреса `dependencies.*`) |

Важно не путаться: в `values-prod.yaml` «внешними» выглядят только адреса
`dependencies.*` (`*.internal.example.com`). Сам чарт `infra` **всегда поднимает
БД внутри кластера**; `values-prod.yaml` нужен лишь тогда, когда вместо него
подключены managed-сервисы.

Секреты приложения создаются заранее (чарт **специально падает** при
`environment=prod` и `secrets.create=true` — §4.7; ключи — §4.3):

```bash
# REDIRECT_BASE — уже на боевой домен, чтобы OAuth не пришлось перенастраивать
REDIRECT_BASE=https://api.вашдомен.ru deploy/helm/scripts/create-app-secret.sh
```

Секреты инфраструктуры — те же имена, что ждёт `infra/values-prod.yaml` в
`existingSecret`, и пароли должны **совпадать** с `ecommerce-shop-secrets`
(§4.7): `postgresql-credentials` (ключи `auth-password`, `inventory-password`,
`order-password`, `user-password`), `redis-credentials` (`password`),
`mongodb-credentials` (`root-password`).

Установка инфраструктуры (БД внутри кластера):

```bash
helm upgrade --install ecommerce-infra deploy/helm/infra \
  --namespace ecommerce --create-namespace \
  -f deploy/helm/infra/values-prod.yaml \
  --wait --timeout 20m
```

Размеры PVC в `values-prod.yaml` рассчитаны на настоящий сервер (см. §13.11) —
на маленьком диске добавьте оверлей с размерами.

### 13.7 Observability (otel-collector, Tempo, Loki, Prometheus, Grafana)

Чарт ставится в тот же namespace `ecommerce`, чтобы приложение видело
`otel-collector:4317` по короткому имени.

```bash
# Grafana-пароль из заранее созданного Secret (values-prod.yaml так ждёт)
kubectl -n ecommerce create secret generic grafana-admin \
  --from-literal=admin-password="$(openssl rand -hex 16)"

# домен Grafana: заменить в values-prod.yaml (grafana.ingress.host + tls)
helm upgrade --install observability deploy/helm/observability \
  --namespace ecommerce \
  -f deploy/helm/observability/values-prod.yaml \
  --wait --timeout 20m
```

Сервисы **не отдают `/metrics`** — они пушат OTLP в otel-collector, а Prometheus
скрейпит `otel-collector:8889` (`deploy/helm/README.md` §9). Поэтому
`observability.enabled=true` в профиле приложения обязателен, иначе телеметрия
не пойдёт.

### 13.8 Приложение

```bash
# домен: заменить в values-server.yaml в ДВУХ местах
#   ingress.tls[0].hosts[]  и  services.api-gateway.ingress.host
helm upgrade --install ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-server.yaml \
  -f deploy/helm/ecommerce-shop/values-prod-onprem.yaml \
  --wait --timeout 20m

kubectl -n ecommerce get pods,ingress
kubectl -n ecommerce get certificate           # cert-manager выпустил TLS
```

Профили:

* `values-server.yaml` — `environment: prod`, зависимости по коротким
  внутрикластерным DNS-именам, 3 реплики каждого сервиса + PDB, HTTPS через
  cert-manager, образы из `ghcr.io/krokozabra213`;
* `values-prod-onprem.yaml` (оверлей поверх `values-server.yaml`) — HPA для
  `api-gateway`/`product-service`, `observability.enabled=true`,
  `networkPolicy.enabled=true`, миграции `pre-install,pre-upgrade`. HPA требует
  metrics-server (в k3s встроен); NetworkPolicy — CNI с поддержкой policy
  (§13.10).

Если БД managed — вместо этих двух файлов берите `values-prod.yaml` и заполните
в нём реальные адреса `dependencies.*`.

### 13.9 Проверка после развёртывания

```bash
kubectl -n ecommerce get pods,jobs,hpa,ingress

curl -sI http://api.вашдомен.ru/healthz            # 308 на https
curl -s  https://api.вашдомен.ru/healthz           # 200
curl -s  https://api.вашдомен.ru/readyz

echo | openssl s_client -connect api.вашдомен.ru:443 -servername api.вашдомен.ru -tls1_3 2>/dev/null | grep Protocol
echo | openssl s_client -connect api.вашдомен.ru:443 -servername api.вашдомен.ru -tls1_2 2>&1 \
  | grep -qE 'alert protocol version|Cipher is \(NONE\)' && echo "TLS 1.2 отклонён"

kubectl -n ecommerce get deploy                  # 3/3 у всех шести
kubectl -n ecommerce get certificate             # cert-manager: Ready=True

# HPA (после установки metrics-server какое-то время может быть <unknown>)
kubectl -n ecommerce get hpa
kubectl top pods -n ecommerce | head

# миграции отработали
kubectl -n ecommerce get jobs | grep migrate     # Completed

# observability
kubectl -n ecommerce get pods | grep -E "otel|tempo|loki|prometheus|grafana"
```

Сквозной smoke-сценарий (register → login → protected endpoint) — §6;
OAuth и TLS-1.3 — §11–§12.

### 13.10 Чего на k3s ожидать не стоит

* **NetworkPolicy не работает** на стандартном k3s (CNI flannel их не
  поддерживает). `networkPolicy.enabled: true` не даст эффекта, пока не
  поставите k3s с `--flannel-backend=none` и Calico/Cilium. Поэтому в
  `values-server.yaml` он выключен.
* **PostgreSQL/Redis/Mongo/Kafka в infra-чарте — single-node.** Требование
  «3 реплики» относится к микросервисам; 3 реплики StatefulSet'а PostgreSQL без
  настроенной репликации не дают отказоустойчивости, а Kafka для 3 брокеров
  требует отдельной настройки кворума контроллеров. Для настоящего HA нужны
  операторы (CloudNativePG, Strimzi) или managed-сервисы.

### 13.11 Размеры PVC и маленькие диски

Прод-профили рассчитаны на настоящий сервер и суммарно требуют очень много
места:

| Чарт | PVC | Итого |
|---|---|---|
| `infra/values-prod.yaml` | PostgreSQL 20Gi ×4, MongoDB 30Gi, Kafka 50Gi, Redis 5Gi | **165 GiB** |
| `observability/values-prod.yaml` | Prometheus 100Gi, Tempo 50Gi, Loki 50Gi, Grafana 10Gi | **210 GiB** |

Итого около **375 GiB**. На одноузловой машине с меньшим диском уменьшите
размеры оверлеем — пример ниже укладывается в ~21 GiB (при этом сам prod-режим
сохраняется полностью, уменьшаются только тома):

```bash
helm upgrade --install ecommerce-infra deploy/helm/infra -n ecommerce \
  -f deploy/helm/infra/values-prod.yaml \
  --set postgresql.persistence.size=2Gi \
  --set mongodb.persistence.size=2Gi \
  --set kafka.persistence.size=4Gi \
  --set redis.persistence.size=1Gi \
  --wait --timeout 20m

helm upgrade --install observability deploy/helm/observability -n ecommerce \
  -f deploy/helm/observability/values-prod.yaml \
  --set prometheus.persistence.size=2Gi \
  --set tempo.persistence.size=2Gi \
  --set loki.persistence.size=2Gi \
  --set grafana.persistence.enabled=false \
  --wait --timeout 20m
```

Нюансы:

* `postgresql.persistence.size` — **на каждый** из четырёх инстансов
  (`auth-postgres`, `inventory-postgres`, `order-postgres`, `user-postgres`).
* StorageClass по умолчанию в k3s — `local-path`: данные лежат на локальном
  диске узла, `ReclaimPolicy: Delete`. Для настоящего прода берите StorageClass
  с репликацией/бэкапами и прод-размеры.
* **Уменьшить PVC после создания нельзя** (расширять — можно, если
  `allowVolumeExpansion`): планируйте размеры до установки.
* Возня с диском не влияет на образ и режим приложения: `global.environment`
  остаётся `prod`, HPA/PDB/observability работают как в проде.

---

## 14. Swagger UI: где лежит и где ловушка с CORS

### 14.1 Где он

Swagger UI отдаёт сам `api-gateway` — роут регистрируется в
`services/api-gateway/cmd/server/main.go`:

```go
httpServer.App.Get("/swagger/*", swaggo.New(swaggo.Config{
    URL:         "/api/openapi.yaml",
    DeepLinking: true,
}))
httpServer.App.Get("/api/openapi.yaml", func(c fiber.Ctx) error {
    return c.SendFile("./api/openapi.yaml")
})
```

То есть:

| Что | Где |
|---|---|
| Swagger UI | `<точка входа>/swagger/index.html` |
| Спека | `<точка входа>/api/openapi.yaml` (файл `services/api-gateway/api/openapi.yaml`, вшит в образ через `COPY api ./api`) |

Открыть локально (туннель в ingress, §12.6):

```bash
kubectl -n ingress-nginx port-forward svc/ingress-nginx-controller 8080:80
# затем в браузере:
#   http://localhost:8080/swagger/index.html
```

Авторизация в UI: `POST /api/v1/auth/login` → скопировать `access_token` →
кнопка **Authorize** → вставить токен **без** слова `Bearer`.

### 14.2 «Failed to fetch» и почему это обычно НЕ CORS

Swagger UI берёт адрес, по которому слать запросы, из блока `servers` спеки.
Если там абсолютный адрес, а UI открыт по другому — браузер шлёт запрос на
чужой (часто не слушающий) порт, получает сетевой отказ и показывает
`Failed to fetch`, а UI в подсказке просто перечисляет частые причины, включая
CORS. Настоящая причина видна в DevTools → Network: запрос ушёл **не туда**.

Было (сломано):

```yaml
servers:
  - url: http://localhost:4000   # UI на 8080, API на 4000 — и на 4000 никого
```

Стало (работает одинаково локально и на сервере):

```yaml
servers:
  - url: /                      # тот же хост и порт, что и у UI
    description: Текущий хост (через ingress / api-gateway)
  - url: http://localhost:4000  # остаётся для запуска без кластера
```

Первый элемент — значение по умолчанию для «Try it out». С относительным `/`
запросы становятся **same-origin**, а same-origin браузер по CORS не проверяет
вообще: ни preflight, ни заголовков `Access-Control-*`.

Проверить, что отдаёт кластер:

```bash
curl -s http://localhost:8080/api/openapi.yaml | sed -n '/^servers:/,/^tags:/p'
```

> Спека вшита в образ api-gateway, поэтому после правки `servers` образ надо
> пересобрать и подменить (§7). Без этого кластер продолжит отдавать старую
> спеку, даже если файл в репозитории уже исправлен.

### 14.3 Когда CORS всё-таки нужен

Только если UI (или фронтенд) открыт с **другого** origin: например фронтенд на
`http://localhost:3000`, а API на `http://localhost:8080`. Тогда origin надо
добавить в allowlist — он живёт в values и попадает в конфиги всех сервисов:

```yaml
# values-local-tls.yaml
defaults:
  allowedOrigins:
    - "http://localhost:3000"
    - "http://localhost:8080"
    - "https://api.ecommerce.local"
```

Проверить preflight «как из браузера»:

```bash
curl -s -o /dev/null -D - -X OPTIONS \
  -H 'Origin: http://localhost:3000' \
  -H 'Access-Control-Request-Method: POST' \
  -H 'Access-Control-Request-Headers: content-type' \
  http://localhost:8080/api/v1/auth/login | grep -i 'access-control'
```

Если в ответе нет `Access-Control-Allow-Origin` с вашим origin — браузер
заблокирует запрос. Для same-origin (`Origin` совпадает с адресом страницы)
preflight не выполняется вовсе, и этот заголовок не нужен.

---

## 15. Реальные OAuth-креды: где взять и как положить

Раздел про то, откуда берутся `client_id` / `client_secret` и куда их класть,
чтобы вход через Google и GitHub работал. Механика доставки в поды описана в
§4.3, здесь — только про сами значения.

### 15.1 Где их взять

**Google**

1. [Google Cloud Console](https://console.cloud.google.com/apis/credentials) →
   **APIs & Services → Credentials → Create credentials → OAuth client ID**.
2. Application type: **Web application**.
3. **Authorized redirect URIs** — добавить:
   ```
   https://<домен>/api/v1/auth/oauth/google/callback
   ```
   Для локальной отладки — `http://localhost:8080/api/v1/auth/oauth/google/callback`
   (Google разрешает `http` только для `localhost`).
4. **Authorized JavaScript origins оставить пустым** — наш флоу серверный.
   Если вписать туда адрес, консоль ответит
   `Invalid Origin: must end with a public top-level domain`.
5. Скопировать **Client ID** и **Client secret**.

**GitHub**

1. GitHub → **Settings → Developer settings → OAuth Apps → New OAuth App**.
2. Homepage URL: `https://<домен>`.
3. **Authorization callback URL**:
   ```
   https://<домен>/api/v1/auth/oauth/github/callback
   ```
4. **Generate a new client secret** и скопировать **Client ID** + секрет.

### 15.2 Куда НЕ класть

* **В `values*.yaml`** — они уедут в git и в историю Helm-релизов.
* **В git вообще** — в репозитории их быть не должно; CI прогоняет `gitleaks`.
* **В образ** — `.env` и `*.pem` исключены корневым `.dockerignore`.

### 15.3 Локально (dev)

Значения живут в `services/auth-service/.env` (файл в `.gitignore`), оттуда их
забирает скрипт:

```bash
# services/auth-service/.env
OAUTH_GOOGLE_CLIENT_ID=...
OAUTH_GOOGLE_CLIENT_SECRET=...
OAUTH_GOOGLE_REDIRECT_URL=http://localhost:8080/api/v1/auth/oauth/google/callback
OAUTH_GITHUB_CLIENT_ID=...
OAUTH_GITHUB_CLIENT_SECRET=...
OAUTH_GITHUB_REDIRECT_URL=http://localhost:8080/api/v1/auth/oauth/github/callback

# затем
deploy/helm/scripts/create-app-secret.sh
kubectl -n ecommerce rollout restart deploy/auth-service
```

Проще — не редактировать `.env` руками, а выставить базу редиректа:

```bash
deploy/helm/scripts/set-oauth-redirect.sh http://localhost:8080
```

### 15.4 Prod / удалённый сервер

**Вариант 1 (рекомендую): файл с секретами на сервере + скрипт.**
Файл лежит вне git, например `/root/ecommerce-oauth.env` с правами `600`:

```bash
cat >/root/ecommerce-oauth.env <<'EOF'
OAUTH_GOOGLE_CLIENT_ID=...
OAUTH_GOOGLE_CLIENT_SECRET=...
OAUTH_GITHUB_CLIENT_ID=...
OAUTH_GITHUB_CLIENT_SECRET=...
EOF
chmod 600 /root/ecommerce-oauth.env

# JWT_KEY — боевой ключ, который вы положили на сервер заранее;
# GENERATE_JWT_KEY=0 запрещает скрипту придумать новый ключ,
# REDIRECT_BASE сам построит оба redirect URL под боевой домен.
ENV_FILE=/root/ecommerce-oauth.env \
JWT_KEY=/etc/ecommerce/jwt-private.pem \
GENERATE_JWT_KEY=0 \
REDIRECT_BASE=https://api.вашдомен.ru \
  deploy/helm/scripts/create-app-secret.sh
```

Пароли БД/Redis/Mongo скрипт возьмёт из умолчаний — если в кластере они другие,
задайте их явно (`POSTGRES_AUTH_PASSWORD=...`, `REDIS_PASSWORD=...`, `MONGO_URI=...`).

**Вариант 2: без файла, одним `kubectl`.**

```bash
kubectl -n ecommerce create secret generic ecommerce-shop-secrets \
  --from-literal=app-secret="$(openssl rand -hex 32)" \
  --from-literal=postgres-auth-password="$(openssl rand -hex 24)" \
  --from-literal=postgres-user-password="$(openssl rand -hex 24)" \
  --from-literal=postgres-inventory-password="$(openssl rand -hex 24)" \
  --from-literal=postgres-order-password="$(openssl rand -hex 24)" \
  --from-literal=redis-password="$(openssl rand -hex 24)" \
  --from-literal=mongodb-uri="mongodb://admin:ПАРОЛЬ@mongodb:27017" \
  --from-file=jwt-private-key=/etc/ecommerce/jwt-private.pem \
  --from-literal=oauth-google-client-id="..." \
  --from-literal=oauth-google-client-secret="..." \
  --from-literal=oauth-google-redirect-url="https://api.вашдомен.ru/api/v1/auth/oauth/google/callback" \
  --from-literal=oauth-github-client-id="..." \
  --from-literal=oauth-github-client-secret="..." \
  --from-literal=oauth-github-redirect-url="https://api.вашдомен.ru/api/v1/auth/oauth/github/callback" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Чарт ставится с `secrets.create=false` + `existingSecret` (так уже настроено в
`values-server.yaml` и `values-prod.yaml`), поэтому значения из Secret'а должны
совпадать с паролями инфраструктуры (`postgresql-credentials`, `redis-credentials`,
`mongodb-credentials`) — иначе сервисы не подключатся.

**Вариант 3: из GitHub Actions.** Технически возможно (секреты в GitHub Secrets
+ шаг, создающий Secret в кластере), но тогда приватный ключ подписи JWT лежит в
CI. Набросок и предупреждение — в конце `.github/workflows/deploy.yaml`.

### 15.5 Проверка и ротация

```bash
# креды доехали (значения не печатаем)
kubectl -n ecommerce exec deploy/auth-service -c auth-service -- \
  sh -c 'echo "google client_id: ${#OAUTH_GOOGLE_CLIENT_ID} симв."; echo "redirect: $OAUTH_GOOGLE_REDIRECT_URL"'

# какой auth_url построится (в нём виден redirect_uri, который уйдёт в Google)
curl -s http://localhost:8080/api/v1/auth/oauth/google/login \
  | python3 -c 'import sys,json,urllib.parse as u; print(u.parse_qs(u.urlparse(json.load(sys.stdin)["auth_url"]).query)["redirect_uri"][0])'
```

Ротация: поменять значение в консоли провайдера → обновить Secret
(`create-app-secret.sh` / `kubectl apply`) → `rollout restart deploy/auth-service`.
Без перезапуска поды продолжат работать со старыми значениями: они приходят через
`secretKeyRef`, а это не триггерит rollout.
