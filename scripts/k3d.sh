#!/usr/bin/env bash
# =============================================================================
# k3d.sh — локальный dev-кластер k3d + helm-чарты «одной командой»
# =============================================================================
# Обычно вызывается через make в корне репозитория:
#   make k3d-up        # создать кластер (если нужно) и выкатить dev-стек
#   make k3d-up-local  # то же, но с локально собранными образами
#   make k3d-load      # пересобрать образы, импортировать, перезапустить
#   make k3d-status    # статус подов/сервисов/ингрессов
#   make k3d-logs SVC=auth-service
#   make k3d-down      # удалить кластер
#   make k3d-reset     # удалить и создать заново
#   make k3d-install   # только поставить бинарь k3d
#
# Переменные:
#   K3D_CLUSTER=ecommerce    имя кластера (kube-контекст k3d-<имя>)
#   K3D_HTTP_PORT=8080       host-порт -> ingress :80
#   K3D_HTTPS_PORT=8443      host-порт -> ingress :443
#   K3D_LOCAL=1              собрать образы локально вместо GHCR
#   K3D_AUTO_INSTALL=1       поставить k3d в ~/.local/bin, если его нет
#   K3D_VERSION=v5.9.0       версия k3d для автоустановки
#   NAMESPACE=ecommerce      namespace для чартов
#   K3D_TIMEOUT=20m          таймаут ожидания helm-релизов
# =============================================================================
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

CLUSTER="${K3D_CLUSTER:-ecommerce}"
CTX="k3d-${CLUSTER}"
NAMESPACE="${NAMESPACE:-ecommerce}"
HTTP_PORT="${K3D_HTTP_PORT:-8080}"
HTTPS_PORT="${K3D_HTTPS_PORT:-8443}"
TIMEOUT="${K3D_TIMEOUT:-20m}"

HELM="${HELM:-helm}"
KUBECTL="${KUBECTL:-kubectl}"

CHART_INFRA="${REPO_ROOT}/deploy/helm/infra"
CHART_APP="${REPO_ROOT}/deploy/helm/ecommerce-shop"
INGRESS_VALUES="${REPO_ROOT}/deploy/cluster/ingress-nginx-values.yaml"
INGRESS_LOCALHOST="${REPO_ROOT}/deploy/cluster/ingress-localhost.yaml"
JWT_KEY="${REPO_ROOT}/services/auth-service/certs/jwt-private.pem"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[x]\033[0m %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "не найден '$1' — установите его и повторите"; }

k()  { "${KUBECTL}" --context "${CTX}" "$@"; }
h()  { "${HELM}" --kube-context "${CTX}" "$@"; }

# -----------------------------------------------------------------------------
# k3d: проверка и (при необходимости) автоустановка
# -----------------------------------------------------------------------------
install_k3d() {
  local version="${K3D_VERSION:-v5.9.0}"
  local install_dir="${K3D_INSTALL_DIR:-${HOME}/.local/bin}"
  local os arch
  command -v curl >/dev/null 2>&1 || die "для автоустановки k3d нужен curl"
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$(uname -m)" in
    x86_64|amd64)  arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "неизвестная архитектура $(uname -m) — поставьте k3d вручную: https://k3d.io" ;;
  esac
  local url="https://github.com/k3d-io/k3d/releases/download/${version}/k3d-${os}-${arch}"

  log "k3d не найден — устанавливаю ${version} в ${install_dir}"
  mkdir -p "${install_dir}"
  curl -fsSL "${url}" -o "${install_dir}/k3d" \
    || die "не удалось скачать ${url}"
  chmod +x "${install_dir}/k3d"
  export PATH="${install_dir}:${PATH}"
  command -v k3d >/dev/null 2>&1 || die "k3d всё ещё не в PATH — добавьте ${install_dir} в PATH"
}

