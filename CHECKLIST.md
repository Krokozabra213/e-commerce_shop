# Памятка: пуш, CI/CD и запуск в Kubernetes

Три сценария в одном месте: что проверить перед пушем, как поднять проект после
клонирования и как переключать версии и профили.

Глубокая документация: [`deploy/helm/DEPLOY-RUNBOOK.md`](deploy/helm/DEPLOY-RUNBOOK.md)
(пошаговые сценарии и разбор проблем) и [`deploy/helm/README.md`](deploy/helm/README.md)
(устройство чартов).

---

## 1. Перед пушем

### Что запустит CI

`.github/workflows/ci.yaml` — на push и pull request в `main` и `prod`:

| Job | Команда | Что проверяет |
|---|---|---|
| `govulncheck` | `govulncheck ./...` по каждому модулю `go.work` | известные уязвимости в зависимостях |
| `trivy` | `trivy fs --config trivy.yaml` | HIGH/CRITICAL в файлах (конфиг в `trivy.yaml`) |
| `gitleaks` | `gitleaks-action` с `.gitleaks.toml` | утечки секретов в истории |
| `lint` | `make lint`, `make fmt-check` | golangci-lint v2.13.2 + форматирование |
| `unit-test` | `make test-race` | unit-тесты с race-детектором |
| `integration-test` | `make test-integration` | тесты с тегом `integration` (testcontainers, нужен docker) |
| `codegen` | `buf lint && buf generate`, `make generate` + `git diff --exit-code` | сгенерированный код совпадает со спеками |

`.github/workflows/deploy.yaml` — **только на push тега `v*`** (или вручную):
собирает 10 образов, пушит в GHCR и падает, если `appVersion` в
`deploy/helm/ecommerce-shop/Chart.yaml` не совпадает с тегом.

### Локальная проверка перед пушем

```bash
# 0) версии как в CI
go version          # 1.26.9
docker version      # нужен для integration-тестов

# 1) линт и форматирование (то же, что job lint)
make fmt-check
make lint

# 2) тесты (то же, что unit-test и integration-test)
make test-race
make test-integration

# 3) уязвимости и утечки (то же, что govulncheck, trivy, gitleaks)
# govulncheck запускается по каждому модулю: в корне go.work, а не go.mod
for d in $(go list -m -f '{{.Dir}}'); do
  [ -n "$(find "$d" -name '*.go' -print -quit)" ] && (cd "$d" && govulncheck ./...)
done
trivy fs --config trivy.yaml .
gitleaks detect --config .gitleaks.toml

# 4) кодогенерация (то же, что job codegen)
make -C api proto-all
make -C services/product-service generate
git diff --exit-code || echo "сгенерированный код устарел — закоммитьте его"

# 5) чарты
make -C deploy/helm lint
make -C deploy/helm validate
```

Чего не хватает из инструментов:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

`buf` ставить не нужно: `make -C api proto-all` сам скачает его в `api/bin/`.

### Релиз (тег)

Версия живёт в одном месте — `appVersion` в `deploy/helm/ecommerce-shop/Chart.yaml`:

```bash
deploy/helm/scripts/bump-version.sh v1.2.3
git commit -am "release: v1.2.3"
git tag v1.2.3
git push && git push --tags
```

После этого `deploy.yaml` соберёт и запушит образы `ghcr.io/krokozabra213/<svc>:v1.2.3`.
Порядок важен: если тег поставить без бампа `appVersion`, job упадёт с подсказкой.

### Секреты

В git не должны попадать `.env` и приватные ключи — за этим следит `gitleaks`.
Уже закрыто `.gitignore`:

* `services/*/.env`
* `services/auth-service/certs/*.pem`

Если добавляете новый секрет — сразу добавляйте путь в `.gitignore`, иначе
`gitleaks` уронит CI (а если и нет — секрет утечёт).

---

## 2. Запуск в Kubernetes после клонирования

```bash
git clone <repo> && cd e-commerce_shop
```

### Инструменты

```bash
kubectl version --client && helm version && openssl version && python3 --version
# docker нужен только для локальной сборки образов
```

### Шаг 1. Кластер и ingress

На текущей машине используется **k3s**: он слушает 80/443 на самой машине,
поэтому внешний адрес доступен сразу и пробрасывать порты не нужно.

**k3s (то же окружение, что на сервере):**

```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--disable traefik" sh -
mkdir -p ~/.kube
sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
sudo chown "$(id -u):$(id -g)" ~/.kube/config

helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx --create-namespace \
  -f deploy/cluster/ingress-nginx-values.yaml \
  --set controller.config.hsts=false \
  --wait --timeout 10m
```

### Шаг 2. Образы

С k3s и публичными пакетами GHCR локальная сборка не нужна — образы тянутся из
реестра. Собирать нужно только для итераций по коду, с импортом в containerd:

```bash
deploy/helm/scripts/build-images.sh
deploy/helm/scripts/build-images.sh --print-tags | xargs -I{} sh -c 'docker save {} | sudo k3s ctr images import -'
```

