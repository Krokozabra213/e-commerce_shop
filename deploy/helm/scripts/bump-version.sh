#!/usr/bin/env bash
# =============================================================================
# bump-version.sh — обновить версию приложения (единый источник)
# =============================================================================
# Меняет appVersion чарта приложения в deploy/helm/ecommerce-shop/Chart.yaml;
# из него берётся тег образов в манифестах.
# =============================================================================
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
CHART_FILE="${REPO_ROOT}/deploy/helm/ecommerce-shop/Chart.yaml"

NEW_VERSION="${1:-}"
CHART_VERSION=""
if [ "${2:-}" = "--chart-version" ]; then
  CHART_VERSION="${3:-}"
fi

die() { printf '\033[1;31m[x]\033[0m %s\n' "$*" >&2; exit 1; }
log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }

[ -n "${NEW_VERSION}" ] || die "укажите версию, например: $0 v1.2.3"
[ -f "${CHART_FILE}" ] || die "не найден ${CHART_FILE}"

case "${NEW_VERSION}" in
  v[0-9]*) : ;;
  *) die "версия должна начинаться с 'v' и цифры (как git-тег), например v1.2.3" ;;
esac

OLD_VERSION="$(sed -nE 's/^appVersion:[[:space:]]*"?([^"[:space:]]+)"?[[:space:]]*$/\1/p' "${CHART_FILE}" | head -1)"

log "appVersion: ${OLD_VERSION:-<пусто>} -> ${NEW_VERSION}"
sed -i -E "s|^appVersion:.*$|appVersion: \"${NEW_VERSION}\"|" "${CHART_FILE}"

if [ -n "${CHART_VERSION}" ]; then
  OLD_CHART="$(sed -nE 's/^version:[[:space:]]*"?([^"[:space:]]+)"?[[:space:]]*$/\1/p' "${CHART_FILE}" | head -1)"
  log "version чарта: ${OLD_CHART:-<пусто>} -> ${CHART_VERSION}"
  sed -i -E "s|^version:.*$|version: ${CHART_VERSION}|" "${CHART_FILE}"
fi

printf '\n'
log "что получилось:"
sed -nE '/^version:|^appVersion:/p' "${CHART_FILE}" | sed 's/^/  /'

printf '\n'
log "проверка рендера (тег должен быть ${NEW_VERSION}):"
"${REPO_ROOT}/deploy/helm/scripts/validate-config.sh" >/dev/null 2>&1 \
  && echo "  validate-config.sh: OK" \
  || echo "  validate-config.sh: нужен helm в PATH (HELM=/путь/к/helm)"

printf '\n'
log "дальше:"
cat <<EOF

  git commit -am "release: ${NEW_VERSION}"
  git tag ${NEW_VERSION}
  git push && git push --tags

EOF