ensure_k3d() {
  if command -v k3d >/dev/null 2>&1; then return 0; fi
  # Бинарь уже мог быть поставлен ранее, но его каталога нет в PATH.
  local install_dir="${K3D_INSTALL_DIR:-${HOME}/.local/bin}"
  if [ -x "${install_dir}/k3d" ]; then
    export PATH="${install_dir}:${PATH}"
    log "k3d найден в ${install_dir} (использую его)"
    return 0
  fi
  if [ "${K3D_AUTO_INSTALL:-1}" = "1" ]; then
    install_k3d
  else
    die "k3d не найден. Установите k3d (https://k3d.io) или запустите: make k3d-install"
  fi
}

cluster_exists() {
  k3d cluster list -o json 2>/dev/null | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin) or []
except Exception:
    sys.exit(1)
sys.exit(0 if '${CLUSTER}' in {c.get('name') for c in data} else 1)
"
}

# -----------------------------------------------------------------------------
# Шаги установки
# -----------------------------------------------------------------------------
create_cluster() {
  if cluster_exists; then
    log "кластер k3d-${CLUSTER} уже существует — переиспользую"
    return 0
  fi
  log "создаю кластер k3d-${CLUSTER} (Traefik отключён, ingress -> localhost:${HTTP_PORT}/${HTTPS_PORT})"
  k3d cluster create "${CLUSTER}" \
    --k3s-arg "--disable=traefik@server:0" \
    -p "${HTTP_PORT}:80@loadbalancer" \
    -p "${HTTPS_PORT}:443@loadbalancer" \
    --wait
}

wait_cluster() {
  log "ожидаю API кластера"
  local i
  for i in $(seq 1 90); do
    if k get --raw='/readyz' >/dev/null 2>&1; then break; fi
    [ "${i}" -eq 90 ] && die "API кластера не поднялся за 180 c"
    sleep 2
  done
  k wait --for=condition=Ready nodes --all --timeout=120s
}

install_ingress() {
  log "ingress-nginx"
  "${HELM}" repo add ingress-nginx https://kubernetes.github.io/ingress-nginx >/dev/null 2>&1 || true
  "${HELM}" repo update >/dev/null 2>&1 || true
  h upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
    -n ingress-nginx --create-namespace \
    -f "${INGRESS_VALUES}" \
    --set controller.config.hsts=false \
    --wait --timeout 10m
}

build_and_import() {
  log "собираю образы локально"
  "${REPO_ROOT}/deploy/helm/scripts/build-images.sh"
  log "импортирую образы в containerd k3d-${CLUSTER}"
  local -a tags
  mapfile -t tags < <("${REPO_ROOT}/deploy/helm/scripts/build-images.sh" --print-tags)
  k3d image import "${tags[@]}" -c "${CLUSTER}"
}

deploy_stack() {
  # JWT-ключ (и .env) — то же, что для docker compose.
  "${REPO_ROOT}/scripts/dev-init.sh"

  log "1/2 инфраструктура (Postgres x4, Redis, MongoDB, Kafka, Schema Registry)"
  h upgrade --install ecommerce-infra "${CHART_INFRA}" \
    -n "${NAMESPACE}" --create-namespace \
    -f "${CHART_INFRA}/values-dev.yaml" \
    --wait --timeout "${TIMEOUT}"

  local -a app_args=(-f "${CHART_APP}/values-dev.yaml")
  if [ "${K3D_LOCAL:-0}" = "1" ]; then
    build_and_import
  else
    # Публичные образы ghcr.io/krokozabra213/<svc>:<appVersion>.
    app_args+=(-f "${CHART_APP}/values-ghcr.yaml")
  fi

  log "2/2 приложение (dev)"
  h upgrade --install ecommerce "${CHART_APP}" \
    -n "${NAMESPACE}" \
    "${app_args[@]}" \
    --set-file "secrets.values.jwtPrivateKey=${JWT_KEY}" \
    --wait --timeout "${TIMEOUT}"

  log "http-Ingress на хост localhost"
  k apply -f "${INGRESS_LOCALHOST}"

  k rollout status deployment/api-gateway -n "${NAMESPACE}" --timeout=120s
}

