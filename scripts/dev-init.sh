#!/usr/bin/env bash
# =============================================================================
# dev-init.sh — подготовка локального окружения для docker compose
# =============================================================================
# Что делает:
#   1) создаёт services/<сервис>/.env из значений по умолчанию, совпадающих с
#      инфраструктурой docker-compose.yaml (postgres/postgres, redis/redis,
#      mongo admin/admin123). Существующие .env НЕ перезаписываются;
#   2) генерирует приватный ключ подписи JWT, если его нет.
#
# Запускается автоматически из `make dev-up`, но можно вызвать отдельно:
#   ./scripts/dev-init.sh
#
# Переменные:
#   FORCE=1  — перезаписать существующие .env (по умолчанию сохраняются)
#
# OAuth: в сгенерированных .env стоят заглушки `disabled`, поэтому сервисы
# стартуют, но вход через Google/GitHub работать не будет. Реальные значения
# нужно вписать в services/auth-service/.env вручную (см. README.md).
# =============================================================================
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

FORCE="${FORCE:-0}"

# Значения, совпадающие с docker-compose.yaml.
APP_SECRET="dev-secret-change-me"
POSTGRES_PASSWORD="postgres"
REDIS_ADDR="redis:6379"
REDIS_PASSWORD="redis"
MONGO_URI="mongodb://admin:admin123@mongodb:27017/products?authSource=admin"
JWT_KEY="services/auth-service/certs/jwt-private.pem"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[x]\033[0m %s\n' "$*" >&2; exit 1; }

command -v openssl >/dev/null 2>&1 || die "нужен openssl (генерация ключа JWT)"

created=0
skipped=0

write_env() {
  local path="$1"
  if [ -s "${path}" ] && [ "${FORCE}" != "1" ]; then
    skipped=$((skipped + 1))
    return 0
  fi
  mkdir -p "$(dirname "${path}")"
  cat > "${path}"
  log "создан ${path}"
  created=$((created + 1))
}

write_env services/api-gateway/.env <<EOF
APP_ENV=dev
APP_SECRET=${APP_SECRET}

REDIS_ADDR=${REDIS_ADDR}
REDIS_PASSWORD=${REDIS_PASSWORD}
EOF

write_env services/auth-service/.env <<EOF
APP_ENV=dev
APP_SECRET=${APP_SECRET}

POSTGRES_PASSWORD=${POSTGRES_PASSWORD}

REDIS_ADDR=${REDIS_ADDR}
REDIS_PASSWORD=${REDIS_PASSWORD}

AUTH_JWT_PRIVATE_KEY_PATH=./certs/jwt-private.pem
AUTH_JWT_KEY_ID=auth-key-v1

# Заглушки: сервис стартует, вход через провайдеров не работает.
# Реальные значения — из настроек OAuth-приложений Google и GitHub.
OAUTH_GOOGLE_CLIENT_ID=disabled
OAUTH_GOOGLE_CLIENT_SECRET=disabled
OAUTH_GOOGLE_REDIRECT_URL=http://localhost:8100/api/v1/auth/oauth/google/callback

OAUTH_GITHUB_CLIENT_ID=disabled
OAUTH_GITHUB_CLIENT_SECRET=disabled
OAUTH_GITHUB_REDIRECT_URL=http://localhost:8100/api/v1/auth/oauth/github/callback

KAFKA_CLIENT_ID=auth-service
EOF

write_env services/user-service/.env <<EOF
APP_ENV=dev
APP_SECRET=${APP_SECRET}

POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
EOF

write_env services/inventory-service/.env <<EOF
APP_ENV=dev
APP_SECRET=${APP_SECRET}

POSTGRES_PASSWORD=${POSTGRES_PASSWORD}

KAFKA_CLIENT_ID=inventory-service
EOF

write_env services/order-service/.env <<EOF
APP_ENV=dev
APP_SECRET=${APP_SECRET}

POSTGRES_PASSWORD=${POSTGRES_PASSWORD}

KAFKA_CLIENT_ID=order-service
EOF

write_env services/product-service/.env <<EOF
APP_ENV=dev
APP_SECRET=${APP_SECRET}

MONGO_URI=${MONGO_URI}
EOF

if [ -s "${JWT_KEY}" ]; then
  log "ключ JWT уже есть: ${JWT_KEY}"
else
  mkdir -p "$(dirname "${JWT_KEY}")"
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "${JWT_KEY}" 2>/dev/null
  chmod 600 "${JWT_KEY}"
  log "сгенерирован ключ JWT: ${JWT_KEY}"
fi

openssl pkey -in "${JWT_KEY}" -noout 2>/dev/null \
  || die "файл ${JWT_KEY} не является корректным приватным ключом"

printf '\n'
log "готово: создано .env — ${created}, уже было — ${skipped}"

if grep -qE '^OAUTH_(GOOGLE|GITHUB)_CLIENT_ID=disabled' services/auth-service/.env 2>/dev/null; then
  printf '\n'
  warn "OAuth-креды — заглушки. Сервисы запустятся, но вход через Google/GitHub не работает."
  warn "Чтобы включить, впишите реальные значения в services/auth-service/.env:"
  warn "  OAUTH_GOOGLE_CLIENT_ID / _CLIENT_SECRET / _REDIRECT_URL"
  warn "  OAUTH_GITHUB_CLIENT_ID / _CLIENT_SECRET / _REDIRECT_URL"
  warn "Затем: make dev-down && make dev-up"
fi
