FOOTLOOSE_TEMPLATE=${FOOTLOOSE_TEMPLATE:-"footloose.yaml.tpl"}

export LINUX_IMAGE=${LINUX_IMAGE:-"quay.io/footloose/ubuntu18.04"}
export PRESERVE_CLUSTER=${PRESERVE_CLUSTER:-""}
export DISABLE_TELEMETRY=true
export K0S_VERSION=${K0S_VERSION:-"1.21.1+k0s.0"}

function createCluster() {
  envsubst < "${FOOTLOOSE_TEMPLATE}" > footloose.yaml
  footloose create
}

function deleteCluster() {
  # cleanup any existing cluster
  envsubst < "${FOOTLOOSE_TEMPLATE}" > footloose.yaml
  footloose delete && docker volume prune -f
}


function cleanup() {
    echo -e "Cleaning up..."

    if [ -z "${PRESERVE_CLUSTER}" ]; then
      deleteCluster
    fi
}

function downloadKubectl() {
    OS=$(uname | tr '[:upper:]' '[:lower:]')
    ARCH="amd64"
    case $(uname -m) in
        arm,arm64) ARCH="arm64" ;;
    esac
    [ -f kubectl ] || (curl -L https://storage.googleapis.com/kubernetes-release/release/v1.21.3/bin/${OS}/${ARCH}/kubectl > ./kubectl && chmod +x ./kubectl)
    ./kubectl version --client
}