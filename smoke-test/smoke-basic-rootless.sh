#!/usr/bin/env sh

export SSH_USER=${SSH_USER:-"k0sctl-user"}
K0SCTL_CONFIG="k0sctl-rootless.yaml"

envsubst < "k0sctl-rootless.yaml.tpl" > "${K0SCTL_CONFIG}"

set -e


. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster

for host in manager0 worker0; do
  echo "* Creating ${SSH_USER} on ${host}"
  bootloose ssh "root@${host}" -- groupadd --system k0sctl-admin
  bootloose ssh "root@${host}" -- useradd -m -G k0sctl-admin -p '*' "${SSH_USER}"
  bootloose ssh "root@${host}" -- echo "'%k0sctl-admin ALL=(ALL)NOPASSWD:ALL'" '>/etc/sudoers.d/k0sctl-admin'
  bootloose ssh "root@${host}" -- chmod 0440 /etc/sudoers.d/k0sctl-admin
  bootloose ssh "root@${host}" -- mkdir -p "/home/${SSH_USER}/.ssh"
  bootloose ssh "root@${host}" -- cp '/root/.ssh/*' "/home/${SSH_USER}/.ssh/"
  bootloose ssh "root@${host}" -- chown -R "${SSH_USER}:${SSH_USER}" "/home/${SSH_USER}/.ssh"
done

echo "* Starting apply"
../k0sctl apply --config "${K0SCTL_CONFIG}" --kubeconfig-out applykubeconfig --debug
echo "* Apply OK"

echo "* Re-applying to verify a non-root apply is idempotent"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug > reapply.log 2>&1 || { cat reapply.log; exit 1; }
echo "* Re-apply OK"

echo "* Verify the existing k0s config was readable as ${SSH_USER}"
grep -q "found existing configuration" reapply.log

echo "* Verify k0s was not reconfigured and restarted for no reason"
if grep -q "restarting k0s service" reapply.log; then
  echo "FAIL: k0s was restarted even though nothing changed"
  exit 1
fi

echo "* Verify hooks were executed on the host"
bootloose ssh root@manager0 -- grep -q hello "~${SSH_USER}/apply.hook"

echo "* Verify 'k0sctl kubeconfig' output includes 'data' block"
../k0sctl kubeconfig --config k0sctl.yaml | grep -v -- "-data"

echo "* Run kubectl on controller"
bootloose ssh root@manager0 -- k0s kubectl get nodes

echo "* Downloading kubectl for local test"
downloadKubectl

echo "* Using the kubectl from apply"
./kubectl --kubeconfig applykubeconfig get nodes

echo "* Using k0sctl kubecofig locally"
../k0sctl kubeconfig --config k0sctl.yaml > kubeconfig

echo "* Output:"
grep -v -- -data kubeconfig

echo "* Running kubectl"
./kubectl --kubeconfig kubeconfig get nodes
echo "* Done"
