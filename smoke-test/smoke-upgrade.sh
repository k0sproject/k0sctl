#!/bin/bash

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e

. ./smoke.common.sh
trap cleanup EXIT


deleteCluster
createCluster

# Create config with older version and apply
K0S_VERSION="${K0S_FROM}"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug

# Create config with blank version (to use latest) and apply as upgrade
K0S_VERSION=""
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug
../k0sctl kubeconfig --config "${K0SCTL_CONFIG}" | grep -v -- "-data"