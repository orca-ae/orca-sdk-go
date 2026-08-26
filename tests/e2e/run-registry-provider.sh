#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

e2e::require_env OAUTH_CLIENT_ID
e2e::require_env OAUTH_CLIENT_SECRET
e2e::require_command jq
e2e::require_command kubectl
e2e::require_command go

OAUTH_AUDIENCE=${OAUTH_AUDIENCE:-urn:sn:pulsar:sndev:function-ci}
port_forward_log=${RUNNER_TEMP:-/tmp}/orca-registry-port-forward.log
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

kubectl port-forward service/registry-worker-fw 8080:6750 >"${port_forward_log}" 2>&1 &
port_forward_pid=$!
e2e::wait_for_http http://localhost:8080/health/ready 180

token_response=$(curl --fail --silent --show-error \
  --request POST \
  --url https://auth.sncloud-stg.dev/oauth/token \
  --header 'content-type: application/json' \
  --data "$(jq --compact-output --null-input \
    --arg client_id "${OAUTH_CLIENT_ID}" \
    --arg client_secret "${OAUTH_CLIENT_SECRET}" \
    --arg audience "${OAUTH_AUDIENCE}" \
    '{grant_type:"client_credentials", client_id:$client_id, client_secret:$client_secret, audience:$audience}')")
access_token=$(jq --exit-status --raw-output \
  '.access_token | strings | select(length > 0)' <<<"${token_response}")

export ORCA_BASE_URL=http://localhost:8080
export ORCA_E2E_ACCESS_TOKEN=${access_token}
unset ORCA_E2E_API_KEY
export ORCA_E2E_EXPECT_CLOUD=true
export ORCA_E2E_EXPECT_EXECUTION=true
# The e2e suite lives in the SDK module and is excluded from the default
# build by the `e2e` tag, so `go test ./...` stays offline and green.
(cd "${SCRIPT_DIR}/../.." && go test -tags e2e -timeout 15m -run TestE2E ./...)

echo "SDK against Registry provider topology passed"
