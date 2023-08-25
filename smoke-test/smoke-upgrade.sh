#!/bin/bash

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e

. ./smoke.common.sh
trap cleanup EXIT


deleteCluster
createCluster

# Create config with older version and apply
K0S_VERSION="${K0S_FROM}"
echo "Installing ${K0S_VERSION}"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug

# Create config with blank version (to use latest) and apply as upgrade
K0S_VERSION="$(curl https://api.github.com/repos/k0sproject/k0s/releases | grep tag_name | cut -d"\"" -f4 | grep "+k0s" | sed -r 's/^(v[0-9]+\.[0-9]+\.[0-9]+)\+/\19999+/g' | sort -r -t "." -k1,5n | head -1 | sed 's/9999//')"

echo "Upgrading to ${K0S_VERSION} using --force"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug --force
