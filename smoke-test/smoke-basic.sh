#!/bin/bash

K0SCTL_YAML=${K0SCTL_YAML:-"k0sctl.yaml"}

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster
../k0sctl init
../k0sctl apply --config ${K0SCTL_YAML} --debug
../k0sctl kubeconfig --config ${K0SCTL_YAML} | grep -v -- "-data"