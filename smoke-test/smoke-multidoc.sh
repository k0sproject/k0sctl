#!/usr/bin/env sh

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}

set -e


. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
createCluster

remoteCommand() {
  local userhost="$1"
  shift
  bootloose ssh "${userhost}" -- "$@"
}

echo "* Starting apply"
../k0sctl apply --config multidoc/ --kubeconfig-out applykubeconfig --debug
echo "* Apply OK"

echo "* Downloading kubectl for local test"
downloadKubectl
    
export KUBECONFIG=applykubeconfig 

echo "*Waiting until the test pod is running"
./kubectl wait --for=condition=Ready pod/hello --timeout=120s

retries=10
delay=2
nginx_ready=false
i=1

while [ "$i" -le "$retries" ]; do
    echo "* Attempt $i: Checking if nginx is ready..."
    if kubectl exec pod/hello -- curl -s http://localhost/ | grep -q "Welcome to nginx!"; then
        echo "nginx is ready!"
        nginx_ready=true
        break
    fi
    echo "  - nginx is not ready"
    sleep $delay
    i=$((i + 1))
done

if [ "$nginx_ready" = false ]; then
    echo "nginx failed to become ready after $retries attempts."
    exit 1
fi

echo " - nginx is ready"

remoteCommand root@manager0 "cat /etc/k0s/k0s.yaml" > k0syaml
echo Resulting k0s.yaml:
cat k0syaml
echo "* Verifying config merging works"
grep -q "concurrencyLevel: 5" k0syaml
grep -q "enabled: false" k0syaml

echo "* Done"

