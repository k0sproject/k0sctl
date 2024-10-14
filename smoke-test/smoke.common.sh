BOOTLOOSE_TEMPLATE=${BOOTLOOSE_TEMPLATE:-"bootloose.yaml.tpl"}

export LINUX_IMAGE="${LINUX_IMAGE:-"quay.io/k0sproject/bootloose-ubuntu20.04"}"
export PRESERVE_CLUSTER="${PRESERVE_CLUSTER:-""}"
export DISABLE_TELEMETRY=true
export K0S_VERSION
export K0S_API_EXTERNAL_ADDRESS="${K0S_API_EXTERNAL_ADDRESS:-172.20.0.1}"

createCluster() {
  envsubst < "${BOOTLOOSE_TEMPLATE}" > bootloose.yaml
  bootloose create
}

deleteCluster() {
  # cleanup any existing cluster
  envsubst < "${BOOTLOOSE_TEMPLATE}" > bootloose.yaml
  bootloose delete && docker volume prune -f
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
    [ -f kubectl ] || (curl -L https://storage.googleapis.com/kubernetes-release/release/v1.28.2/bin/"${OS}"/${ARCH}/kubectl > ./kubectl && chmod +x ./kubectl)
    ./kubectl version --client
}
