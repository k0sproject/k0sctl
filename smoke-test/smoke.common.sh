BOOTLOOSE_TEMPLATE=${BOOTLOOSE_TEMPLATE:-"bootloose.yaml.tpl"}

export LINUX_IMAGE="${LINUX_IMAGE:-"quay.io/k0sproject/bootloose-ubuntu26.04:latest"}"
export PRESERVE_CLUSTER="${PRESERVE_CLUSTER:-""}"
export K0S_VERSION

# bootloose create returns once the containers exist, which is not the same as
# sshd inside them accepting connections. k0sctl apply retries its own
# connections, so scripts that go straight into apply never see the gap, but a
# bare `bootloose ssh` issued right after createCluster races sshd and dies with
# exit 255 - intermittently, and more often on a loaded runner.
waitForSSH() {
  userhost="$1"
  timeout="${2:-60}"
  deadline=$(( $(date +%s) + timeout ))
  until bootloose ssh "${userhost}" -- true >/dev/null 2>&1; do
    if [ "$(date +%s)" -ge "${deadline}" ]; then
      echo "Timed out after ${timeout}s waiting for sshd on ${userhost}" >&2
      return 1
    fi
    sleep 1
  done
}

# Waits for manager0, which every bootloose template here defines, so no script
# has to remember to do it. Name any other machines the script will ssh into
# before k0sctl does as extra arguments.
createCluster() {
  envsubst < "${BOOTLOOSE_TEMPLATE}" > bootloose.yaml
  bootloose create
  for userhost in "root@manager0" "$@"; do
    waitForSSH "${userhost}"
  done
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

# latestPatch prints the latest non-prerelease k0s version of a minor version, such as v1.36.
# Tags are listed with git instead of the GitHub API to stay clear of API rate limits.
latestPatch() {
  local minor=$1
  git ls-remote --tags --refs https://github.com/k0sproject/k0s "refs/tags/${minor}.*" \
    | sed 's|.*refs/tags/||' \
    | grep -E "^${minor//./\\.}\.[0-9]+\+k0s\.[0-9]+$" \
    | sort -V \
    | tail -n 1
}

# k0sVersionBefore prints the latest non-prerelease k0s version that is the given number of minor
# versions older than the given k0s version.
k0sVersionBefore() {
  local version=$1
  local steps=$2
  if [[ ! "${version}" =~ ^v([0-9]+)\.([0-9]+)\. ]]; then
    echo "Can't parse the k0s version ${version}" >&2
    return 1
  fi
  local minor="v${BASH_REMATCH[1]}.$((BASH_REMATCH[2] - steps))"
  local found
  found=$(latestPatch "${minor}")
  if [ -z "${found}" ]; then
    echo "Can't find a k0s ${minor} release" >&2
    return 1
  fi
  echo "${found}"
}

downloadKubectl() {
    OS=$(uname | tr '[:upper:]' '[:lower:]')
    ARCH="amd64"
    case $(uname -m) in
        arm,arm64) ARCH="arm64" ;;
        riscv64) ARCH="riscv64" ;;
    esac
    [ -f kubectl ] || (curl -L https://dl.k8s.io/release/v1.28.2/bin/${OS}/${ARCH}/kubectl > ./kubectl && chmod +x ./kubectl)
    ./kubectl version --client
}
