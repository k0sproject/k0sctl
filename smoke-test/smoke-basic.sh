#!/bin/bash

K0SCTL_TEMPLATE=${K0SCTL_TEMPLATE:-"k0sctl.yaml.tpl"}

set -e


. ./smoke.common.sh
trap cleanup EXIT

envsubst < "${K0SCTL_TEMPLATE}" > k0sctl.yaml

deleteCluster
createCluster
../k0sctl apply --config k0sctl.yaml --debug
# Check that the hooks got actually ran properly
footloose ssh root@manager0 -- grep -q hello apply.hook

../k0sctl kubeconfig --config k0sctl.yaml | grep -v -- "-data"

echo Downloading kubectl
downloadKubectl

../k0sctl kubeconfig --config k0sctl.yaml > kubeconfig
./kubectl --kubeconfig kubeconfig get nodes