summary() {
  printf '\n'
  log "поды в namespace ${NAMESPACE}"
  k get pods -n "${NAMESPACE}"

  local health
  health="$(curl -fsS --max-time 5 "http://localhost:${HTTP_PORT}/healthz" 2>/dev/null || true)"
  printf '\n'
  if [ -n "${health}" ]; then
    log "healthz: ${health}"
  else
    warn "healthz пока не отвечает — поды могут ещё стартовать (make k3d-status)"
  fi

  cat <<EOF

  Кластер:        k3d-${CLUSTER} (контекст ${CTX})
  API:            http://localhost:${HTTP_PORT}
  Swagger UI:     http://localhost:${HTTP_PORT}/swagger/index.html
  healthz:        curl -s http://localhost:${HTTP_PORT}/healthz
  Логи сервиса:   make k3d-logs SVC=api-gateway
  Статус:         make k3d-status
  Локальные образы: make k3d-up-local   (собрать из текущего кода)
  Удалить:        make k3d-down

EOF
}

# -----------------------------------------------------------------------------
# Команды
# -----------------------------------------------------------------------------
cmd_up() {
  need docker; need "${KUBECTL}"; need "${HELM}"; need openssl; need curl; need python3
  ensure_k3d
  create_cluster
  wait_cluster
  install_ingress
  deploy_stack
  summary
}

cmd_load() {
  need docker
  ensure_k3d
  cluster_exists || die "кластера k3d-${CLUSTER} нет — сначала make k3d-up"
  build_and_import
  log "переключаю релиз на локальные образы и перезапускаю поды"
  h upgrade --install ecommerce "${CHART_APP}" \
    -n "${NAMESPACE}" \
    -f "${CHART_APP}/values-dev.yaml" \
    --set-file "secrets.values.jwtPrivateKey=${JWT_KEY}" \
    --wait --timeout "${TIMEOUT}"
  k rollout restart deployment -n "${NAMESPACE}"
  k rollout status deployment/api-gateway -n "${NAMESPACE}" --timeout=180s
  log "готово"
}

cmd_down() {
  ensure_k3d
  if cluster_exists; then
    log "удаляю кластер k3d-${CLUSTER}"
    k3d cluster delete "${CLUSTER}"
  else
    log "кластера k3d-${CLUSTER} нет"
  fi
}

cmd_reset() {
  cmd_down
  cmd_up
}

cmd_status() {
  ensure_k3d
  k3d cluster list
  printf '\n'
  k get pods,svc,ingress -n "${NAMESPACE}" 2>&1 || warn "namespace ${NAMESPACE} ещё не создан"
}

cmd_logs() {
  local svc="${1:-}"
  [ -n "${svc}" ] || die "укажите сервис: make k3d-logs SVC=auth-service"
  ensure_k3d
  k logs -n "${NAMESPACE}" -l "app.kubernetes.io/component=${svc}" --tail=200 -f
}

usage() {
  cat <<'EOF'
Локальный dev-кластер k3d для e-commerce_shop.

  make k3d-up          создать кластер (если нужно) и выкатить dev-стек
  make k3d-up-local    то же, но с локально собранными образами
  make k3d-load        пересобрать образы, импортировать, перезапустить
  make k3d-status      статус кластера и подов
  make k3d-logs        логи сервиса: make k3d-logs SVC=auth-service
  make k3d-down        удалить кластер
  make k3d-reset       удалить и создать заново
  make k3d-install     только поставить бинарь k3d

Переменные: K3D_CLUSTER, K3D_HTTP_PORT, K3D_HTTPS_PORT, K3D_LOCAL,
K3D_AUTO_INSTALL, K3D_VERSION, NAMESPACE.
EOF
}

case "${1:-}" in
  up)          cmd_up ;;
  load)        cmd_load ;;
  down)        cmd_down ;;
  reset)       cmd_reset ;;
  status)      cmd_status ;;
  logs)        shift; cmd_logs "$@" ;;
  install-k3d) install_k3d ;;
  ""|-h|--help|help) usage ;;
  *) die "неизвестная команда '$1' (up|load|down|reset|status|logs|install-k3d)" ;;
esac
