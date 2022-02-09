#!/bin/bash

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl-dynamic.yaml"}

set -e


. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster

echo "* Starting apply"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug
echo "* Apply OK"

max_retry=5
counter=0
echo "* Verifying dynamic config reconciliation was a success"
until ../k0sctl config status -o json --config "${K0SCTL_CONFIG}" | grep -q "SuccessfulReconcile"
do
   [[ counter -eq $max_retry ]] && echo "Failed!" && exit 1
   echo "* Waiting for a couple of seconds to retry"
   sleep 5
   ((counter++))
done

echo "* OK"

echo "* Dynamic config reconciliation status:"
../k0sctl config status --config "${K0SCTL_CONFIG}"

echo "* Done"
