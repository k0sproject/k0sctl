#!/bin/bash

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e

. ./smoke.common.sh
trap cleanup EXIT


deleteCluster
createCluster

# Create config with older version and apply
K0S_VERSION="1.20.6+k0s.0"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug

# Create config with newer version and apply as upgrade
K0S_VERSION="1.21.1+k0s.0"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug
../k0sctl kubeconfig --config "${K0SCTL_CONFIG}" | grep -v -- "-data"