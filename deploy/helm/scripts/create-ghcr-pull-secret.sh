#!/usr/bin/env bash
# =============================================================================
# create-ghcr-pull-secret.sh — pull-секрет для приватных пакетов GHCR
# =============================================================================
# Зачем: после первого push пакеты в GHCR создаются ПРИВАТНЫМИ. Чтобы kubelet
# мог их скачать, в namespace нужен Secret типа kubernetes.io/dockerconfigjson.
# Имя секрета должно совпадать с global.imagePullSecrets в values —
# по умолчанию это `ghcr-pull` (уже прописан в values-ghcr.yaml,
# values-server.yaml и values-prod.yaml, и подставляется в 10 workload'ов:
# 6 Deployment'ов и 4 Job'а миграций).
#
# ВАЖНО про порядок: секрет должен существовать ДО создания подов, иначе они
# уйдут в ImagePullBackOff (kubelet не найдёт указанный imagePullSecret).
#
# Создание PAT:
#   GitHub → Settings → Developer settings → Personal access tokens → Tokens
#   (classic) → Generate new token. Нужен scope `read:packages`.
#   Для приватного репозитория/организации может потребоваться ещё
#   `repo` и авторизация токена для SSO организации.
#
# Использование:
#   CR_PAT=ghp_xxx deploy/helm/scripts/create-ghcr-pull-secret.sh
#   CR_PAT=ghp_xxx NAMESPACE=shop deploy/helm/scripts/create-ghcr-pull-secret.sh
#   CR_PAT=ghp_xxx DRY_RUN=1 deploy/helm/scripts/create-ghcr-pull-secret.sh
#
# Переменные:
#   CR_PAT        (обязательно) personal access token со scope read:packages
#                 (альтернативно GHCR_TOKEN)
#   NAMESPACE     namespace (ecommerce)
#   SECRET_NAME   имя секрета (ghcr-pull)
#   REGISTRY      хост реестра (ghcr.io)
#   GHCR_USER     логин GitHub в НИЖНЕМ регистре (krokozabra213).
#                 ВНИМАНИЕ: не USERNAME — эта переменная уже занята окружением
#   KUBECTL       бинарь kubectl
#   DRY_RUN       1 = ничего не создавать
#
# Безопасность: токен передаётся только в кластер. Он НЕ попадает ни в файлы,
# ни в git, ни в историю Helm (секрет создаётся вручную, а не чартом).
# =============================================================================
set -euo pipefail

KUBECTL="${KUBECTL:-kubectl}"
NAMESPACE="${NAMESPACE:-ecommerce}"
SECRET_NAME="${SECRET_NAME:-ghcr-pull}"
REGISTRY="${REGISTRY:-ghcr.io}"
GHCR_USER="${GHCR_USER:-krokozabra213}"
DRY_RUN="${DRY_RUN:-0}"
# Мягкая предпроверка токена (не влияет на результат, только предупреждает).
PRECHECK="${PRECHECK:-1}"
# Репозиторий для предпроверки — любой из ваших пакетов.
CHECK_REPO="${CHECK_REPO:-api-gateway}"

TOKEN="${CR_PAT:-${GHCR_TOKEN:-}}"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[x]\033[0m %s\n' "$*" >&2; exit 1; }

[ -n "${TOKEN}" ] || die "не задан CR_PAT.
Нужен personal access token со scope read:packages:
  GitHub → Settings → Developer settings → Personal access tokens (classic)
       → Generate new token → отметить read:packages
Затем:
  CR_PAT=ghp_... $0"

command -v curl >/dev/null 2>&1 || PRECHECK=0

# --- предпроверка: принимает ли GHCR этот токен -------------------------------
if [ "${PRECHECK}" = "1" ]; then
  log "проверяю токен в GHCR (репозиторий ${GHCR_USER}/${CHECK_REPO})"
  CODE="$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 \
    -u "${GHCR_USER}:${TOKEN}" \
    "https://${REGISTRY}/token?service=${REGISTRY}&scope=repository:${GHCR_USER}/${CHECK_REPO}:pull" || echo 000)"
  case "${CODE}" in
    200) log "токен принят (HTTP 200)" ;;
    000) warn "не смог связаться с ${REGISTRY} — проверку пропускаю" ;;
    403|401)
      warn "GHCR ответил HTTP ${CODE} — токен не принят ЛИБО у него нет прав на"
      warn "namespace '${GHCR_USER}'. Это ожидаемо, если пакет ещё не запушен."
      warn "Если пакеты уже есть и токен верный — проверьте scope read:packages"
      warn "и (для организации) авторизацию токена для SSO."
      ;;
    *) warn "неожиданный ответ GHCR: HTTP ${CODE}" ;;
  esac
