#!/usr/bin/env sh

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster

EXCLUDES='proc|sys|dev|tmp|run|var/log|var/cache'

snapshot() {
  bootloose ssh root@manager0 -- tree -af -I "$EXCLUDES" / \
    | sed 's/^[[:space:]│├└─]*//' \
    | sort
}

echo "* File system snapshot"
snapshot > /tmp/tree_before.txt

echo "* Applying"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug
echo "* Resetting"
../k0sctl reset --config "${K0SCTL_CONFIG}" --debug --force
echo "* Ensure binary was removed"
bootloose ssh root@manager0 -- '[ ! -f /usr/local/bin/k0s ]'
echo "* Second snapshot"
snapshot > /tmp/tree_after.txt
echo "* File system diff:"
diff -u /tmp/tree_before.txt /tmp/tree_after.txt || true

echo "* Done, cleaning up"
