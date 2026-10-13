# e-commerce_shop

Микросервисный интернет-магазин на Go. Шесть сервисов, асинхронный обмен через
Kafka (outbox-паттерн), PostgreSQL / MongoDB / Redis, телеметрия через
OpenTelemetry.

| Сервис | Порт | Назначение | Хранилище |
|---|---|---|---|
| `api-gateway` | 4000 | единая точка входа: JWT, роли, проксирование | Redis (rate limit) |
| `auth-service` | 8100 | регистрация, вход, JWT (RS256), OAuth, подтверждение email | PostgreSQL |
| `user-service` | 8200 | профили и роли | PostgreSQL |
| `product-service` | 8300 (gRPC 44400) | товары и категории | MongoDB |
| `inventory-service` | 8400 (gRPC 44100) | остатки и резервы | PostgreSQL |
| `order-service` | 8600 | заказы | PostgreSQL |

```
api/                 protobuf-контракты и сгенерированный код
infra/               общая библиотека: конфиг, логи, Kafka, JWT, телеметрия
services/<сервис>/   сервисы (cmd, internal, configs, migrations, Dockerfile)
deploy/helm/         чарты и документация по развёртыванию в Kubernetes
docker-compose.yaml  локальный запуск всего стека
scripts/dev-init.sh  подготовка локального окружения
scripts/k3d.sh       локальный кластер k3d + helm-чарты (make k3d-up)
```

---

## Локальный запуск (docker compose)

### Что нужно

`docker` с плагином compose, `make`, `openssl`, `git`.

### Запуск

```bash
git clone <repo> && cd e-commerce_shop
make dev-up
```

`dev-up` делает две вещи: сначала `scripts/dev-init.sh` готовит окружение, потом
`docker compose up -d --build` поднимает стек.

`dev-init.sh` создаёт то, чего нет в git:

* `services/<сервис>/.env` со значениями, совпадающими с инфраструктурой из
  `docker-compose.yaml` (postgres/postgres, redis/redis, mongo admin/admin123).
  **Существующие `.env` не перезаписываются** — их можно спокойно править;
* `services/auth-service/certs/jwt-private.pem` — приватный ключ подписи JWT
  (генерируется RSA-2048, права `644`: контейнеры работают от uid 65534 и должны
  его читать — см. «Безопасность образов»).

Файлы `.env` и `*.pem` в git не хранятся (`.gitignore`), поэтому у каждого
разработчика они свои. Повторный `make dev-up` ничего не перезапишет.

> Первый запуск собирает 6 Go-образов и занимает несколько минут. Дальше
> пересборка идёт из кэша.

### OAuth нужно настроить вручную

После `make dev-up` сервисы работают, но вход через Google и GitHub — нет: в
сгенерированном `.env` стоят заглушки `disabled`. Это сделано намеренно, чтобы
стек поднимался без секретов.

Чтобы включить:

