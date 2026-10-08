#!/usr/bin/env bash
# =============================================================================
# create-app-secret.sh — собрать Secret приложения из локальных .env и ключа JWT
# =============================================================================
# Зачем это нужно.
#   В кластере сервисы НЕ читают services/*/.env: внутри контейнера godotenv
#   файл не находит, и вся конфигурация приходит из
#     * ConfigMap — несекретные значения (files/configs/*.yaml),
#     * Secret    — секреты через env-переменные (POSTGRES_PASSWORD,
#                   REDIS_PASSWORD, MONGO_URI, OAUTH_*, APP_SECRET)
#                   плюс приватный ключ JWT (монтируется файлом).
#   Этот скрипт переносит локальные секреты в кластер ОДИН раз, после чего чарт
#   ставится с secrets.create=false + secrets.existingSecret:
#
#     helm upgrade --install ecommerce deploy/helm/ecommerce-shop \
#       -n ecommerce -f deploy/helm/ecommerce-shop/values-dev.yaml \
#       -f deploy/helm/ecommerce-shop/values-ghcr.yaml \
#       --set secrets.create=false \
#       --set secrets.existingSecret=ecommerce-shop-secrets
#
#   Плюс против secrets.create=true: реальные OAuth-секреты и пароли не попадают
#   ни в values-файлы, ни в историю Helm-релизов.
#
# Использование:
#   ./create-app-secret.sh                    # создать/обновить Secret
#   DRY_RUN=1 ./create-app-secret.sh          # показать план, ничего не менять
#   NAMESPACE=shop ./create-app-secret.sh
#   REDIRECT_BASE=http://api.ecommerce.local ./create-app-secret.sh
#
# Переменные (все необязательные; значения по умолчанию совпадают с
# deploy/helm/infra/values-dev.yaml, поэтому «просто работает»):
#   NAMESPACE                 namespace (по умолчанию ecommerce)
#   SECRET_NAME               имя Secret'а (ecommerce-shop-secrets)
#   ENV_FILE                  откуда брать OAUTH_*/APP_SECRET (auth-service/.env)
#   JWT_KEY                   путь к приватному ключу JWT
#   GENERATE_JWT_KEY          1 (по умолчанию) — сгенерировать ключ, если его
#                             нет; 0 — упасть с ошибкой (для prod)
#   REDIRECT_BASE             перезаписать redirect URL на
#                             <REDIRECT_BASE>/api/v1/auth/oauth/<provider>/callback
#   APP_SECRET, POSTGRES_{AUTH,USER,INVENTORY,ORDER}_PASSWORD, REDIS_PASSWORD
#   MONGO_URI либо MONGODB_{USER,PASSWORD,HOST,PORT,DATABASE}
#   KUBECTL                   бинарь kubectl
#
# Приоритет значения: env-переменная -> services/auth-service/.env -> умолчание.
# =============================================================================
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

KUBECTL="${KUBECTL:-kubectl}"
NAMESPACE="${NAMESPACE:-ecommerce}"
SECRET_NAME="${SECRET_NAME:-ecommerce-shop-secrets}"
DRY_RUN="${DRY_RUN:-0}"
# Генерировать приватный ключ JWT, если его нет (удобно для dev / свежего
# клона). Для prod выставьте GENERATE_JWT_KEY=0 — тогда скрипт упадёт вместо
# создания нового ключа.
GENERATE_JWT_KEY="${GENERATE_JWT_KEY:-1}"

ENV_FILE="${ENV_FILE:-${REPO_ROOT}/services/auth-service/.env}"
JWT_KEY="${JWT_KEY:-${REPO_ROOT}/services/auth-service/certs/jwt-private.pem}"

