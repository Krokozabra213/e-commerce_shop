#!/usr/bin/env bash
# =============================================================================
# build-images.sh — сборка образов всех сервисов и их миграций
# =============================================================================
# Собирает:
#   <REGISTRY>/<PROJECT>/<service>:<TAG>            — сервис
#   <REGISTRY>/<PROJECT>/<service>-migrate:<TAG>    — goose-миграции
#
# Эти имена должны совпадать с image.repository в
# deploy/helm/ecommerce-shop/values.yaml.
#
# Использование:
#   ./build-images.sh                         # только сборка
#   PUSH=1 ./build-images.sh                  # сборка + push
#   REGISTRY=ghcr.io PROJECT=myorg TAG=1.2.3 PUSH=1 ./build-images.sh
#   ./build-images.sh --print-tags            # напечатать имена образов (для kind)
#
# Переменные:
#   REGISTRY  хост реестра (без пути). Пусто = локальные образы без реестра.
#   PROJECT   путь/проект в реестре. По умолчанию "ecommerce".
#   TAG       тег. По умолчанию "1.0.0".
#   PUSH      1 = docker push после сборки.
#   PLATFORM  например linux/amd64.
# =============================================================================
set -euo pipefail

REGISTRY="${REGISTRY:-}"
PROJECT="${PROJECT:-ecommerce}"
TAG="${TAG:-1.0.0}"
PUSH="${PUSH:-0}"
PLATFORM="${PLATFORM:-}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

# Сервис с Dockerfile. Миграции собираются только для сервисов с БД.
SERVICES=(
  auth-service
  user-service
  product-service
  inventory-service
  order-service
  api-gateway
)
# Сервисы, у которых есть Migrate.Dockerfile (см. services/*/migrations).
MIGRATION_SERVICES=(
  auth-service
  user-service
  inventory-service
  order-service
)

image_name() {
  local name="$1"
  if [ -n "${REGISTRY}" ]; then
    printf '%s/%s/%s:%s' "${REGISTRY%/}" "${PROJECT}" "${name}" "${TAG}"
  else
    printf '%s/%s:%s' "${PROJECT}" "${name}" "${TAG}"
  fi
}

all_tags() {
  local svc
  for svc in "${SERVICES[@]}"; do image_name "${svc}"; done
  for svc in "${MIGRATION_SERVICES[@]}"; do image_name "${svc}-migrate"; done
}

if [ "${1:-}" = "--print-tags" ]; then
  all_tags
  exit 0
fi

log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }

build() {
  local dockerfile="$1" image="$2"
  log "собираю ${image}"
  local args=(build -f "${dockerfile}" -t "${image}")
  [ -n "${PLATFORM}" ] && args+=(--platform "${PLATFORM}")
  docker "${args[@]}" "${REPO_ROOT}"
  if [ "${PUSH}" = "1" ]; then
    log "пушу ${image}"
    docker push "${image}"
  fi
}

cd "${REPO_ROOT}"

for svc in "${SERVICES[@]}"; do
  build "services/${svc}/Dockerfile" "$(image_name "${svc}")"
done

for svc in "${MIGRATION_SERVICES[@]}"; do
  build "services/${svc}/Migrate.Dockerfile" "$(image_name "${svc}-migrate")"
done

log "готово"
echo
echo "Образы:"
all_tags | sed 's/^/  /'
