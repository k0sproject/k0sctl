#!/bin/bash

set -e

function downloadOldK0sctl() {
    mkdir -p ~/.cache
    curl -sSfL https://github.com/k0sproject/k0sctl/releases/download/v0.4.0/k0sctl-linux-x64 -o ~/.cache/k0sctl_040
    chmod +x ~/.cache/k0sctl_040
}

. ./smoke.common.sh
trap cleanup EXIT

[ -f ~/.cache/k0sctl_040 ] || downloadOldK0sctl

deleteCluster
createCluster

# k0sctl 0.4.0 does not fall back from /var/cache/k0sctl, so this needs sudo.
sudo ~/.cache/k0sctl_040 apply --config k0sctl_legacy.yaml --debug
k0sctl apply --config k0sctl.yaml --debug
k0sctl kubeconfig --config k0sctl.yaml | grep -v -- "-data"