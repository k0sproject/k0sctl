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

remoteCommand root@manager0 "cat /etc/k0s/k0s.yaml" > k0syaml
echo Resulting k0s.yaml:
cat k0syaml
echo "* Verifying config merging works"
grep -q "concurrencyLevel: 5" k0syaml
grep -q "enabled: false" k0syaml

echo "* Done"

