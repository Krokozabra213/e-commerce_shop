#!/usr/bin/env bash
# =============================================================================
# deploy.sh — установка приложения в Kubernetes «одной командой»
# =============================================================================
# Порядок (важен!):
#   1. infra         — Postgres/Redis/Mongo/Kafka/Schema Registry + топики
#   2. observability — otel-collector/tempo/loki/prometheus/grafana (опционально)
#   3. app           — 6 сервисов + миграции goose
#
# Почему инфраструктура отдельным релизом: миграции приложения запускаются
# Helm-хуком и должны видеть живую БД. Внутри одного релиза это невозможно
# (хуки выполняются до/после основных ресурсов, но не «между» ними).
#
# Использование:
#   ./deploy.sh                          # dev-профиль в namespace ecommerce
#   ENV=prod ./deploy.sh                 # prod-профиль
#   NAMESPACE=shop ENV=prod ./deploy.sh
#   WITH_OBSERVABILITY=0 ./deploy.sh     # без observability
#   DRY_RUN=1 ./deploy.sh                # только показать, что будет сделано
#
# Переменные:
#   ENV=dev|prod            профиль values (по умолчанию dev)
#   NAMESPACE=ecommerce
#   RELEASE_INFRA / RELEASE_APP / RELEASE_OBS
#   REGISTRY, PROJECT, TAG  для целей сборки образов (только вывод подсказки)
#   WITH_INFRA=1            ставить infra (для prod обычно 0 — БД внешние)
#   WITH_OBSERVABILITY=1
#   TIMEOUT=15m
#   DRY_RUN=0
# =============================================================================
set -euo pipefail

HELM="${HELM:-helm}"
KUBECTL="${KUBECTL:-kubectl}"

ENV="${ENV:-dev}"
NAMESPACE="${NAMESPACE:-ecommerce}"
RELEASE_INFRA="${RELEASE_INFRA:-ecommerce-infra}"
RELEASE_APP="${RELEASE_APP:-ecommerce}"
RELEASE_OBS="${RELEASE_OBS:-observability}"
TIMEOUT="${TIMEOUT:-15m}"
DRY_RUN="${DRY_RUN:-0}"

# В prod по умолчанию БД внешние -> infra не ставим.
if [ "${ENV}" = "prod" ]; then
  WITH_INFRA="${WITH_INFRA:-0}"
  WITH_OBSERVABILITY="${WITH_OBSERVABILITY:-1}"
else
  WITH_INFRA="${WITH_INFRA:-1}"
  WITH_OBSERVABILITY="${WITH_OBSERVABILITY:-0}"
fi

HELM_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${HELM_DIR}/../.." && pwd)"
CHART_INFRA="${HELM_DIR}/infra"
CHART_APP="${HELM_DIR}/ecommerce-shop"
CHART_OBS="${HELM_DIR}/observability"

JWT_KEY="${JWT_KEY:-${REPO_ROOT}/services/auth-service/certs/jwt-private.pem}"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[x]\033[0m %s\n' "$*" >&2; exit 1; }

run() {
  if [ "${DRY_RUN}" = "1" ]; then
    printf '    %s\n' "$*"
  else
    "$@"
  fi
}

command -v "${HELM}" >/dev/null || die "не найден ${HELM}"
command -v "${KUBECTL}" >/dev/null || die "не найден ${KUBECTL}"

VALUES_INFRA="${CHART_INFRA}/values-${ENV}.yaml"
VALUES_APP="${CHART_APP}/values-${ENV}.yaml"
VALUES_OBS="${CHART_OBS}/values-${ENV}.yaml"

[ -f "${VALUES_APP}" ] || die "нет файла values: ${VALUES_APP}"

# -----------------------------------------------------------------------------
# Приватный ключ JWT нужен только если чарт сам создаёт Secret (dev).
# Значение secrets.create ищем по цепочке values.yaml -> values-<env>.yaml.
# -----------------------------------------------------------------------------
JWT_ARGS=()
APP_SECRETS_CREATE="$(python3 - "${CHART_APP}/values.yaml" "${VALUES_APP}" <<'PY'
import sys, yaml
create = True
for path in sys.argv[1:]:
    try:
        data = yaml.safe_load(open(path)) or {}
    except FileNotFoundError:
        continue
    secrets = data.get('secrets') or {}
    if 'create' in secrets:
        create = bool(secrets['create'])
    if secrets.get('existingSecret'):
        create = False
print('true' if create else 'false')
PY
)"

