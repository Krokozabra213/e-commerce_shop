#!/usr/bin/env bash
# =============================================================================
# validate-config.sh — проверка: чарт рендерится и даёт валидный YAML
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
