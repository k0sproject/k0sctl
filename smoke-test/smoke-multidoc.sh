#!/usr/bin/env sh

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e


. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster

remoteCommand() {
  local userhost="$1"
  shift
  bootloose ssh "${userhost}" -- "$@"
}

echo "* Starting apply"
../k0sctl apply --config multidoc/ --kubeconfig-out applykubeconfig --debug
echo "* Apply OK"

echo "* Downloading kubectl for local test"
downloadKubectl

echo "*Waiting until the test pod is running"
KUBECONFIG=applykubeconfig ./kubectl wait --for=condition=Ready pod/hello --timeout=120s

sleep 2

echo "* Using kubectl to verify the test pod works"
KUBECONFIG=applykubeconfig ./kubectl exec -it pod/hello -- curl http://localhost/ | grep -q "Welcome to nginx!"

remoteCommand root@manager0 "cat /etc/k0s/k0s.yaml" > k0syaml
echo Resulting k0s.yaml:
cat k0syaml
echo "* Verifying config merging works"
grep -q "concurrencyLevel: 5" k0syaml
grep -q "enabled: false" k0syaml

echo "* Done"

