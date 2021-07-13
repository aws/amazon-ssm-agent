#!/usr/bin/env bash

set -e

declare -r PERSISTENT_STORAGE_BASE_DIR="/.ssm/containers/current"
declare -r USER_DATA="${PERSISTENT_STORAGE_BASE_DIR}/user-data"
declare -r SSM_AGENT_LOCAL_STATE_DIR="/var/lib/amazon/ssm"

log() {
  echo "$*" >&2
}

enable_hybrid_env_ssm() {
  # SSM parameters necessary to register with a hybrid activation
  local activation_code
  local activation_id
  local region

  activation_code=$(fetch_from_json '.["ActivationCode"]' "${USER_DATA}")
  activation_id=$(fetch_from_json '.["ActivationId"]' "${USER_DATA}")
  region=$(fetch_from_json '.["Region"]' "${USER_DATA}")

  # Register with AWS Systems Manager (SSM)
  if ! amazon-ssm-agent -register -code "${activation_code}" -id "${activation_id}" -region "${region}"; then

    # Print errors from ssm agent error log
    cat "/var/log/amazon/ssm/errors.log" >&2

    log "Failed to register with AWS Systems Manager (SSM)"
    exit 1
  fi
}

# Fetch the values from json, and exit on failure (if any)
fetch_from_json() {
  local key="${1:?}"
  local file="${2:?}"
  local value
  if ! value=$(jq -e -r "${key}" "${file}"); then
    log "Unable to parse ${key} from ${file}"
    return 1
  fi
  if [[ -z "${value}" ]]; then
    log "No value set for ${key} in ${file}"
    return 1
  fi
  echo "${value}"
}

if [[ -s "${USER_DATA}" ]] \
&& [[ ! -s "${SSM_AGENT_LOCAL_STATE_DIR}/registration" ]] \
&& jq --exit-status '.["ActivationId"]' "${USER_DATA}" &>/dev/null ; then
  enable_hybrid_env_ssm
fi

if [[ -s "${SSM_AGENT_LOCAL_STATE_DIR}/registration" ]] ; then
  echo "SSM Agent instance id is $(jq -e -r '.["ManagedInstanceID"]' ${SSM_AGENT_LOCAL_STATE_DIR}/registration)"
fi

# Start an SSM Agent process in the foreground
exec /usr/bin/amazon-ssm-agent
