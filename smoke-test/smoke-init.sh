#!/bin/bash

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster
footloose status | grep -v NAME | awk '{print $5 ":" $4}'|sed 's/}//' | ../k0sctl init --keypath ./id_rsa_k0s | ../k0sctl apply --config - --debug
