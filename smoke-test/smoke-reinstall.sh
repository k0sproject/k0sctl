#!/usr/bin/env bash

K0SCTL_CONFIG="k0sctl-installflags.yaml"
export K0S_VERSION=v1.34.4+k0s.0
export K0S_CONTROLLER_FLAG="--labels=smoke-stage=1"
export K0S_WORKER_FLAG="--labels=smoke-stage=1"
envsubst < "k0sctl-installflags.yaml.tpl" > "${K0SCTL_CONFIG}"

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster

remoteCommand() {
  local userhost="$1"
  shift
  echo "* Running command on ${userhost}: $*"
  bootloose ssh "${userhost}" -- "$*"
}

echo "Installing ${K0S_VERSION}"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug | tee apply.log
echo "Initial apply should not perform a re-install"
grep -ivq "reinstalling" apply.log

echo "Install flags should contain the expected flag on a controller"
remoteCommand "root@manager0" "k0s status -o json | grep -q -- ${K0S_CONTROLLER_FLAG}"
if echo $LINUX_IMAGE | grep -q "ubuntu"; then
  remoteCommand "root@manager0" journalctl -xeu k0scontroller --no-pager
else
  remoteCommand "root@manager0" tail -n 20 "/var/log/k0s.log"
fi

echo "Install flags should contain the expected flag on a worker"
remoteCommand "root@worker0" "k0s status -o json | grep -q -- ${K0S_WORKER_FLAG}"
if echo $LINUX_IMAGE | grep -q "ubuntu"; then
  remoteCommand "root@worker0" journalctl -xeu k0sworker --no-pager
else
  remoteCommand "root@worker0" tail -n 20 "/var/log/k0s.log"
fi

echo "A re-apply should not re-install if there are no changes"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug | tee apply.log
grep -ivq "reinstalling" apply.log

export K0S_CONTROLLER_FLAG="--labels=smoke-stage=2" 
export K0S_WORKER_FLAG="--labels=smoke-stage=2" 
envsubst < "k0sctl-installflags.yaml.tpl" > "${K0SCTL_CONFIG}"

echo "Re-applying ${K0S_VERSION} with modified installFlags"
../k0sctl apply --config "${K0SCTL_CONFIG}" --debug | tee apply.log
echo "A re-apply should perform a re-install if there are changes"
grep -iq "reinstalling" apply.log

max_retry=20
counter=0
echo "Install flags should change for controller"
until remoteCommand "root@manager0" "k0s status -o json | grep -q -- ${K0S_CONTROLLER_FLAG}"
do
   [ $counter -eq $max_retry ] && echo "Failed!" && exit 1
   echo "* Waiting for a couple of seconds to retry"
   if echo $LINUX_IMAGE | grep -q "ubuntu"; then
     remoteCommand "root@manager0" journalctl -xeu k0scontroller --no-pager
   else
     remoteCommand "root@manager0" tail -n 20 "/var/log/k0s.log"
   fi
   sleep 10
   counter=$((counter+1))
done

counter=0
echo "Install flags should change for worker"
until remoteCommand "root@worker0" "k0s status -o json | grep -q -- ${K0S_WORKER_FLAG}"
do
   [ $counter -eq $max_retry ] && echo "Failed!" && exit 1
   echo "* Waiting for a couple of seconds to retry"
   if echo $LINUX_IMAGE | grep -q "ubuntu"; then
     remoteCommand "root@worker0" journalctl -xeu k0sworker --no-pager
   else
     remoteCommand "root@worker0" tail -n 20 "/var/log/k0s.log"
   fi
   sleep 10 
   counter=$((counter+1))
done
