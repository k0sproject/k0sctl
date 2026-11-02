#!/usr/bin/env bash

K0SCTL_CONFIG=${K0SCTL_CONFIG:-"k0sctl.yaml"}
K0S_TO=${K0S_TO:-"$(curl -s "https://docs.k0sproject.io/stable.txt")"}

set -e

. ./smoke.common.sh
trap cleanup EXIT

remoteCommand() {
  local userhost="$1"
  shift
  echo "* Running command on ${userhost}: $*"
  bootloose ssh "${userhost}" -- "$*"
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

applyVersion() {
  K0S_VERSION=$1
  shift
  echo "Applying k0s ${K0S_VERSION}"
  ../k0sctl apply --config "${K0SCTL_CONFIG}" --debug "$@"
  remoteCommand "root@manager0" "k0s version | grep -q ${K0S_VERSION}"
}

if [[ ! "${K0S_TO}" =~ ^v([0-9]+)\.([0-9]+)\. ]]; then
  echo "Can't parse the target k0s version ${K0S_TO}"
  exit 1
fi
major=${BASH_REMATCH[1]}
minor=${BASH_REMATCH[2]}
K0S_PREV=$(latestPatch "v${major}.$((minor - 1))")
K0S_FROM=$(latestPatch "v${major}.$((minor - 2))")
if [ -z "${K0S_PREV}" ] || [ -z "${K0S_FROM}" ]; then
  echo "Can't find the k0s releases for the two minor versions before ${K0S_TO}"
  exit 1
fi
echo "Upgrade path: ${K0S_FROM} -> ${K0S_PREV} -> ${K0S_TO}"

deleteCluster
createCluster

applyVersion "${K0S_FROM}"

# Going straight to the target skips a minor version, so the version skew check must block it before anything changes
K0S_VERSION="${K0S_TO}"
echo "Upgrading to k0s ${K0S_VERSION}, expecting the version skew check to fail the apply"
if ../k0sctl apply --config "${K0SCTL_CONFIG}" --debug > smoke-upgrade-skew.log 2>&1; then
  cat smoke-upgrade-skew.log
  echo "Expected the version skew check to fail the apply"
  exit 1
fi
cat smoke-upgrade-skew.log
grep -q "one minor version at a time" smoke-upgrade-skew.log
remoteCommand "root@manager0" "k0s version | grep -q ${K0S_FROM}"

# Upgrade one minor version at a time
applyVersion "${K0S_PREV}"
applyVersion "${K0S_TO}"
