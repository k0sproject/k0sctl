#!/bin/bash

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster
../k0sctl init
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug --trace
../k0sctl reset --config "${K0SCTL_CONFIG}" --debug --trace --force
echo "came back?"
echo "or did it?"
exit 0
