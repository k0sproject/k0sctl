#!/bin/bash

set -e

function downloadOldK0sctl() {
    curl -sSfL https://github.com/k0sproject/k0sctl/releases/download/v0.4.0/k0sctl-linux-x64 -o k0sctl_040
    chmod +x k0sctl_040
}

. ./smoke.common.sh
trap cleanup EXIT

downloadOldK0sctl

deleteCluster
createCluster
./k0sctl_040 apply --config k0sctl_legacy.yaml --debug
../k0sctl apply --config k0sctl.yaml --debug
../k0sctl kubeconfig --config k0sctl.yaml | grep -v -- "-data"