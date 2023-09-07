#!/usr/bin/env sh

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster
echo "* Applying"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug
echo "* Resetting"
../k0sctl reset --config "${K0SCTL_CONFIG}" --debug --force
echo "* Done, cleaning up"
