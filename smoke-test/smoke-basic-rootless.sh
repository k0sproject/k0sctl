#!/usr/bin/env sh

export SSH_USER=${SSH_USER:-"k0sctl-user"}
K0SCTL_CONFIG="k0sctl-rootless.yaml"
mkdir foo
FOO_DIR=$(cd foo && pwd)
export K0SCTL_SSH_KEY="${FOO_DIR}/key"

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

cp id_rsa_k0s foo/key

echo "* Starting apply"
../k0sctl apply --config "${K0SCTL_CONFIG}" --kubeconfig-out applykubeconfig --debug
echo "* Apply OK"

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
