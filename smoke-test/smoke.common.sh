VAGRANT_TEMPLATE=${VAGRANT_TEMPLATE:-"Vagrantfile.template"}

export VAGRANT_BOX=${VAGRANT_BOX:-"dongsupark/flatcar-stable"}
export PRESERVE_CLUSTER=${PRESERVE_CLUSTER:-""}
export DISABLE_TELEMETRY=true
export K0S_VERSION

function createCluster() {
  envsubst < "${VAGRANT_TEMPLATE}" > Vagrantfile
  vagrant up
  vagrant status
  vagrant ssh-config host-01
  vagrant ssh-config host-02
}

function deleteCluster() {
  # cleanup any existing cluster
  envsubst < "${VAGRANT_TEMPLATE}" > Vagrantfile
  vagrant destroy --force
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
