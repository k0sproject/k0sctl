#!/bin/bash

K0SCTL_YAML=${K0SCTL_YAML:-"k0sctl.yaml"}

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster
../k0sctl apply --config ${K0SCTL_YAML} --debug
../k0sctl kubeconfig --config ${K0SCTL_YAML} | grep -v -- "-data"
# just to check we can get the backup file succesfully.
# Once we have restore capability on k0sctl we might want to have the full apply&backup&reset&restore process as separate smoke
../k0sctl backup --config ${K0SCTL_YAML}