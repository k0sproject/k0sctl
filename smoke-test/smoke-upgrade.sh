#!/bin/bash

K0SCTL_TEMPLATE=${K0SCTL_TEMPLATE:-"k0sctl.yaml.tpl"}

set -e

. ./smoke.common.sh
trap cleanup EXIT


deleteCluster
createCluster

# Create config with older version and apply
K0S_VERSION="1.20.6+k0s.0"
envsubst < "${K0SCTL_TEMPLATE}" > k0sctl.yaml
../k0sctl apply --config k0sctl.yaml --debug

# Create config with newer version and apply as upgrade
K0S_VERSION="1.21.1+k0s.0"
envsubst < "${K0SCTL_TEMPLATE}" > k0sctl.yaml
../k0sctl apply --config k0sctl.yaml --debug
../k0sctl kubeconfig --config k0sctl.yaml | grep -v -- "-data"