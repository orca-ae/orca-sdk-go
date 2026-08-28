#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

e2e::require_env MANAGED_AGENTS_CHART_DIR
e2e::require_env MANAGED_AGENTS_NAMESPACE
e2e::require_env MANAGED_AGENTS_REGISTRY_IMAGE
e2e::require_env MANAGED_AGENTS_HARNESS_IMAGE
e2e::require_env MANAGED_AGENTS_AI_GATEWAY_IMAGE
e2e::require_env MANAGED_AGENTS_FIXTURE_IMAGE
e2e::require_command helm
e2e::require_command kubectl
e2e::require_command openssl

if [[ ! "${MANAGED_AGENTS_NAMESPACE}" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
  echo "invalid Kubernetes namespace: ${MANAGED_AGENTS_NAMESPACE}" >&2
  exit 1
fi
if [[ ! -f "${MANAGED_AGENTS_CHART_DIR}/Chart.yaml" ]]; then
  echo "Managed Agents Helm chart not found: ${MANAGED_AGENTS_CHART_DIR}" >&2
  exit 1
fi

split_tagged_image() {
  local image=$1
  local output_prefix=$2
  local last_segment=${image##*/}
  if [[ "${image}" == *@* || "${last_segment}" != *:* ]]; then
    echo "${output_prefix} image must use an explicit tag: ${image}" >&2
    return 1
  fi
  printf -v "${output_prefix}_repository" '%s' "${image%:*}"
  printf -v "${output_prefix}_tag" '%s' "${image##*:}"
}

split_tagged_image "${MANAGED_AGENTS_REGISTRY_IMAGE}" registry_image
split_tagged_image "${MANAGED_AGENTS_HARNESS_IMAGE}" harness_image
split_tagged_image "${MANAGED_AGENTS_AI_GATEWAY_IMAGE}" gateway_image

if ! kubectl get namespace "${MANAGED_AGENTS_NAMESPACE}" >/dev/null 2>&1; then
  kubectl create namespace "${MANAGED_AGENTS_NAMESPACE}"
fi

if [[ "${MANAGED_AGENTS_INSTALL_INFRA:-false}" == "true" ]]; then
  kubectl --namespace "${MANAGED_AGENTS_NAMESPACE}" delete \
    job/orca-managed-agents-minio-init --ignore-not-found
  sed "s/__NAMESPACE__/${MANAGED_AGENTS_NAMESPACE}/g" \
    "${SCRIPT_DIR}/managed-agents-api-infra.yaml" | kubectl apply -f -
  kubectl --namespace "${MANAGED_AGENTS_NAMESPACE}" wait \
    --for=condition=available --timeout=3m \
    deployment/orca-managed-agents-postgres deployment/orca-managed-agents-minio
  kubectl --namespace "${MANAGED_AGENTS_NAMESPACE}" wait \
    --for=condition=complete --timeout=3m job/orca-managed-agents-minio-init
fi

if [[ ! "${MANAGED_AGENTS_FIXTURE_IMAGE}" =~ ^[A-Za-z0-9._:/-]+$ ]]; then
  echo "invalid deterministic fixture image: ${MANAGED_AGENTS_FIXTURE_IMAGE}" >&2
  exit 1
fi
sed \
  -e "s|__NAMESPACE__|${MANAGED_AGENTS_NAMESPACE}|g" \
  -e "s|__FIXTURE_IMAGE__|${MANAGED_AGENTS_FIXTURE_IMAGE}|g" \
  "${SCRIPT_DIR}/managed-agents-fixture.yaml" | kubectl apply -f -
kubectl --namespace "${MANAGED_AGENTS_NAMESPACE}" rollout status \
  deployment/orca-managed-agents-fixture --timeout=3m

temp_dir=$(mktemp -d)
cleanup() {
  rm -rf "${temp_dir}"
}
trap cleanup EXIT

openssl genrsa -out "${temp_dir}/session-jwt-private.pem" 2048
openssl pkey \
  -in "${temp_dir}/session-jwt-private.pem" \
  -pubout \
  -out "${temp_dir}/session-jwt-public.pem"

image_pull_secret_args=()
if [[ -n "${MANAGED_AGENTS_IMAGE_PULL_SECRET:-}" ]]; then
  image_pull_secret_args+=(
    --set-string "imagePullSecrets[0].name=${MANAGED_AGENTS_IMAGE_PULL_SECRET}"
  )
fi

helm upgrade --install orca-managed-agents "${MANAGED_AGENTS_CHART_DIR}" \
  --namespace "${MANAGED_AGENTS_NAMESPACE}" \
  --values "${SCRIPT_DIR}/managed-agents-values.yaml" \
  --set-string "images.registry.repository=${registry_image_repository}" \
  --set-string "images.registry.tag=${registry_image_tag}" \
  --set-string "images.harness.repository=${harness_image_repository}" \
  --set-string "images.harness.tag=${harness_image_tag}" \
  --set-string "images.aiGateway.repository=${gateway_image_repository}" \
  --set-string "images.aiGateway.tag=${gateway_image_tag}" \
  "${image_pull_secret_args[@]}" \
  --set-file "secrets.values.sessionJwtPrivateKeyPem=${temp_dir}/session-jwt-private.pem" \
  --set-file "secrets.values.sessionJwtPublicKeyPem=${temp_dir}/session-jwt-public.pem" \
  --atomic \
  --wait \
  --timeout 10m \
  --history-max 3

kubectl --namespace "${MANAGED_AGENTS_NAMESPACE}" rollout status \
  deployment/orca-managed-agents-registry \
  deployment/orca-managed-agents-harness \
  deployment/orca-managed-agents-ai-gateway \
  --timeout=5m
