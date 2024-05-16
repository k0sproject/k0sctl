#!/usr/bin/env sh

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl-controller-swap.yaml"}

set -ex


. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster

echo "* Starting apply"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug
echo "* Apply OK"

echo "* Get the ip of the last controller"
controllerip=$(bootloose show manager2 -o json | grep '"ip"' | head -1 | cut -d'"' -f4)

echo "* Wipe controller 3"
docker rm -fv "$(bootloose show manager2 -o json | grep '"container"' | head -1 | cut -d'"' -f4)"

echo "* Verify its gone"
bootloose show manager2 | grep "Not created"

echo "* Recreate controller2"
createCluster

echo "* Verify its back and IP is the same"
bootloose show manager2 | grep "Running"
newip=$(bootloose show manager2 -o json | grep '"ip"' | head -1 | cut -d'"' -f4)
if [ "$controllerip" != "$newip" ]; then
  echo "IP mismatch: $controllerip != $newip - ip should get reused"
  exit 1
fi

echo "* Re-apply should fail because of known hosts"
if ../k0sctl apply --config "${K0SCTL_CONFIG}" --debug; then
  echo "Re-apply should have failed because of known hosts"
  exit 1
fi

echo "* Clear known hosts"
truncate -s 0 ~/.ssh/known_hosts

echo "* Re-apply should fail because of replaced controller"
if ../k0sctl apply --config "${K0SCTL_CONFIG}" --debug; then
  echo "Re-apply should have failed because of replaced controller"
  exit 1
fi

echo "* Perform etcd member removal"
bootloose ssh root@manager0 -- k0s etcd leave --peer-address "$controllerip"

echo "* Re-apply should succeed"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug

echo "* Done"
