#!/usr/bin/env sh

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e


. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster

echo "* Starting apply"
../k0sctl apply --config "${K0SCTL_CONFIG}" --kubeconfig-out applykubeconfig --debug
echo "* Apply OK"

echo "* Verify hooks were executed on the host"
bootloose ssh root@manager0 -- grep -q hello apply.hook

echo "* Verify 'k0sctl kubeconfig' output includes 'data' block"
../k0sctl kubeconfig --config k0sctl.yaml | grep -v -- "-data"

echo "* Run kubectl on controller"
bootloose ssh root@manager0 -- k0s kubectl get nodes

echo "* Downloading kubectl for local test"
downloadKubectl

echo "* Using the kubectl from apply"
./kubectl --kubeconfig applykubeconfig get nodes

echo "* Using k0sctl kubecofig locally"
../k0sctl kubeconfig --config k0sctl.yaml --user smoke --cluster test > kubeconfig

echo "* Output:"
grep -v -- -data kubeconfig

echo "* Running kubectl"
./kubectl --kubeconfig kubeconfig --user smoke --cluster test get nodes
echo "* Done"