DEFAULT_APP_SECRET="dev-secret-change-me"
DEFAULT_PG_PASSWORD="postgres"
DEFAULT_REDIS_PASSWORD="redis"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[x]\033[0m %s\n' "$*" >&2; exit 1; }

# -----------------------------------------------------------------------------
# Чтение ключа из .env-файла (значение — всё после первого '=').
# -----------------------------------------------------------------------------
env_get() {
  local key="$1" file="$2" line
  [ -f "${file}" ] || return 0
  line="$(grep -E "^[[:space:]]*${key}=" "${file}" | tail -n 1 || true)"
  [ -n "${line}" ] || return 0
  printf '%s' "${line#*=}" | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//"
}

# Значение по цепочке: env-переменная -> .env (fallback_key) -> умолчание.
# Печатает "<источник> <значение>"; источник нужен только для отчёта.
pick() {
  local var="$1" fallback_key="$2" default="$3" v
  if [ -n "${!var:-}" ]; then printf 'env %s' "${!var}"; return; fi
  v="$(env_get "${var}" "${ENV_FILE}")"
  if [ -n "${v}" ]; then printf '.env %s' "${v}"; return; fi
  if [ -n "${fallback_key}" ]; then
    v="$(env_get "${fallback_key}" "${ENV_FILE}")"
    if [ -n "${v}" ]; then printf '.env(%s) %s' "${fallback_key}" "${v}"; return; fi
  fi
  printf 'default %s' "${default}"
}

src_of() { printf '%s' "${1%% *}"; }
val_of() { printf '%s' "${1#* }"; }

if [ ! -f "${ENV_FILE}" ]; then
  warn "не найден ${ENV_FILE} — OAuth/APP_SECRET будут взяты из умолчаний"
fi

# --- APP_SECRET (во всех .env одно и то же значение) --------------------------
_r="$(pick APP_SECRET "" "${DEFAULT_APP_SECRET}")"
APP_SECRET_SRC="$(src_of "${_r}")"; APP_SECRET="$(val_of "${_r}")"

# --- Пароли PostgreSQL: у каждого инстанса свой ключ в Secret'е ---------------
_r="$(pick POSTGRES_AUTH_PASSWORD      POSTGRES_PASSWORD "${DEFAULT_PG_PASSWORD}")"
PG_AUTH_SRC="$(src_of "${_r}")";      POSTGRES_AUTH_PASSWORD="$(val_of "${_r}")"
_r="$(pick POSTGRES_USER_PASSWORD      POSTGRES_PASSWORD "${DEFAULT_PG_PASSWORD}")"
PG_USER_SRC="$(src_of "${_r}")";      POSTGRES_USER_PASSWORD="$(val_of "${_r}")"
_r="$(pick POSTGRES_INVENTORY_PASSWORD POSTGRES_PASSWORD "${DEFAULT_PG_PASSWORD}")"
PG_INVENTORY_SRC="$(src_of "${_r}")"; POSTGRES_INVENTORY_PASSWORD="$(val_of "${_r}")"
_r="$(pick POSTGRES_ORDER_PASSWORD     POSTGRES_PASSWORD "${DEFAULT_PG_PASSWORD}")"
PG_ORDER_SRC="$(src_of "${_r}")";     POSTGRES_ORDER_PASSWORD="$(val_of "${_r}")"

# --- Redis --------------------------------------------------------------------
_r="$(pick REDIS_PASSWORD "" "${DEFAULT_REDIS_PASSWORD}")"
REDIS_PASSWORD_SRC="$(src_of "${_r}")"; REDIS_PASSWORD="$(val_of "${_r}")"

# --- MongoDB: чарт собирает mongodb://<user>:<pass>@<host>:<port> -------------
# База задаётся отдельно в configs/product-service.yaml (mongo.database), поэтому
# в URI её нет: тогда драйвер берёт authSource=admin — ровно то, что создаёт
# инфра-чарт (root-пользователь в admin).
MONGODB_USER="${MONGODB_USER:-admin}"
MONGODB_PASSWORD="${MONGODB_PASSWORD:-admin123}"
MONGODB_HOST="${MONGODB_HOST:-mongodb}"
MONGODB_PORT="${MONGODB_PORT:-27017}"
if [ -z "${MONGO_URI:-}" ]; then
  MONGO_URI="mongodb://${MONGODB_USER}:${MONGODB_PASSWORD}@${MONGODB_HOST}:${MONGODB_PORT}"
  MONGO_URI_SRC="собран из MONGODB_*"
else
  MONGO_URI_SRC="env"
fi

# --- OAuth --------------------------------------------------------------------
# clientID/clientSecret/redirectURL помечены в Go-коде env-required и проверяются
# ВСЕГДА, даже при oauth.*.enabled=false, поэтому пустыми им быть нельзя:
# иначе под упадёт с "field ... is required but the value is not provided".
oauth_pick() { # $1 = имя переменной, $2 = заглушка
  local r; r="$(pick "$1" "" "$2")"
  local v; v="$(val_of "${r}")"
  [ -n "${v}" ] || r="default ${2}"
  printf '%s' "${r}"
}
_r="$(oauth_pick OAUTH_GOOGLE_CLIENT_ID disabled)"
OAUTH_GOOGLE_CLIENT_ID_SRC="$(src_of "${_r}")"; OAUTH_GOOGLE_CLIENT_ID="$(val_of "${_r}")"
_r="$(oauth_pick OAUTH_GOOGLE_CLIENT_SECRET disabled)"
OAUTH_GOOGLE_CLIENT_SECRET_SRC="$(src_of "${_r}")"; OAUTH_GOOGLE_CLIENT_SECRET="$(val_of "${_r}")"
_r="$(oauth_pick OAUTH_GOOGLE_REDIRECT_URL http://localhost:8100/api/v1/auth/oauth/google/callback)"
OAUTH_GOOGLE_REDIRECT_URL_SRC="$(src_of "${_r}")"; OAUTH_GOOGLE_REDIRECT_URL="$(val_of "${_r}")"
_r="$(oauth_pick OAUTH_GITHUB_CLIENT_ID disabled)"
OAUTH_GITHUB_CLIENT_ID_SRC="$(src_of "${_r}")"; OAUTH_GITHUB_CLIENT_ID="$(val_of "${_r}")"
_r="$(oauth_pick OAUTH_GITHUB_CLIENT_SECRET disabled)"
OAUTH_GITHUB_CLIENT_SECRET_SRC="$(src_of "${_r}")"; OAUTH_GITHUB_CLIENT_SECRET="$(val_of "${_r}")"
_r="$(oauth_pick OAUTH_GITHUB_REDIRECT_URL http://localhost:8100/api/v1/auth/oauth/github/callback)"
OAUTH_GITHUB_REDIRECT_URL_SRC="$(src_of "${_r}")"; OAUTH_GITHUB_REDIRECT_URL="$(val_of "${_r}")"

if [ -n "${REDIRECT_BASE:-}" ]; then
  OAUTH_GOOGLE_REDIRECT_URL="${REDIRECT_BASE%/}/api/v1/auth/oauth/google/callback"
  OAUTH_GOOGLE_REDIRECT_URL_SRC="REDIRECT_BASE"
  OAUTH_GITHUB_REDIRECT_URL="${REDIRECT_BASE%/}/api/v1/auth/oauth/github/callback"
  OAUTH_GITHUB_REDIRECT_URL_SRC="REDIRECT_BASE"
fi

# --- Приватный ключ JWT -------------------------------------------------------
# Если ключа нет (например, свежий клон репозитория — файл в .gitignore),
# генерируем его. Это удобно для dev, но такой ключ НЕЛЬЗЯ использовать в prod:
# он существует только на этой машине, и при потере все выданные токены станут
# невалидны, а при утечке — позволит подделывать JWT. Для prod положите ключ
# заранее и выставьте GENERATE_JWT_KEY=0, чтобы скрипт падал вместо генерации.
JWT_GENERATED=0
if [ ! -s "${JWT_KEY}" ]; then
  if [ "${GENERATE_JWT_KEY}" = "1" ] && [ "${DRY_RUN}" != "1" ]; then
    warn "нет приватного ключа JWT: ${JWT_KEY}"
    warn "генерирую новый RSA-2048 — подходит ТОЛЬКО для dev"
    mkdir -p "$(dirname "${JWT_KEY}")"
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "${JWT_KEY}"
    chmod 600 "${JWT_KEY}"
    JWT_GENERATED=1
  elif [ "${DRY_RUN}" = "1" ]; then
    warn "нет приватного ключа JWT (${JWT_KEY}); DRY_RUN — генерацию пропускаю"
    JWT_KEY_PENDING=1
  else
    die "нет приватного ключа JWT: ${JWT_KEY}, а GENERATE_JWT_KEY=0.
Сгенерировать вручную (только для dev):
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out ${JWT_KEY}
  chmod 600 ${JWT_KEY}"
  fi
fi

# Проверяем ключ, если он есть (при DRY_RUN без ключа проверять нечего).
if [ -s "${JWT_KEY}" ]; then
  openssl pkey -in "${JWT_KEY}" -noout >/dev/null 2>&1 \
    || die "файл ${JWT_KEY} не является корректным приватным ключом"
fi

# -----------------------------------------------------------------------------
# Отчёт (значения секретов НЕ печатаются — только источник)
# -----------------------------------------------------------------------------
printf '\n'
log "Secret ${SECRET_NAME} в namespace ${NAMESPACE}"
printf '    %-30s %s\n' app-secret                  "${APP_SECRET_SRC}"
printf '    %-30s %s\n' postgres-auth-password      "${PG_AUTH_SRC}"
printf '    %-30s %s\n' postgres-user-password      "${PG_USER_SRC}"
printf '    %-30s %s\n' postgres-inventory-password "${PG_INVENTORY_SRC}"
printf '    %-30s %s\n' postgres-order-password     "${PG_ORDER_SRC}"
printf '    %-30s %s\n' redis-password              "${REDIS_PASSWORD_SRC}"
printf '    %-30s %s\n' mongodb-uri                 "${MONGO_URI_SRC}"
printf '    %-30s %s\n' jwt-private-key             "файл ${JWT_KEY}$( [ "${JWT_GENERATED}" = "1" ] && echo " (СГЕНЕРИРОВАН — только для dev)" || ( [ "${JWT_KEY_PENDING:-0}" = "1" ] && echo " (будет сгенерирован)" ) )"
printf '    %-30s %s\n' oauth-google-client-id       "${OAUTH_GOOGLE_CLIENT_ID_SRC}"
printf '    %-30s %s\n' oauth-google-client-secret   "${OAUTH_GOOGLE_CLIENT_SECRET_SRC}"
printf '    %-30s %s\n' oauth-google-redirect-url    "${OAUTH_GOOGLE_REDIRECT_URL_SRC} (${OAUTH_GOOGLE_REDIRECT_URL})"
printf '    %-30s %s\n' oauth-github-client-id       "${OAUTH_GITHUB_CLIENT_ID_SRC}"
printf '    %-30s %s\n' oauth-github-client-secret   "${OAUTH_GITHUB_CLIENT_SECRET_SRC}"
printf '    %-30s %s\n' oauth-github-redirect-url    "${OAUTH_GITHUB_REDIRECT_URL_SRC} (${OAUTH_GITHUB_REDIRECT_URL})"
printf '\n'

if [ "${OAUTH_GOOGLE_CLIENT_ID}" = "disabled" ] && [ "${OAUTH_GITHUB_CLIENT_ID}" = "disabled" ]; then
  warn "OAuth-креды — заглушки: поды стартуют, но вход через Google/GitHub не работает"
fi
case "${OAUTH_GOOGLE_REDIRECT_URL}" in
  https://*) : ;;
  http://localhost*|http://127.0.0.1*) : ;;
  *) warn "Google принимает http только для localhost — для ${OAUTH_GOOGLE_REDIRECT_URL} нужен https"
     warn "и этот точный URL должен быть указан в настройках OAuth-приложения Google/GitHub" ;;
