#!/usr/bin/env bash
# =============================================================================
# validate-config-contract.sh
# =============================================================================
# Проверяет, что конфиги, которые Helm рендерит в ConfigMap'ы, РЕАЛЬНО
# принимаются кодом сервисов: каждый отрендеренный <env>.yaml скармливается
# настоящему пакету services/<name>/internal/config через config.Init().
#
# Это ловит:
#   * опечатки в yaml-ключах (cleanenv их молча игнорирует -> поле остаётся
#     нулевым -> падает env-required);
#   * отсутствующие обязательные поля;
#   * неверные имена переменных окружения (env-prefix + env-тег);
#   * ошибки типов/длительностей.
#
# Использование (из корня репозитория):
#   deploy/helm/scripts/validate-config-contract.sh [values-file]
#
# По умолчанию values-file = deploy/helm/ecommerce-shop/values-dev.yaml
# =============================================================================
set -euo pipefail

HELM_BIN="${HELM_BIN:-/snap/helm/545/helm}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
CHART_DIR="${REPO_ROOT}/deploy/helm/ecommerce-shop"
VALUES_FILE="${1:-${CHART_DIR}/values-dev.yaml}"
WORK_DIR="${REPO_ROOT}/deploy/.validate-config-contract"

SERVICES=(api-gateway auth-service user-service product-service inventory-service order-service)

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
ok()   { printf '\033[1;32m  OK\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m  FAIL\033[0m %s\n' "$*"; }

cleanup() {
  rm -rf "${WORK_DIR}"
  for svc in "${SERVICES[@]}"; do
    rm -f "${REPO_ROOT}/services/${svc}/internal/config/zz_render_check_test.go"
  done
}
trap cleanup EXIT

rm -rf "${WORK_DIR}"
mkdir -p "${WORK_DIR}"

log "рендерю чарт (values: ${VALUES_FILE#"${REPO_ROOT}/"})"
RENDERED="${WORK_DIR}/rendered.yaml"
if [ -f "${REPO_ROOT}/services/auth-service/certs/jwt-private.pem" ]; then
  "${HELM_BIN}" template ecommerce "${CHART_DIR}" -f "${VALUES_FILE}" \
    --set-file secrets.values.jwtPrivateKey="${REPO_ROOT}/services/auth-service/certs/jwt-private.pem" \
    > "${RENDERED}"
else
  "${HELM_BIN}" template ecommerce "${CHART_DIR}" -f "${VALUES_FILE}" \
    --set secrets.create=false --set secrets.existingSecret=dummy \
    > "${RENDERED}"
fi

log "разбираю ConfigMap'ы"
python3 - "${RENDERED}" "${WORK_DIR}" <<'PY'
import sys, yaml, os
rendered, workdir = sys.argv[1], sys.argv[2]
env_name, payloads = None, {}
for doc in yaml.safe_load_all(open(rendered)):
    if not doc or doc.get('kind') != 'ConfigMap':
        continue
    name = doc['metadata']['name']
    for key, value in (doc.get('data') or {}).items():
        env_name = key
        payloads[name.replace('-config', '')] = value
if not payloads:
    sys.exit('не найдено ни одного ConfigMap')
for svc, content in payloads.items():
    d = os.path.join(workdir, svc, 'configs')
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, env_name), 'w') as f:
        f.write(content)
# ENV для config.Init() — имя ключа БЕЗ расширения .yaml
# (Init() сам добавляет "configs/%s.yaml").
with open(os.path.join(workdir, 'env'), 'w') as f:
    f.write(env_name.rsplit('.yaml', 1)[0])
print(f'  извлечено конфигов: {len(payloads)} (файл {env_name})')
PY

ENV_FILE="$(cat "${WORK_DIR}/env")"
# Переменные окружения, которые в кластере приходят из Secret'а.
common_env=("ENV=${ENV_FILE}" "APP_ENV=${ENV_FILE}" APP_SECRET=contract-test-secret)
declare -A svc_env=(
  [api-gateway]="REDIS_PASSWORD=redis"
  [auth-service]="POSTGRES_PASSWORD=postgres REDIS_PASSWORD=redis OAUTH_GOOGLE_CLIENT_ID=gid OAUTH_GOOGLE_CLIENT_SECRET=gsecret OAUTH_GOOGLE_REDIRECT_URL=http://localhost/cb OAUTH_GITHUB_CLIENT_ID=ghid OAUTH_GITHUB_CLIENT_SECRET=ghsecret OAUTH_GITHUB_REDIRECT_URL=http://localhost/cb"
  [user-service]="POSTGRES_PASSWORD=postgres"
  [inventory-service]="POSTGRES_PASSWORD=postgres"
  [order-service]="POSTGRES_PASSWORD=postgres"
  [product-service]="MONGO_URI=mongodb://admin:admin123@mongodb:27017"
)

rc=0
for svc in "${SERVICES[@]}"; do
  log "проверяю ${svc}"
  pkg_dir="${REPO_ROOT}/services/${svc}/internal/config"
  if [ ! -d "${pkg_dir}" ]; then
    fail "нет каталога ${pkg_dir}"
    rc=1
    continue
  fi
  cat > "${pkg_dir}/zz_render_check_test.go" <<'GO'
package config

import (
	"os"
	"testing"
)

// TestRenderedConfigLoads проверяет, что конфиг, отрендеренный Helm'ом,
// принимается cleanenv (все env-required поля заполнены, типы корректны).
func TestRenderedConfigLoads(t *testing.T) {
	if os.Getenv("ENV") == "" {
		t.Skip("ENV не задан")
	}
	if _, err := Init(); err != nil {
		t.Fatalf("Init() отрендеренного конфига: %v", err)
	}
}
GO

  bin="${WORK_DIR}/${svc}/configcheck"
  if ! (cd "${REPO_ROOT}/services/${svc}" && go test -c -o "${bin}" ./internal/config/ 2>"${WORK_DIR}/${svc}/build.log"); then
    fail "не собрался тест: $(tail -5 "${WORK_DIR}/${svc}/build.log")"
    rc=1
    continue
  fi

  if (cd "${WORK_DIR}/${svc}" && env "${common_env[@]}" ${svc_env[$svc]} "${bin}" -test.run TestRenderedConfigLoads -test.v \
        > "${WORK_DIR}/${svc}/run.log" 2>&1); then
    ok "config.Init() принял отрендеренный конфиг"
  else
    fail "config.Init() отверг конфиг:"
    sed 's/^/      /' "${WORK_DIR}/${svc}/run.log" | tail -20
    rc=1
  fi
done

echo
if [ "${rc}" -eq 0 ]; then
  printf '\033[1;32mВСЕ КОНФИГИ ВАЛИДНЫ\033[0m\n'
else
  printf '\033[1;31mЕСТЬ ОШИБКИ\033[0m\n'
fi
exit "${rc}"
