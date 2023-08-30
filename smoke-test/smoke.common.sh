FOOTLOOSE_TEMPLATE=${FOOTLOOSE_TEMPLATE:-"footloose.yaml.tpl"}

export LINUX_IMAGE="${LINUX_IMAGE:-"quay.io/footloose/ubuntu18.04"}"
export PRESERVE_CLUSTER="${PRESERVE_CLUSTER:-""}"
export DISABLE_TELEMETRY=true
export K0S_VERSION

createCluster() {
  envsubst < "${FOOTLOOSE_TEMPLATE}" > footloose.yaml
  footloose create
  if [ "${LINUX_IMAGE}" = "quay.io/footloose/debian10" ]; then
    for host in $(footloose status -o json|grep hostname|cut -d"\"" -f4); do
      footloose ssh root@"${host}" -- rm -f /etc/machine-id /var/lib/dbus/machine-id
      footloose ssh root@"${host}" -- systemd-machine-id-setup
    done
  fi
}

deleteCluster() {
  # cleanup any existing cluster
  envsubst < "${FOOTLOOSE_TEMPLATE}" > footloose.yaml
  footloose delete && docker volume prune -f
}


cleanup() {
    echo "Cleaning up..."

    if [ -z "${PRESERVE_CLUSTER}" ]; then
      deleteCluster
    fi
}

downloadKubectl() {
    OS=$(uname | tr '[:upper:]' '[:lower:]')
    ARCH="amd64"
    case $(uname -m) in
        arm,arm64) ARCH="arm64" ;;
    esac
    [ -f kubectl ] || (curl -L https://storage.googleapis.com/kubernetes-release/release/v1.21.3/bin/"${OS}"/${ARCH}/kubectl > ./kubectl && chmod +x ./kubectl)
    ./kubectl version --client
}
