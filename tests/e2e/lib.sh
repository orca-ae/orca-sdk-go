#!/usr/bin/env bash
# Copyright The Orca Authors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

e2e::require_command() {
  local command_name=$1
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "required command is not installed: ${command_name}" >&2
    return 1
  fi
}

e2e::require_env() {
  local variable_name=$1
  if [[ -z "${!variable_name:-}" ]]; then
    echo "required environment variable is not set: ${variable_name}" >&2
    return 1
  fi
}

e2e::wait_for_http() {
  local url=$1
  local timeout_seconds=${2:-300}
  local deadline=$((SECONDS + timeout_seconds))

  until curl --fail --silent --show-error --connect-timeout 2 --max-time 5 "${url}" >/dev/null; do
    if ((SECONDS >= deadline)); then
      echo "timed out waiting for ${url}" >&2
      return 1
    fi
    sleep 2
  done
}
