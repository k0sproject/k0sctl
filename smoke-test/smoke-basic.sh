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

K0S="k0s"

if [ "${INSTALL_PATH}" != "" ]; then
    echo "* Checking k0s binary is at the custom install path ${INSTALL_PATH}"
    bootloose ssh root@manager0 -- ls -al "$(dirname "${INSTALL_PATH}")"
    bootloose ssh root@manager0 -- test -x "${INSTALL_PATH}"
    K0S="${INSTALL_PATH}"
fi

echo "* Verify hooks were executed on the host"
bootloose ssh root@manager0 -- grep -q hello apply.hook

echo "* Verify 'k0sctl kubeconfig' output includes 'data' block"
(../k0sctl kubeconfig --debug --trace --config "${K0SCTL_CONFIG}" 2> kubeconfig.log | grep -v -- "-data") || (echo "No data block found in kubeconfig output"; cat "kubeconfig.log"; exit 1)

echo "* Run kubectl on controller"
bootloose ssh root@manager0 -- "${K0S}" kubectl get nodes

echo "* Downloading kubectl for local test"
downloadKubectl

echo "* Using the kubectl from apply"
./kubectl --kubeconfig applykubeconfig get nodes

echo "* Using k0sctl kubecofig locally"
../k0sctl kubeconfig --config "${K0SCTL_CONFIG}" --user smoke --cluster test > kubeconfig

echo "* Output:"
grep -v -- -data kubeconfig

echo "* Running kubectl"
./kubectl --kubeconfig kubeconfig --user smoke --cluster test get nodes
echo "* Done"
