#!/usr/bin/env bash
# =============================================================================
# validate-render.sh — «безкластерная» проверка всех чартов
# =============================================================================
# Для каждого чарта и каждого профиля values:
#   1. helm lint
#   2. helm template
#   3. проверка, что весь вывод — валидный YAML
#   4. проверка, что не выставлен metadata.namespace (namespace задаёт --namespace)
#   5. проверка отключения всех компонентов (все enabled=false -> пустой вывод)
#   6. (если установлен) kubeconform по схемам Kubernetes
#
# Использование (из корня репозитория):
#   deploy/helm/scripts/validate-render.sh
# =============================================================================
set -uo pipefail

HELM="${HELM:-/snap/helm/545/helm}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
HELM_DIR="${REPO_ROOT}/deploy/helm"
# ВНИМАНИЕ: результат рендера пишется в git-ignored каталог build/.
# Чтобы НАСТОЯЩИЙ приватный ключ JWT не попадал в файлы на диске, для
# валидации подставляется заведомо фиктивное значение: манифестам и загрузке
# конфигов важен только факт непустого значения.
OUT_DIR="${HELM_DIR}/build/validate"

command -v "${HELM}" >/dev/null || { echo "нет helm: ${HELM}"; exit 1; }

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
ok()   { printf '\033[1;32m  OK\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m  FAIL\033[0m %s\n' "$*"; }

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"
rc=0

# Фиктивный ключ: реальный PEM здесь не нужен и не должен попадать в build/.
jwt_args=(--set "secrets.values.jwtPrivateKey=VALIDATION-ONLY-NOT-A-REAL-KEY")

check_yaml() { # $1 = файл, $2 = описание
  python3 - "$1" "$2" <<'PY'
import sys, yaml
path, desc = sys.argv[1], sys.argv[2]
problems = []
count = 0
try:
    for doc in yaml.safe_load_all(open(path)):
        if not doc:
            continue
        count += 1
        if doc.get('kind') in (None, 'List'):
            continue
        md = doc.get('metadata') or {}
        if 'namespace' in md:
            problems.append(f"{md.get('name')}: выставлен metadata.namespace")
except yaml.YAMLError as e:
    problems.append(f'YAML не парсится: {e}')
if problems:
    print(f'  FAIL {desc}: документов={count}')
    for p in problems:
        print(f'       - {p}')
    sys.exit(1)
print(f'  OK {desc}: документов={count}, namespace не захардкожен')
PY
}

render() { # $1 chart path, $2 release, $3 values file ("" = skip), $4 out file, $5 описание, $6.. доп --set
  local chart="$1" release="$2" values="$3" out="$4" desc="$5"; shift 5
  local args=(template "${release}" "${chart}" "$@")
  [ -n "${values}" ] && args+=(-f "${values}")
  if [ "${chart}" = "${HELM_DIR}/ecommerce-shop" ]; then
    args+=("${jwt_args[@]}")
  fi
  if ! "${HELM}" "${args[@]}" > "${out}" 2>"${out}.err"; then
    fail "helm template: ${desc}"
    sed 's/^/       /' "${out}.err" | head -20
    rc=1
    return 1
  fi
  check_yaml "${out}" "${desc}" || rc=1
}

# -----------------------------------------------------------------------------
log "helm lint"
for chart in infra ecommerce-shop observability; do
  args=(lint "${HELM_DIR}/${chart}")
  [ "${chart}" = "ecommerce-shop" ] && args+=("${jwt_args[@]}")
  if "${HELM}" "${args[@]}" >"${OUT_DIR}/lint-${chart}.log" 2>&1; then
    ok "lint ${chart}"
  else
    fail "lint ${chart}"
    sed 's/^/       /' "${OUT_DIR}/lint-${chart}.log" | tail -20
    rc=1
  fi
done

# -----------------------------------------------------------------------------
log "helm template (все профили)"
for env in dev prod; do
  render "${HELM_DIR}/infra" ecommerce-infra "${HELM_DIR}/infra/values-${env}.yaml" \
    "${OUT_DIR}/infra-${env}.yaml" "infra/${env}"
  render "${HELM_DIR}/ecommerce-shop" ecommerce "${HELM_DIR}/ecommerce-shop/values-${env}.yaml" \
    "${OUT_DIR}/app-${env}.yaml" "ecommerce-shop/${env}"
  render "${HELM_DIR}/observability" observability "${HELM_DIR}/observability/values-${env}.yaml" \
    "${OUT_DIR}/observability-${env}.yaml" "observability/${env}"
