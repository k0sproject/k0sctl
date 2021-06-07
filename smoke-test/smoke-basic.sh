#!/bin/bash

K0SCTL_TEMPLATE=${K0SCTL_TEMPLATE:-"k0sctl.yaml.tpl"}

set -e


. ./smoke.common.sh
trap cleanup EXIT

envsubst < "${K0SCTL_TEMPLATE}" > k0sctl.yaml

deleteCluster
createCluster
../k0sctl apply --config k0sctl.yaml --debug
../k0sctl kubeconfig --config k0sctl.yaml | grep -v -- "-data"
# just to check we can get the backup file succesfully.
# Once we have restore capability on k0sctl we might want to have the full apply&backup&reset&restore process as separate smoke
../k0sctl backup --config k0sctl.yaml