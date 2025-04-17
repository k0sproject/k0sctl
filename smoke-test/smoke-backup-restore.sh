#!/usr/bin/env sh

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}
OUT=${OUT:-""}

set -ex

. ./smoke.common.sh
trap runCleanup EXIT

# custom exit trap to cleanup the backup archives
runCleanup() {
    cleanup
    rm -f k0s_backup*.tar.gz || true
}

deleteCluster
createCluster
../k0sctl init
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug

# Collect some facts so we can validate restore actually did full restore
system_ns_uid=$(bootloose ssh root@manager0 -- k0s kubectl --kubeconfig "/var/lib/k0s/pki/admin.conf" get -n kube-system namespace kube-system -o template='{{.metadata.uid}}')
node_uid=$(bootloose ssh root@manager0 -- k0s kubectl --kubeconfig "/var/lib/k0s/pki/admin.conf" get node worker0 -o template='{{.metadata.uid}}')

if [ -z "${OUT}" ]; then
    echo "Backup with default output filename"
    ../k0sctl backup --config "${K0SCTL_CONFIG}" --debug
    RESTORE_FROM="$(ls -t k0s_backup_*.tar.gz 2>/dev/null | head -n1)"
    if [ ! -f "${RESTORE_FROM}" ]; then
        echo "Backup archive not found!"
        exit 1
    fi
else
    RESTORE_FROM="${OUT}"
  ../k0sctl backup --config "${K0SCTL_CONFIG}" --debug --output "${OUT}"
fi

echo "Restore from ${RESTORE_FROM} header hexdump:"
hexdump -C -n 1024 "${RESTORE_FROM}"

# Reset the controller
bootloose ssh root@manager0 -- k0s stop
bootloose ssh root@manager0 -- k0s reset

echo "Restoring from ${RESTORE_FROM}"

../k0sctl apply --config "${K0SCTL_CONFIG}" --debug --restore-from "${RESTORE_FROM}"

rm -f -- "${RESTORE_FROM}" || true

# Verify kube object UIDs match so we know we did full restore of the API objects
new_system_ns_uid=$(bootloose ssh root@manager0 -- k0s kubectl --kubeconfig "/var/lib/k0s/pki/admin.conf" get -n kube-system namespace kube-system -o template='{{.metadata.uid}}')
if [ "$system_ns_uid" != "$new_system_ns_uid" ]; then
    echo "kube-system UIDs do not match after restore!!!"
    exit 1
fi
new_node_uid=$(bootloose ssh root@manager0 -- k0s kubectl --kubeconfig "/var/lib/k0s/pki/admin.conf" get node worker0 -o template='{{.metadata.uid}}')
if [ "$node_uid" != "$new_node_uid" ]; then
    echo "worker0 UIDs do not match after restore!!!"
    exit 1
fi
