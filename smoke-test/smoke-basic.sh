#!/bin/bash

K0SCTL_TEMPLATE=${K0SCTL_TEMPLATE:-"k0sctl.yaml.tpl"}

set -e


. ./smoke.common.sh
trap cleanup EXIT

envsubst < "${K0SCTL_TEMPLATE}" > k0sctl.yaml

deleteCluster
createCluster

echo "* Starting apply"
../k0sctl apply --config k0sctl.yaml --debug
echo "* Apply OK"

echo "* Verify hooks were executed on the host"
footloose ssh root@manager0 -- grep -q hello apply.hook

echo "* Verify 'k0sctl kubeconfig' output includes 'data' block"
../k0sctl kubeconfig --config k0sctl.yaml | grep -v -- "-data"

echo "* Run kubectl on controller"
footloose ssh root@manager0 -- k0s kubectl get nodes

echo "* Downloading kubectl for local test"
downloadKubectl

echo "* Using k0sctl kubecofig locally"
../k0sctl kubeconfig --config k0sctl.yaml > kubeconfig

echo "* Output:"
cat kubeconfig | grep -v -- "-data"

echo "* Running kubectl"
./kubectl --kubeconfig kubeconfig get nodes
echo "* Done"

