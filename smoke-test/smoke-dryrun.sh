#!/usr/bin/env bash

# Default values for environment variables
K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl-dryrun.yaml"}
K0S_FROM=${K0S_FROM:-"v1.21.6+k0s.0"}
K0S_TO=${K0S_TO:-"$(curl -s "https://docs.k0sproject.io/stable.txt")"}

log="smoke-dryrun.log"

# Source common functions
. ./smoke.common.sh
trap cleanup EXIT

# Define functions
remoteCommand() {
  local userhost="$1"
  shift
  bootloose ssh "${userhost}" -- "$*"
}

colorEcho() {
  local color=$1
  shift
  echo -e "\033[1;3${color}m************************************************************\033[0m"
  echo -e "\033[1;3${color}m$*\033[0m"
  echo -e "\033[1;3${color}m************************************************************\033[0m"
}

checkDryRunLines() {
  local mode=$1
  local expected=$2
  local count
  count=$(grep -c "dry-run" "${log}")
  case "${mode}" in
    min)
      if [ "${count}" -lt "${expected}" ]; then
        colorEcho 1 "Expected at least ${expected} dry-run lines, got ${count}"
        exit 1
      fi
      ;;
    none)
      if [ "${count}" -ne 0 ]; then
        colorEcho 1 "Expected zero dry-run lines, got ${count}"
        exit 1
      fi
      ;;
    *)
      echo "Unknown mode for checkDryRunLines"
      exit 1
      ;;
  esac
}

dryRunNoChanges() {
  if ! grep -q "no cluster state altering actions" "${log}"; then
    colorEcho 1 "Expected dry-run to have no changes"
    exit 1
  fi
}

dumpDryRunLines() {
  colorEcho 2 "Dry-run filtered log:"
  grep "dry-run" "${log}"
}

expectK0sVersion() {
  local expected=$1
  local remote
  remote=$(remoteCommand "root@manager0" "k0s version")
  if [ "${remote}" != "${expected}" ]; then
    colorEcho 1 "Expected k0s version ${expected}, got ${remote}"
    exit 1
  fi
}

expectNoK0s() {
  echo "Expecting no k0s on controller"
  if remoteCommand "root@manager0" "test -d /etc/k0s"; then
    colorEcho 1 "Expected no /etc/k0s on controller"
    exit 1
  fi
  if remoteCommand "root@manager0" "test -f /etc/k0s/k0s.yaml"; then
    colorEcho 1 "Expected no /etc/k0s/k0s.yaml on controller"
    exit 1
  fi
  if remoteCommand "root@manager0" "ps -ef" | grep -q "k0s controller"; then
    colorEcho 1 "Expected no k0s controller process on controller"
    exit 1
  fi
}

applyConfig() {
  local extra_flags=("$@")
    ../k0sctl apply --config "${K0SCTL_CONFIG}" --debug "${extra_flags[@]}" | tee "${log}"
}

deleteCluster
createCluster

K0S_VERSION="${K0S_FROM}"

colorEcho 3 "Installing ${K0S_VERSION} with --dry-run"
applyConfig --dry-run
expectNoK0s
checkDryRunLines min 3
dumpDryRunLines

colorEcho 3 "Installing ${K0S_VERSION}"
applyConfig
expectK0sVersion "${K0S_FROM}"
checkDryRunLines none

colorEcho 3 "Installing ${K0S_VERSION} with --dry-run again"
applyConfig --dry-run
expectK0sVersion "${K0S_FROM}"
dryRunNoChanges

colorEcho 4 "Succesfully installed ${K0S_FROM}, moving on to upgrade to ${K0S_TO}"
K0S_VERSION="${K0S_TO}"

colorEcho 3 "Upgrading to ${K0S_VERSION} with --dry-run"
applyConfig --dry-run --force
expectK0sVersion "${K0S_FROM}"
checkDryRunLines min 3
dumpDryRunLines

colorEcho 3 "Upgrading to ${K0S_VERSION}"
applyConfig --force
expectK0sVersion "${K0S_TO}"
checkDryRunLines none

colorEcho 3 "Upgrading to ${K0S_VERSION} with --dry-run again"
applyConfig --dry-run --force
expectK0sVersion "${K0S_TO}"
dryRunNoChanges
