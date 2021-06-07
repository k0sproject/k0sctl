#!/bin/bash

K0SCTL_TEMPLATE=${K0SCTL_TEMPLATE:-"k0sctl.yaml.tpl"}

set -e

. ./smoke.common.sh
trap cleanup EXIT

envsubst < "${K0SCTL_TEMPLATE}" > k0sctl.yaml

deleteCluster
createCluster
../k0sctl init
../k0sctl apply --config k0sctl.yaml --debug
../k0sctl reset --config k0sctl.yaml --debug --force
