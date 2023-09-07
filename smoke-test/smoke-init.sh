#!/usr/bin/env sh

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster
../k0sctl init --key-path ./id_rsa_k0s 127.0.0.1:9022 root@127.0.0.1:9023 | ../k0sctl apply --config - --debug
