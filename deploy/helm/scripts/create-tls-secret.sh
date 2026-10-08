#!/usr/bin/env bash
# =============================================================================
# create-tls-secret.sh — самоподписанный TLS-сертификат для Ingress
# =============================================================================
# Нужен, когда домена и сертификата от удостоверяющего центра ещё нет, а HTTPS
# через единую точку входа (ingress-nginx) проверить надо.
#
# Что делает:
#   1. генерирует приватный ключ и самоподписанный сертификат с SAN для HOST;
#   2. создаёт/обновляет Secret типа kubernetes.io/tls в namespace.
#
# Использование:
#   HOST=api.ecommerce.local ./create-tls-secret.sh
#   HOST=api.example.com EXTRA_HOSTS="www.example.com" ./create-tls-secret.sh
#
# Переменные:
#   HOST          основной CN/SAN (по умолчанию api.ecommerce.local)
#   EXTRA_HOSTS   дополнительные DNS-имена через запятую
#   EXTRA_IPS     дополнительные IP через запятую (например, IP сервера)
#   NAMESPACE     namespace (ecommerce)
#   SECRET_NAME   имя Secret'а (ecommerce-tls) — должно совпадать с
#                 ingress.tls[].secretName в values
#   DAYS          срок действия (825)
#   OUT_DIR       куда положить ключ и сертификат (deploy/.tools/tls — gitignored)
#   KUBECTL       бинарь kubectl
#
# ВАЖНО: браузер будет ругаться на самоподписанный сертификат — это ожидаемо,
# нужно один раз нажать «Дополнительно → Перейти». Для прода используйте
# cert-manager + Let's Encrypt (см. DEPLOY-RUNBOOK.md, §11).
# =============================================================================
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

KUBECTL="${KUBECTL:-kubectl}"
NAMESPACE="${NAMESPACE:-ecommerce}"
SECRET_NAME="${SECRET_NAME:-ecommerce-tls}"
HOST="${HOST:-api.ecommerce.local}"
EXTRA_HOSTS="${EXTRA_HOSTS:-}"
EXTRA_IPS="${EXTRA_IPS:-}"
DAYS="${DAYS:-825}"
OUT_DIR="${OUT_DIR:-${REPO_ROOT}/deploy/.tools/tls}"
DRY_RUN="${DRY_RUN:-0}"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[x]\033[0m %s\n' "$*" >&2; exit 1; }

command -v openssl >/dev/null || die "не найден openssl"

# --- SAN: основной хост + localhost/127.0.0.1 (удобно для локальной проверки) --
mapfile -t hosts < <(
  printf '%s\n' "${HOST}"
  printf '%s\n' localhost
  if [ -n "${EXTRA_HOSTS}" ]; then
    printf '%s\n' ${EXTRA_HOSTS} | tr ',' '\n' | tr -d ' '
  fi
)
# 127.0.0.1 добавляем всегда, дальше — EXTRA_IPS
mapfile -t ips < <(
  printf '%s\n' 127.0.0.1
  if [ -n "${EXTRA_IPS}" ]; then
    printf '%s\n' ${EXTRA_IPS} | tr ',' '\n' | tr -d ' '
  fi
)

SAN=""
for h in "${hosts[@]}"; do
  [ -n "$h" ] || continue
  SAN="${SAN:+${SAN},}DNS:${h}"
done
for i in "${ips[@]}"; do
  [ -n "$i" ] || continue
  SAN="${SAN:+${SAN},}IP:${i}"
done

mkdir -p "${OUT_DIR}"
KEY="${OUT_DIR}/${HOST}.key.pem"
CRT="${OUT_DIR}/${HOST}.crt.pem"

log "генерирую самоподписанный сертификат для ${HOST}"
log "SAN: ${SAN}"
if [ "${DRY_RUN}" = "1" ]; then
  log "DRY_RUN=1 — файлы и Secret не создаются"
  exit 0
fi

openssl req -x509 -newkey rsa:2048 -nodes -sha256 -days "${DAYS}" \
  -keyout "${KEY}" -out "${CRT}" \
  -subj "/CN=${HOST}" \
  -addext "subjectAltName=${SAN}" \
  -addext "keyUsage=digitalSignature,keyEncipherment" \
  -addext "extendedKeyUsage=serverAuth" \
  -addext "basicConstraints=critical,CA:FALSE" 2>/dev/null

chmod 600 "${KEY}"
log "ключ:       ${KEY}"
log "сертификат: ${CRT}"

command -v "${KUBECTL}" >/dev/null || die "не найден ${KUBECTL}"
log "namespace ${NAMESPACE}"
"${KUBECTL}" create namespace "${NAMESPACE}" --dry-run=client -o yaml | "${KUBECTL}" apply -f - >/dev/null

log "создаю/обновляю Secret типа kubernetes.io/tls: ${SECRET_NAME}"
"${KUBECTL}" -n "${NAMESPACE}" create secret tls "${SECRET_NAME}" \
  --cert "${CRT}" --key "${KEY}" \
  --dry-run=client -o yaml | "${KUBECTL}" apply -f -

printf '\n'
log "готово. Проверить срок и SAN:"
cat <<EOF

  kubectl -n ${NAMESPACE} get secret ${SECRET_NAME} -o jsonpath='{.data.tls\.crt}' \\
    | base64 -d | openssl x509 -noout -subject -dates -ext subjectAltName

EOF
warn "самоподписанный сертификат: браузер покажет предупреждение — это ожидаемо"
