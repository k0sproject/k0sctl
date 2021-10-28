#!/bin/bash

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster
../k0sctl init
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug
../k0sctl reset --config "${K0SCTL_CONFIG}" --debug --force
