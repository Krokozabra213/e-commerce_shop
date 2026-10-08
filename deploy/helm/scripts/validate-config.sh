#!/usr/bin/env bash
# =============================================================================
# validate-config.sh — простая проверка: чарт рендерится и даёт валидный YAML
# =============================================================================
# Что делает:
#   для профилей dev и prod рендерит чарт приложения (helm template) и
#   проверяет, что весь вывод — корректный YAML.
#
# Чего НЕ делает (сознательно упрощено):
#   * не проверяет схемы Kubernetes (kubeconform);
#   * не прогоняет конфиги через реальный config.Init() сервисов;
#   * не проверяет чарты infra и observability.
#   Если понадобится вернуть эти проверки — они есть в истории git.
#
# Использование:
#   deploy/helm/scripts/validate-config.sh
#   HELM=/path/to/helm deploy/helm/scripts/validate-config.sh
# =============================================================================
set -euo pipefail

HELM="${HELM:-helm}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
CHART="${REPO_ROOT}/deploy/helm/ecommerce-shop"

for env in dev prod; do
  "${HELM}" template ecommerce "${CHART}" \
    -f "${CHART}/values-${env}.yaml" \
    --set secrets.values.jwtPrivateKey=dummy \
    | python3 -c 'import sys, yaml; list(yaml.safe_load_all(sys.stdin))' \
    && echo "OK app/${env}"
done
