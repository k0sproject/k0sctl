#!/usr/bin/env bash

K0SCTL_CONFIG="k0sctl-installflags.yaml"

export K0S_CONTROLLER_FLAG="--labels=smoke-stage=1"
export K0S_WORKER_FLAG="--labels=smoke-stage=1"
envsubst < "k0sctl-installflags.yaml.tpl" > "${K0SCTL_CONFIG}"

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

echo "Installing ${K0S_VERSION}"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug
remoteCommand "root@manager0" "k0s status -o json | grep -q -- ${K0S_CONTROLLER_FLAG}"
remoteCommand "root@worker0" "k0s status -o json | grep -q -- ${K0S_WORKER_FLAG}"

echo "A re-apply should not re-install if there are no changes"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug | grep -ivq "reinstalling"

export K0S_CONTROLLER_FLAG="--labels=smoke-stage=2" 
export K0S_WORKER_FLAG="--labels=smoke-stage=2" 
envsubst < "k0sctl-installflags.yaml.tpl" > "${K0SCTL_CONFIG}"

echo "Re-applying ${K0S_VERSION} with modified installFlags"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug
remoteCommand "root@manager0" "k0s status -o json | grep -q -- ${K0S_CONTROLLER_FLAG}"
remoteCommand "root@worker0" "k0s status -o json | grep -q -- ${K0S_WORKER_FLAG}"