1. Создайте OAuth-приложения и скопируйте креды:

   * **Google** — [Cloud Console → Credentials](https://console.cloud.google.com/apis/credentials)
     → Create credentials → OAuth client ID → Web application.
     Authorized redirect URI:
     `http://localhost:8100/api/v1/auth/oauth/google/callback`
     Поле *Authorized JavaScript origins* оставьте пустым.
   * **GitHub** — Settings → Developer settings → OAuth Apps → New OAuth App.
     Authorization callback URL:
     `http://localhost:8100/api/v1/auth/oauth/github/callback`

2. Впишите значения в `services/auth-service/.env`:

   ```
   OAUTH_GOOGLE_CLIENT_ID=...
   OAUTH_GOOGLE_CLIENT_SECRET=...
   OAUTH_GITHUB_CLIENT_ID=...
   OAUTH_GITHUB_CLIENT_SECRET=...
   ```

   URL-ы редиректа там уже проставлены и совпадают с адресами выше.

3. Перезапустите auth-service:

   ```bash
   make dev-down && make dev-up
   ```

Без этого шага всё остальное работает: регистрация по email и паролю, заказы,
каталог.

### Куда заходить

| Что | Адрес |
|---|---|
| API (через gateway) | http://localhost:4000 |
| Swagger UI | http://localhost:4000/swagger/index.html |
| Grafana | http://localhost:3000 |
| Kafka UI | http://localhost:8085 |
| Mongo Express | http://localhost:8101 |
| Метрики otel-collector | http://localhost:8888/metrics |

Сервисы также доступны напрямую по своим портам (см. таблицу выше) — это удобно
при отладке конкретного сервиса.

### Команды

```bash
make            # список всех целей
make dev-up     # поднять стек
make dev-down   # остановить (данные в томах сохраняются)
make dev-reset  # остановить и удалить данные
make dev-ps     # статус контейнеров
make dev-logs SVC=auth-service
```

### Тесты и линт

```bash
make test              # все тесты
make test-race         # с race-детектором
make test-integration  # интеграционные (testcontainers, нужен docker)
make lint              # golangci-lint по всем модулям
make fmt               # форматирование
```

---

## Локальный запуск в Kubernetes (k3d, одной командой)

Полноценный прогон **helm-чартов** в настоящем Kubernetes — без ручной установки
кластера и без `kubectl`-простыней.

### Что нужно

`docker`, `make`, `kubectl`, `helm`, `openssl`, `curl`.

Сам `k3d` ставить не обязательно: если его нет, `make k3d-up` скачает бинарь
нужной версии в `~/.local/bin` (отключить — `K3D_AUTO_INSTALL=0`, поставить
отдельно — `make k3d-install`).

### Запуск

```bash
git clone <repo> && cd e-commerce_shop
make k3d-up
```

Команда делает всё сама:

1. создаёт кластер `k3d-ecommerce` (Traefik отключён, ingress проброшен на
   `localhost:8080`/`8443`);
2. ставит `ingress-nginx`;
3. готовит JWT-ключ (тот же `scripts/dev-init.sh`, что и для compose);
4. ставит инфраструктуру (`infra`, профиль `values-dev.yaml`): PostgreSQL ×4,
   Redis, MongoDB, Kafka (KRaft), Schema Registry;
5. ставит приложение (`ecommerce-shop`, `values-dev.yaml`) в **dev-режиме** и
   добавляет http-Ingress на хост `localhost`.

Образы берутся из публичного GHCR (`ghcr.io/krokozabra213/<сервис>:<appVersion>`),
поэтому сборка не нужна. Первый запуск скачивает образы и занимает несколько
минут.

### Куда заходить

| Что | Адрес |
|---|---|
| API (через ingress) | http://localhost:8080 |
| Swagger UI | http://localhost:8080/swagger/index.html |
| healthz | `curl -s http://localhost:8080/healthz` |

Порты переопределяются: `K3D_HTTP_PORT=9090 K3D_HTTPS_PORT=9443 make k3d-up`.

### Команды

```bash
make k3d-up          # создать кластер и выкатить dev-стек (образы из GHCR)
make k3d-up-local    # то же, но образы собираются из текущего кода
make k3d-load        # пересобрать образы, импортировать в k3d, перезапустить поды
make k3d-status      # статус кластера и подов
make k3d-logs SVC=auth-service
make k3d-down        # удалить кластер (кластер одноразовый)
make k3d-reset       # удалить и создать заново
```

`make k3d-up` ставит опубликованную версию из GHCR. Если вы правили код и хотите
проверить именно его — `make k3d-up-local` (или, на уже поднятом кластере,
`make k3d-load`): образы собираются локально, импортируются в containerd k3d и
релиз переключается на них.

### OAuth

Как и в docker compose, вход через Google/GitHub по умолчанию выключен
(значения-заглушки `disabled` в `deploy/helm/ecommerce-shop/values-dev.yaml`).
Регистрация по email/паролю, каталог и заказы работают. Чтобы включить OAuth,
впишите реальные `client_id`/`secret`/`redirect_url` в
`secrets.values.oauth` профиля `values-dev.yaml` и повторите `make k3d-up`.

### k3d или k3s?

| | k3d | k3s |
|---|---|---|
| Где живёт | контейнеры Docker, кластер одноразовый | systemd-служба на машине, живёт постоянно |
| Запуск | `make k3d-up` | `curl … get.k3s.io` + ingress-nginx (см. ниже) |
| Остановка | `make k3d-down` | `sudo systemctl stop k3s` |
| Для чего | быстрый прогон чартов, CI, эксперименты | постоянный локальный стенд |

k3d не трогает ваш k3s: кластер и его kube-контекст (`k3d-ecommerce`) отдельные.

---

## Развёртывание в Kubernetes (k3s)

Локальный кластер — k3s, образы берутся из GHCR. Пошаговые инструкции:

* [`CHECKLIST.md`](CHECKLIST.md) — короткая памятка: что проверить перед пушем,
  как поднять проект после клонирования, как переключать профили и версии;
* [`deploy/helm/README.md`](deploy/helm/README.md) — быстрый старт и устройство
  чартов;
* [`deploy/helm/DEPLOY-RUNBOOK.md`](deploy/helm/DEPLOY-RUNBOOK.md) — подробные
  сценарии: секреты, OAuth, TLS 1.3, CI, удалённый сервер.

Релиз новой версии:

```bash
deploy/helm/scripts/bump-version.sh v1.2.3
git commit -am "release: v1.2.3"
git tag v1.2.3 && git push && git push --tags
```

GitHub Actions соберёт образы `ghcr.io/krokozabra213/<сервис>:v1.2.3`
(`.github/workflows/deploy.yaml`).

## Безопасность образов

Все прикладные образы (`services/*/Dockerfile` и `services/*/Migrate.Dockerfile`)
собираются по одной схеме:

* финальный слой — `gcr.io/distroless/static-debian12:nonroot`: нет shell,
  пакетного менеджера и busybox; `ca-certificates`, `tzdata` и `nsswitch.conf`
  уже внутри. Поэтому поды не требуют shell (пробы — `httpGet`, init-контейнеры
  ожидания используют отдельный `busybox`);
* бинарь собирается с `CGO_ENABLED=0` — это обязательное условие для
  `distroless/static`, в образе нет libc;
* в образе стоит `USER 65534:65534` (совпадает с
  `defaults.podSecurityContext.runAsUser/runAsGroup` в чарте и с `user:` в
  `docker-compose.yaml`). Раньше `USER` не было, и образы запускались от root;
* в Kubernetes это дополнительно подкреплено `runAsNonRoot: true`,
  `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false` и
  `capabilities.drop: [ALL]` (`deploy/helm/ecommerce-shop/values.yaml`);
* CI (job `image-scan`) собирает все 10 образов, проверяет, что `Config.User`
  не root, и сканирует их `trivy image` (HIGH/CRITICAL). В `deploy.yaml` те же
  проверки стоят **до** push в GHCR.

Приватный ключ JWT в образ не попадает: в K8s он монтируется из Secret
(`defaultMode: 0440`, доступ через `fsGroup: 65534`), в docker compose — с хоста.
Так как контейнер работает от uid 65534, локальный ключ должен быть читаем для
всех: `make dev-up` (`scripts/dev-init.sh`) выставляет `0644`. Это dev-only ключ;
в проде используется отдельный Secret.
