#!/bin/bash

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster
../k0sctl apply --config k0sctl.yaml --debug