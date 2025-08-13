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

# Create config with latest version and apply as upgrade
echo "Upgrading to k0s ${K0S_VERSION}"
# First attempt should fail without --force because of version skew (tee stdout/stderr to log file)
if ../k0sctl apply --config "${K0SCTL_CONFIG}" --debug 2>&1 | tee apply.log | grep -q "not within kubernetes version skew policy"; then
  echo "Expected skew failure when applying without --force"
  exit 1
fi

# Second attempt should succeed with --force
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug --force
remoteCommand "root@manager0" "k0s version | grep -q ${K0S_VERSION}"
