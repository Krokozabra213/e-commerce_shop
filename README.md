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
  (генерируется RSA-2048, права `600`).

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
