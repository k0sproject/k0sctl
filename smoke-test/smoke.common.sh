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


# dumpK0sLogs prints the recent k0s service logs from the given machines, to
# show why k0s was not running when a smoke test fails. It covers systemd and
# the OpenRC log files.
dumpK0sLogs() {
  for machine in "$@"; do
    echo "* k0s service logs on ${machine}"
    bootloose ssh "root@${machine}" -- 'for s in k0scontroller k0sworker; do
      if command -v journalctl > /dev/null; then journalctl -u "$s" --no-pager -n 300 2> /dev/null; fi
      for f in "/var/log/$s.log" "/var/log/$s.err"; do if [ -f "$f" ]; then echo "--- $f"; tail -n 300 "$f"; fi; done
    done' || true
  done
}

cleanup() {
    echo "Cleaning up..."

    if [ -z "${PRESERVE_CLUSTER}" ]; then
      deleteCluster
    fi
}

# latestPatch prints the latest non-prerelease k0s version of a minor version, such as v1.36,
# that has a downloadable binary.
# Tags are listed with git instead of the GitHub API to stay clear of API rate limits. A tag
# can exist long before its release is published, and the binaries of a draft release can't
# be downloaded, so the tags are tried from the newest until one has a binary.
# k0s_sort sorts the versions, because sort -V does not order k0s versions correctly.
latestPatch() {
  k0s_sort=$(command -v k0s_sort || echo "$(go env GOPATH)/bin/k0s_sort")
  for candidate in $(git ls-remote --tags --refs https://github.com/k0sproject/k0s "refs/tags/$1.*" \
    | sed 's|.*refs/tags/||' \
    | grep -E "^$(echo "$1" | sed 's/\./\\./g')\.[0-9]+\+k0s\.[0-9]+$" \
    | "${k0s_sort}" \
    | sed 's/^v*/v/' \
    | awk '{ v[NR] = $0 } END { for (i = NR; i > 0; i--) print v[i] }'); do
    if k0sBinaryAvailable "${candidate}"; then
      echo "${candidate}"
      return 0
    fi
    echo "Skipping k0s ${candidate}, its binary is not downloadable" >&2
  done
}

# k0sBinaryAvailable succeeds when the linux amd64 binary of the given k0s version can be downloaded.
k0sBinaryAvailable() {
  url="https://github.com/k0sproject/k0s/releases/download/$(echo "$1" | sed 's/+/%2B/')/k0s-$1-amd64"
  curl -sfIL -o /dev/null "${url}"
}

# k0sVersionBefore prints the latest non-prerelease k0s version that is the given number of minor
# versions older than the given k0s version.
#
# This file is also sourced by scripts that run under plain sh, so no bashisms here.
k0sVersionBefore() {
  major=$(echo "$1" | sed -n 's/^v\([0-9][0-9]*\)\.[0-9][0-9]*\..*/\1/p')
  minor=$(echo "$1" | sed -n 's/^v[0-9][0-9]*\.\([0-9][0-9]*\)\..*/\1/p')
  if [ -z "${major}" ] || [ -z "${minor}" ]; then
    echo "Can't parse the k0s version $1" >&2
    return 1
  fi
  wanted="v${major}.$((minor - $2))"
  found=$(latestPatch "${wanted}")
  if [ -z "${found}" ]; then
    echo "Can't find a k0s ${wanted} release" >&2
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