done

# -----------------------------------------------------------------------------
log "helm template (базовые values.yaml без -f)"
render "${HELM_DIR}/infra" ecommerce-infra "" "${OUT_DIR}/infra-base.yaml" "infra/base"
render "${HELM_DIR}/observability" observability "" "${OUT_DIR}/observability-base.yaml" "observability/base"

# -----------------------------------------------------------------------------
log "проверка выключения компонентов"
expect_empty() { # $1 = описание, далее — аргументы helm
  local desc="$1"; shift
  local out="${OUT_DIR}/off-$(echo "${desc}" | tr ' /' '__').yaml"
  if ! "${HELM}" template "$@" > "${out}" 2>"${out}.err"; then
    fail "${desc}: helm template упал"
    sed 's/^/       /' "${out}.err" | head -20
    rc=1
    return
  fi
  if grep -q '^kind:' "${out}"; then
    fail "${desc}: при enabled=false что-то рендерится ($(grep -c '^kind:' "${out}") объектов)"
    rc=1
  else
    ok "${desc}: пустой рендер"
  fi
}

expect_empty "infra: все компоненты off" ecommerce-infra "${HELM_DIR}/infra" \
  --set postgresql.enabled=false --set redis.enabled=false --set mongodb.enabled=false \
  --set kafka.enabled=false --set schemaRegistry.enabled=false

all_off_app=()
for svc in api-gateway auth-service user-service inventory-service product-service order-service; do
  all_off_app+=(--set "services.${svc}.enabled=false")
done
all_off_app+=(--set migrations.enabled=false --set ingress.enabled=false --set networkPolicy.enabled=false)
expect_empty "ecommerce-shop: все сервисы off" ecommerce "${HELM_DIR}/ecommerce-shop" "${all_off_app[@]}"

expect_empty "observability: все компоненты off" observability "${HELM_DIR}/observability" \
  --set grafana.enabled=false --set tempo.enabled=false --set loki.enabled=false \
  --set prometheus.enabled=false --set otel-collector.enabled=false

# -----------------------------------------------------------------------------
log "проверка отключения отдельных сервисов"
for svc in api-gateway auth-service user-service product-service inventory-service order-service; do
  if "${HELM}" template ecommerce "${HELM_DIR}/ecommerce-shop" \
      --set "services.${svc}.enabled=false" "${jwt_args[@]}" >/dev/null 2>"${OUT_DIR}/svc-${svc}.err"; then
    ok "services.${svc}.enabled=false рендерится"
  else
    fail "services.${svc}.enabled=false"
    sed 's/^/       /' "${OUT_DIR}/svc-${svc}.err" | head -10
    rc=1
  fi
done

# -----------------------------------------------------------------------------
log "проверка HPA/NetworkPolicy/миграций (prod)"
if "${HELM}" template ecommerce "${HELM_DIR}/ecommerce-shop" \
    -f "${HELM_DIR}/ecommerce-shop/values-prod.yaml" >/dev/null 2>"${OUT_DIR}/prod.err"; then
  ok "prod-профиль с HPA+NetworkPolicy+миграциями"
else
  fail "prod-профиль"
  sed 's/^/       /' "${OUT_DIR}/prod.err" | head -20
  rc=1
fi

# -----------------------------------------------------------------------------
if command -v kubeconform >/dev/null 2>&1; then
  log "kubeconform"
  if kubeconform -strict -summary -ignore-missing-schemas "${OUT_DIR}"/*.yaml \
      >"${OUT_DIR}/kubeconform.log" 2>&1; then
    ok "kubeconform"
  else
    fail "kubeconform"
    sed 's/^/       /' "${OUT_DIR}/kubeconform.log" | tail -20
    rc=1
  fi
else
  log "kubeconform не установлен — пропускаю (установите: go install github.com/yannh/kubeconform/cmd/kubeconform@latest)"
fi

echo
if [ "${rc}" -eq 0 ]; then
  printf '\033[1;32mВСЁ ВАЛИДНО\033[0m  (результаты рендера: %s)\n' "${OUT_DIR#"${REPO_ROOT}/"}"
else
  printf '\033[1;31mЕСТЬ ОШИБКИ\033[0m\n'
fi
exit "${rc}"