if [ "${APP_SECRETS_CREATE}" = "true" ]; then
  if [ ! -s "${JWT_KEY}" ]; then
    if [ "${DRY_RUN}" = "1" ]; then
      warn "нет приватного ключа JWT (${JWT_KEY}); DRY_RUN — генерацию пропускаю"
    else
      warn "нет приватного ключа JWT: ${JWT_KEY}"
      warn "генерирую новый RSA-2048 (подходит ТОЛЬКО для dev; ключ игнорируется git'ом)"
      mkdir -p "$(dirname "${JWT_KEY}")"
      openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "${JWT_KEY}"
      chmod 600 "${JWT_KEY}"
      warn "в prod НЕ используйте этот ключ: там ключ должен приходить из Secret'а"
    fi
  fi
  if [ -s "${JWT_KEY}" ]; then
    # Проверяем, что ключ читается.
    openssl pkey -in "${JWT_KEY}" -noout >/dev/null 2>&1 \
      || die "файл ${JWT_KEY} не является корректным приватным ключом"
    JWT_ARGS=(--set-file "secrets.values.jwtPrivateKey=${JWT_KEY}")
  fi
else
  log "secrets.create=false -> ключ JWT берётся из существующего Secret'а"
fi

if [ "${DRY_RUN}" != "1" ]; then
  log "namespace ${NAMESPACE}"
  "${KUBECTL}" create namespace "${NAMESPACE}" --dry-run=client -o yaml | "${KUBECTL}" apply -f -
fi

if [ "${WITH_INFRA}" = "1" ]; then
  log "1/3 infra (release ${RELEASE_INFRA}, values ${VALUES_INFRA##*/})"
  run "${HELM}" upgrade --install "${RELEASE_INFRA}" "${CHART_INFRA}" \
    --namespace "${NAMESPACE}" -f "${VALUES_INFRA}" \
    --wait --timeout "${TIMEOUT}"
else
  log "1/3 infra пропущена (WITH_INFRA=0) — предполагаются внешние БД/брокер"
fi

if [ "${WITH_OBSERVABILITY}" = "1" ]; then
  log "2/3 observability (release ${RELEASE_OBS})"
  run "${HELM}" upgrade --install "${RELEASE_OBS}" "${CHART_OBS}" \
    --namespace "${NAMESPACE}" -f "${VALUES_OBS}" \
    --wait --timeout "${TIMEOUT}"
else
  log "2/3 observability пропущена (WITH_OBSERVABILITY=0)"
fi

log "3/3 app (release ${RELEASE_APP}, values ${VALUES_APP##*/})"
# ${arr[@]+"${arr[@]}"} — безопасная передача пустого массива при set -u
# (bash < 4.4, например macOS /bin/bash 3.2, иначе "unbound variable").
run "${HELM}" upgrade --install "${RELEASE_APP}" "${CHART_APP}" \
  --namespace "${NAMESPACE}" -f "${VALUES_APP}" ${JWT_ARGS[@]+"${JWT_ARGS[@]}"} \
  --wait --timeout "${TIMEOUT}"

echo
if [ "${DRY_RUN}" = "1" ]; then
  log "DRY_RUN: команды выше не выполнялись"
  exit 0
fi

log "состояние"
"${KUBECTL}" get pods,svc,ingress -n "${NAMESPACE}"
echo
log "готово"