fi

if [ "${DRY_RUN}" = "1" ]; then
  log "DRY_RUN=1 — секрет не создаётся"
  printf '\n  будет выполнено:\n'
  printf '    kubectl create namespace %s --dry-run=client -o yaml | kubectl apply -f -\n' "${NAMESPACE}"
  printf '    kubectl -n %s create secret docker-registry %s \\\n' "${NAMESPACE}" "${SECRET_NAME}"
  printf '      --docker-server=%s --docker-username=%s --docker-password=<CR_PAT> \\\n' "${REGISTRY}" "${GHCR_USER}"
  printf '      --dry-run=client -o yaml | kubectl apply -f -\n'
  exit 0
fi

command -v "${KUBECTL}" >/dev/null || die "не найден ${KUBECTL}"

log "namespace ${NAMESPACE}"
"${KUBECTL}" create namespace "${NAMESPACE}" --dry-run=client -o yaml | "${KUBECTL}" apply -f - >/dev/null

log "создаю/обновляю pull-секрет ${SECRET_NAME} (тип kubernetes.io/dockerconfigjson)"
# --dry-run + apply: идемпотентно, повторный запуск просто обновит токен.
"${KUBECTL}" -n "${NAMESPACE}" create secret docker-registry "${SECRET_NAME}" \
  --docker-server="${REGISTRY}" \
  --docker-username="${GHCR_USER}" \
  --docker-password="${TOKEN}" \
  --dry-run=client -o yaml | "${KUBECTL}" apply -f -

printf '\n'
log "готово. Секрет ${SECRET_NAME} в namespace ${NAMESPACE}"
cat <<EOF

Дальше:
  1) убедиться, что образы запушены в GHCR (workflow deploy.yaml или вручную):
       REGISTRY=${REGISTRY} PUSH=1 deploy/helm/scripts/build-images.sh

  2) выкатить чарт с профилем, где указан этот секрет:
       helm upgrade --install ecommerce deploy/helm/ecommerce-shop -n ${NAMESPACE} \\
         -f deploy/helm/ecommerce-shop/values-dev.yaml \\
         -f deploy/helm/ecommerce-shop/values-ghcr.yaml \\
         --set secrets.create=false --set secrets.existingSecret=ecommerce-shop-secrets

  3) проверить, что поды реально тянут образы из GHCR:
       kubectl -n ${NAMESPACE} get pods
       kubectl -n ${NAMESPACE} describe pod <pod> | grep -A2 'Failed\\|Pulling'

Проверить содержимое секрета (не печатает токен):
  kubectl -n ${NAMESPACE} get secret ${SECRET_NAME} -o jsonpath='{.type}{"\\n"}'
  kubectl -n ${NAMESPACE} get secret ${SECRET_NAME} -o jsonpath='{.data.\\.dockerconfigjson}' \\
    | base64 -d | python3 -c 'import sys,json;d=json.load(sys.stdin);print(list(d["auths"].keys()))'

Если поды не должны указывать секрет явно (например, сторонние чарты),
можно повесить его на ServiceAccount по умолчанию:
  kubectl -n ${NAMESPACE} patch serviceaccount default \\
    -p '{"imagePullSecrets":[{"name":"${SECRET_NAME}"}]}'

Секрет создан вручную, поэтому \`helm uninstall\` его НЕ удаляет — удалять отдельно:
  kubectl -n ${NAMESPACE} delete secret ${SECRET_NAME}

EOF
warn "если токен утёк — отзовите его в GitHub и создайте новый (шаг 1)"
