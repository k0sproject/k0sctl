#!/usr/bin/env sh

set -e

. ./smoke.common.sh
trap cleanup EXIT

nodeTaints() {
  bootloose ssh root@manager0 -- k0s kubectl get node "$1" -o "jsonpath='{range .spec.taints[*]}{.key}={.value}:{.effect}{\"\\n\"}{end}'"
}

hasLabel() {
  bootloose ssh root@manager0 -- k0s kubectl get node "$1" --show-labels --no-headers | grep -q "$2"
}

# apply runs k0sctl apply with the given config and keeps its output in the given log file.
apply() {
  if ! ../k0sctl apply --config "$1" --debug > "$2" 2>&1; then
    cat "$2"
    exit 1
  fi
  cat "$2"
}

nodeChanges() {
  grep -E "(set|remove) (labels|taints)" "$1" || true
}

deleteCluster
createCluster

echo "* Applying node labels and taints"
apply k0sctl-nodelabels.yaml apply.log

echo "* Verifying labels and taints on manager0"
bootloose ssh root@manager0 -- k0s kubectl get nodes --show-labels
hasLabel manager0 "node-role.kubernetes.io/smoke="
if ! grep -q "127.0.0.1:9022: remove taints node-role.kubernetes.io/control-plane" apply.log; then
  echo "the controller+worker taint was not removed from manager0"
  exit 1
fi
if nodeTaints manager0 | grep -q "node-role.kubernetes.io/control-plane"; then
  echo "manager0 still has the controller+worker taint"
  exit 1
fi

echo "* Verifying labels and taints on worker0"
hasLabel worker0 "smoke.k0sctl.io/zone=a"
hasLabel worker0 "smoke.k0sctl.io/tier=web"
nodeTaints worker0 | grep -q "smoke.k0sctl.io/dedicated=test:PreferNoSchedule"

echo "* Verifying the controller+worker taint warning is not shown when the taint is deleted"
if grep -q "will not schedule regular workloads" apply.log; then
  echo "unexpected noTaints warning"
  exit 1
fi

echo "* Re-applying the same configuration should make no node changes"
apply k0sctl-nodelabels.yaml reapply.log
if [ -n "$(nodeChanges reapply.log)" ]; then
  echo "unexpected node changes on re-apply:"
  nodeChanges reapply.log
  exit 1
fi

echo "* Changing and deleting labels and taints"
apply k0sctl-nodelabels-delete.yaml delete.log
bootloose ssh root@manager0 -- k0s kubectl get nodes --show-labels
hasLabel worker0 "smoke.k0sctl.io/zone=b"
if hasLabel worker0 "smoke.k0sctl.io/tier="; then
  echo "label smoke.k0sctl.io/tier was not deleted"
  exit 1
fi
if nodeTaints worker0 | grep -q "smoke.k0sctl.io/dedicated"; then
  echo "taint smoke.k0sctl.io/dedicated was not deleted"
  exit 1
fi
hasLabel manager0 "node-role.kubernetes.io/smoke="

echo "* Done"
