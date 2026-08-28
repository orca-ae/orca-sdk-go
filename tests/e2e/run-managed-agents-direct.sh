#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

e2e::require_env KIND_HELM_NAMESPACE
e2e::require_command kubectl
e2e::require_command go

port_forward_log=${RUNNER_TEMP:-/tmp}/orca-managed-agents-port-forward.log
port_forward_pid=

cleanup() {
  local status=$?
  trap - EXIT
  if [[ -n "${port_forward_pid}" ]] && kill -0 "${port_forward_pid}" >/dev/null 2>&1; then
    kill "${port_forward_pid}" >/dev/null 2>&1 || true
    wait "${port_forward_pid}" >/dev/null 2>&1 || true
  fi
  if ((status != 0)) && [[ -f "${port_forward_log}" ]]; then
    cat "${port_forward_log}"
  fi
  exit "${status}"
}
trap cleanup EXIT

kubectl --namespace "${KIND_HELM_NAMESPACE}" port-forward \
  service/orca-managed-agents-registry 18080:8080 >"${port_forward_log}" 2>&1 &
port_forward_pid=$!
e2e::wait_for_http http://localhost:18080/healthz 180

api_key=orca_sdk_e2e_local_key
"${SCRIPT_DIR}/seed-managed-agents-api-key.sh" \
  "${KIND_HELM_NAMESPACE}" orca-managed-agents-registry "${api_key}"

export ORCA_BASE_URL=http://localhost:18080
export ORCA_E2E_API_KEY=${api_key}
unset ORCA_E2E_ACCESS_TOKEN
export ORCA_E2E_EXPECT_CLOUD=false
export ORCA_E2E_EXPECT_EXECUTION=true
# The e2e suite lives in the SDK module and is excluded from the default
# build by the `e2e` tag, so `go test ./...` stays offline and green.
(cd "${SCRIPT_DIR}/../.." && go test -tags e2e -timeout 15m -run TestE2E ./...)

echo "SDK against Managed Agents direct topology passed"