esac

ARGS=(
  "--from-literal=app-secret=${APP_SECRET}"
  "--from-literal=postgres-auth-password=${POSTGRES_AUTH_PASSWORD}"
  "--from-literal=postgres-user-password=${POSTGRES_USER_PASSWORD}"
  "--from-literal=postgres-inventory-password=${POSTGRES_INVENTORY_PASSWORD}"
  "--from-literal=postgres-order-password=${POSTGRES_ORDER_PASSWORD}"
  "--from-literal=redis-password=${REDIS_PASSWORD}"
  "--from-literal=mongodb-uri=${MONGO_URI}"
  "--from-file=jwt-private-key=${JWT_KEY}"
  "--from-literal=oauth-google-client-id=${OAUTH_GOOGLE_CLIENT_ID}"
  "--from-literal=oauth-google-client-secret=${OAUTH_GOOGLE_CLIENT_SECRET}"
  "--from-literal=oauth-google-redirect-url=${OAUTH_GOOGLE_REDIRECT_URL}"
  "--from-literal=oauth-github-client-id=${OAUTH_GITHUB_CLIENT_ID}"
  "--from-literal=oauth-github-client-secret=${OAUTH_GITHUB_CLIENT_SECRET}"
  "--from-literal=oauth-github-redirect-url=${OAUTH_GITHUB_REDIRECT_URL}"
)

if [ "${DRY_RUN}" = "1" ]; then
  log "DRY_RUN=1 — Secret не создаётся, команды не выполняются"
  exit 0
fi

command -v "${KUBECTL}" >/dev/null || die "не найден ${KUBECTL}"

log "namespace ${NAMESPACE}"
"${KUBECTL}" create namespace "${NAMESPACE}" --dry-run=client -o yaml | "${KUBECTL}" apply -f - >/dev/null

log "применяю Secret (идемпотентно: create --dry-run | apply)"
"${KUBECTL}" -n "${NAMESPACE}" create secret generic "${SECRET_NAME}" "${ARGS[@]}" \
  --dry-run=client -o yaml | "${KUBECTL}" apply -f -

printf '\n'
log "готово. Приложение ставится с уже существующим Secret'ом:"
cat <<EOF

  helm upgrade --install ecommerce deploy/helm/ecommerce-shop \\
    --namespace ${NAMESPACE} \\
    -f deploy/helm/ecommerce-shop/values-dev.yaml \\
    -f deploy/helm/ecommerce-shop/values-ghcr.yaml \\
    --set secrets.create=false \\
    --set secrets.existingSecret=${SECRET_NAME} \\
    --wait --timeout 15m

EOF
