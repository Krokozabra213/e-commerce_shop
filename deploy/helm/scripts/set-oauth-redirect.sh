#!/usr/bin/env bash
# =============================================================================
# set-oauth-redirect.sh — переключить OAuth-redirect между localhost и доменом
# =============================================================================
# Меняет адрес callback'а в Secret'е и перезапускает auth-service, чтобы новые
# значения подхватились.
#
# Переменные: NAMESPACE, SECRET_NAME, KUBECTL — как в create-app-secret.sh.
# =============================================================================
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
KUBECTL="${KUBECTL:-kubectl}"
NAMESPACE="${NAMESPACE:-ecommerce}"

BASE_URL="${1:-}"
RESTART=1
[ "${2:-}" = "--no-restart" ] && RESTART=0

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[x]\033[0m %s\n' "$*" >&2; exit 1; }

[ -n "${BASE_URL}" ] || die "укажите базовый URL, например:
  $0 http://localhost:8080
  $0 https://api.вашдомен.ru"

case "${BASE_URL}" in
  http://localhost*|http://127.0.0.1*|https://*) : ;;
  *) warn "нестандартный BASE_URL: ${BASE_URL}
   Google принимает http ТОЛЬКО для localhost; для остальных хостов нужен https" ;;
esac

log "переключаю OAuth-redirect на ${BASE_URL%/}"
REDIRECT_BASE="${BASE_URL%/}" \
  "${REPO_ROOT}/deploy/helm/scripts/create-app-secret.sh"

if [ "${RESTART}" = "1" ]; then
  command -v "${KUBECTL}" >/dev/null || die "не найден ${KUBECTL}"
  # Значения приходят через secretKeyRef, поэтому Secret не триггерит rollout:
  # поды пересоздаются вручную.
  log "перезапускаю auth-service"
  "${KUBECTL}" -n "${NAMESPACE}" rollout restart deploy/auth-service
  "${KUBECTL}" -n "${NAMESPACE}" rollout status deploy/auth-service --timeout=120s
fi

printf '\n'
log "Зарегистрируйте эти адреса у провайдеров (иначе будет redirect_uri_mismatch):"
cat <<EOF

  Google Cloud Console -> APIs & Services -> Credentials -> ваш OAuth client
    -> Authorized redirect URIs:
       ${BASE_URL%/}/api/v1/auth/oauth/google/callback
    (Authorized JavaScript origins НЕ нужен — он для чисто браузерных приложений)

  GitHub -> Settings -> Developer settings -> OAuth Apps -> Authorization callback URL:
       ${BASE_URL%/}/api/v1/auth/oauth/github/callback

EOF
