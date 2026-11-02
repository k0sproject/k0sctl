#!/usr/bin/env bash

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e

. ./smoke.common.sh
trap cleanup EXIT


deleteCluster
createCluster

remoteCommand() {
  local userhost="$1"
  shift
  echo "* Running command on ${userhost}: $*"
  bootloose ssh "${userhost}" -- "$*"
}

# Create config with older version and apply
K0S_VERSION="${K0S_FROM}"
echo "Installing ${K0S_VERSION}"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug
remoteCommand "root@manager0" "k0s version | grep -q ${K0S_FROM}"

K0S_VERSION=$(curl -s "https://docs.k0sproject.io/stable.txt")

# The upgrade skips minor versions, so the version skew check must block it before anything changes
echo "Upgrading to k0s ${K0S_VERSION} without --disable-version-skew-check, expecting a failure"
if ../k0sctl apply --config "${K0SCTL_CONFIG}" --debug > smoke-upgrade-skew.log 2>&1; then
  cat smoke-upgrade-skew.log
  echo "Expected the version skew check to fail the apply"
  exit 1
fi
cat smoke-upgrade-skew.log
grep -q "one minor version at a time" smoke-upgrade-skew.log
remoteCommand "root@manager0" "k0s version | grep -q ${K0S_FROM}"

# Create config with latest version and apply as upgrade
echo "Upgrading to k0s ${K0S_VERSION}"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug --disable-version-skew-check
remoteCommand "root@manager0" "k0s version | grep -q ${K0S_VERSION}"
