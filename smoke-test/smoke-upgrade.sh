#!/usr/bin/env bash

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}
K0S_TO=${K0S_TO:-"$(curl -s "https://docs.k0sproject.io/stable.txt")"}

set -e

. ./smoke.common.sh
trap cleanup EXIT

remoteCommand() {
  local userhost="$1"
  shift
  echo "* Running command on ${userhost}: $*"
  bootloose ssh "${userhost}" -- "$*"
}

applyVersion() {
  K0S_VERSION=$1
  shift
  echo "Applying k0s ${K0S_VERSION}"
  ../k0sctl apply --config "${K0SCTL_CONFIG}" --debug "$@"
  remoteCommand "root@manager0" "k0s version | grep -q ${K0S_VERSION}"
}

K0S_PREV=$(k0sVersionBefore "${K0S_TO}" 1)
K0S_FROM=$(k0sVersionBefore "${K0S_TO}" 2)
echo "Upgrade path: ${K0S_FROM} -> ${K0S_PREV} -> ${K0S_TO}"

deleteCluster
createCluster

applyVersion "${K0S_FROM}"

# Going straight to the target skips a minor version, so the version skew check must block it before anything changes
K0S_VERSION="${K0S_TO}"
echo "Upgrading to k0s ${K0S_VERSION}, expecting the version skew check to fail the apply"
if ../k0sctl apply --config "${K0SCTL_CONFIG}" --debug > smoke-upgrade-skew.log 2>&1; then
  cat smoke-upgrade-skew.log
  echo "Expected the version skew check to fail the apply"
  exit 1
fi
cat smoke-upgrade-skew.log
grep -q "one minor version at a time" smoke-upgrade-skew.log
remoteCommand "root@manager0" "k0s version | grep -q ${K0S_FROM}"

# Upgrade one minor version at a time
applyVersion "${K0S_PREV}"
applyVersion "${K0S_TO}"
