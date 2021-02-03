FOOTLOOSE_TEMPLATE=${FOOTLOOSE_TEMPLATE:-"footloose.yaml.tpl"}

export LINUX_IMAGE=${LINUX_IMAGE:-"quay.io/footloose/ubuntu18.04"}
export PRESERVE_CLUSTER=${PRESERVE_CLUSTER:-""}


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