### Шаг 3. Установка

```bash
# Secret приложения и TLS-сертификат
REDIRECT_BASE=http://localhost deploy/helm/scripts/create-app-secret.sh
HOST=api.ecommerce.local deploy/helm/scripts/create-tls-secret.sh
kubectl apply -f deploy/cluster/ingress-localhost.yaml

# инфраструктура
helm upgrade --install ecommerce-infra deploy/helm/infra -n ecommerce \
  --create-namespace -f deploy/helm/infra/values-dev.yaml --wait --timeout 20m

# приложение: образы из GHCR, 3 реплики, HTTPS
helm upgrade --install ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  -f deploy/helm/ecommerce-shop/values-local-tls.yaml \
  -f deploy/helm/ecommerce-shop/values-ghcr.yaml \
  --set secrets.create=false --set secrets.existingSecret=ecommerce-shop-secrets \
  --wait --timeout 15m
```

Альтернатива на случай, когда нужно поднять всё одной командой без GHCR —
`deploy/helm/deploy.sh`: она создаёт Secret сама, а приватный ключ JWT
генерируется автоматически (образы должны быть импортированы в k3s, шаг 2).

### Шаг 4. Проверка

```bash
kubectl -n ecommerce get pods
kubectl -n ecommerce get jobs

# единая точка входа, без пробросов портов
curl -s  http://localhost/healthz
curl -sk https://api.ecommerce.local/healthz
# Swagger UI: http://localhost/swagger/index.html
```

Браузер и Swagger UI: `deploy/helm/DEPLOY-RUNBOOK.md` §12 и §14.

### Если нужны реальные OAuth-креды

Вместо шага 3 (база редиректа — тот адрес, по которому открыт вход; для
localhost Google разрешает http):

```bash
REDIRECT_BASE=http://localhost deploy/helm/scripts/create-app-secret.sh
kubectl -n ecommerce rollout restart deploy/auth-service
```

Где взять `client_id`/`client_secret` и куда их класть — §15 в runbook.
Сменить адрес потом: `deploy/helm/scripts/set-oauth-redirect.sh https://<домен>`.

### Публичные или приватные пакеты в GHCR

Профиль `values-ghcr.yaml` ссылается на pull-секрет `ghcr-pull`, но при
**публичных** пакетах он не нужен: kubelet выдаёт warning
`FailedToRetrieveImagePullSecret` и всё равно скачивает образ. Если пакеты
приватные — секрет обязателен:

```bash
CR_PAT=ghp_xxx deploy/helm/scripts/create-ghcr-pull-secret.sh
```

Проверить видимость пакета можно анонимно: `docker manifest inspect
ghcr.io/krokozabra213/api-gateway:<тег>`.

---

## 3. Профили и версии

### Профили (что и в какой конфигурации поднимается)

| Профиль | Для чего | Особенности |
|---|---|---|
| `values-dev.yaml` | локальная разработка | http, по 1 реплике, debug-логи, образы без реестра (из узла) |
| `values-ghcr.yaml` | оверлей к dev | образы из `ghcr.io`; pull-секрет `ghcr-pull` нужен только для приватных пакетов |
| `values-local-tls.yaml` | локально как в проде | HTTPS (самоподписанный), 3 реплики, PDB, распределение по узлам |
| `values-server.yaml` | один сервер (k3s) | HTTPS через cert-manager, 3 реплики, инфра в кластере, ghcr.io |
| `values-prod.yaml` | prod | managed-БД, HPA, NetworkPolicy, cert-manager |

Профили накладываются слева направо, базовые значения — всегда из `values.yaml`:

```bash
helm upgrade --install ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-dev.yaml \
  -f deploy/helm/ecommerce-shop/values-local-tls.yaml
```

### Версии образов

Тег = `appVersion` чарта. Приоритет переопределения:

```
services.<name>.image.tag   ->   global.imageTag   ->   .Chart.AppVersion
```

Поднять конкретную версию, не трогая файлы:

```bash
helm upgrade ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-server.yaml \
  --set global.imageRegistry=ghcr.io \
  --set global.imageTag=v1.2.3 \
  --wait --timeout 20m
```

Проверить, во что отрендерится, без кластера:

```bash
helm template ecommerce deploy/helm/ecommerce-shop -n ecommerce \
  -f deploy/helm/ecommerce-shop/values-server.yaml \
  --set global.imageTag=v1.2.3 | grep -E '^\s+image:' | sort -u
```

### Локальная сборка: импорт образов в k3s

Собранные локально образы нужно положить в containerd k3s (одна команда):

```bash
deploy/helm/scripts/build-images.sh
deploy/helm/scripts/build-images.sh --print-tags | xargs -I{} sh -c 'docker save {} | sudo k3s ctr images import -'
kubectl -n ecommerce rollout restart deploy
```

Для проверки пайплайна образы просто собираются в GHCR (раздел 3).

> После смены `appVersion` (`bump-version.sh`) локальные образы нужно пересобрать
> и снова импортировать: чарт будет искать новый тег.
