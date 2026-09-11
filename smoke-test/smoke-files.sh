#!/usr/bin/env sh

K0SCTL_TEMPLATE=${K0SCTL_TEMPLATE:-"k0sctl.yaml.tpl"}
# The template adds a machine that serves the URL sources, so that this test can
# decide what the server says about them.
BOOTLOOSE_TEMPLATE=${BOOTLOOSE_TEMPLATE:-"bootloose.yaml.files.tpl"}
export BOOTLOOSE_TEMPLATE

set -e

. ./smoke.common.sh
trap cleanup EXIT

deleteCluster
# filesrv0 is ssh'd into long before k0sctl connects to anything, so it has to be
# waited for as well - k0sctl's own connection retries do not cover it.
createCluster root@filesrv0

remoteCommand() {
  local userhost="$1"
  shift
  bootloose ssh "${userhost}" -- "$@"
}

remoteFileExist() {
  local userhost="$1"
  local path="$2"
  remoteCommand "${userhost}" test -e "${path}"
}

remoteFileContent() {
  local userhost="$1"
  local path="$2"
  remoteCommand "${userhost}" cat "${path}"
}

remoteSha256() {
  local userhost="$1"
  local path="$2"
  remoteCommand "${userhost}" sha256sum "${path}" | awk '{print $1}' | tr -d '\r'
}

FILESRV_HOST="root@filesrv0"

# serverLog prints the requests the file server has served so far, one
# "<method> <path> <status>" per line.
serverLog() {
  remoteCommand "${FILESRV_HOST}" cat /srv/requests.log | tr -d '\r'
}

# serverRequests counts the requests for a path, for example
# "serverRequests GET /files/plain.bin".
serverRequests() {
  serverLog | grep -c "^$1 $2 " || true
}

# assertRequestCount fails the test when a path has been requested more times
# than it was when the count was taken.
assertRequestCount() {
  local method="$1"
  local path="$2"
  local want="$3"
  local now
  now=$(serverRequests "${method}" "${path}")
  if [ "${now}" != "${want}" ]; then
    echo "FAIL"
    echo "  ${method} ${path} was requested ${now} times, expected ${want}"
    serverLog
    exit 1
  fi
}

echo "* Creating random files"
mkdir -p upload
mkdir -p upload/nested
mkdir -p upload_chmod

head -c 8192 </dev/urandom > upload/toplevel.txt
head -c 8192 </dev/urandom > upload/nested/nested.txt
head -c 8192 </dev/urandom > upload/nested/exclude-on-glob
cat << EOF > upload_chmod/script.sh
#!/bin/sh
echo hello
EOF
chmod 0744 upload_chmod/script.sh

echo "* Starting the file server"
# Not every image in the matrix ships python3, and the file server machine is
# not the one under test, so installing it there is fair game.
remoteCommand "${FILESRV_HOST}" "command -v python3 > /dev/null || { apt-get update > /dev/null && apt-get install -y python3 > /dev/null; }"
remoteCommand "${FILESRV_HOST}" "mkdir -p /srv/files && : > /srv/requests.log"
# Handing the script over on the command line keeps this from depending on file
# transfer support in the ssh helper.
remoteCommand "${FILESRV_HOST}" "printf %s '$(base64 < filesrv.py | tr -d '\n')' | base64 -d > /srv/filesrv.py"
remoteCommand "${FILESRV_HOST}" "head -c 1048576 </dev/urandom > /srv/files/plain.bin"
remoteCommand "${FILESRV_HOST}" "head -c 2097152 </dev/urandom > /srv/files/bundle.bin"
remoteCommand "${FILESRV_HOST}" "head -c 8192 </dev/urandom > /srv/files/badsum.bin"
remoteCommand "${FILESRV_HOST}" "head -c 524288 </dev/urandom > /srv/files/adopt.bin"
remoteCommand "${FILESRV_HOST}" "echo ok > /srv/files/health"
remoteCommand "${FILESRV_HOST}" "setsid python3 /srv/filesrv.py > /srv/filesrv.out 2>&1 < /dev/null &"

FILESRV="$(remoteCommand "${FILESRV_HOST}" hostname -i | awk '{print $1}' | tr -d '\r'):8080"
export FILESRV
echo "  - serving on ${FILESRV}"

echo "* Waiting for the file server to answer from the cluster"
i=0
until remoteCommand root@manager0 "curl -sSf -o /dev/null http://${FILESRV}/files/health"; do
  i=$((i+1))
  if [ $i -gt 30 ]; then
    echo "the file server did not come up"
    remoteCommand "${FILESRV_HOST}" cat /srv/filesrv.out
    exit 1
  fi
  sleep 1
done

PLAIN_SHA256=$(remoteSha256 "${FILESRV_HOST}" /srv/files/plain.bin)
BUNDLE_SHA256=$(remoteSha256 "${FILESRV_HOST}" /srv/files/bundle.bin)
ADOPT_SHA256=$(remoteSha256 "${FILESRV_HOST}" /srv/files/adopt.bin)
export BUNDLE_SHA256 ADOPT_SHA256

# Put one of the files on the host by other means, so the apply meets a
# destination that is already correct and has nothing recording it.
remoteCommand root@manager0 "mkdir -p /root/srv && curl -sSf -o /root/srv/adopt.bin http://${FILESRV}/files/adopt.bin"
adopt_gets=$(serverRequests GET /files/adopt.bin)

# The marker makes the file server cut the first transfer of the bundle short,
# so the download has to be resumed to ever finish. It is consumed by that
# transfer, which leaves the retry to succeed.
remoteCommand "${FILESRV_HOST}" touch /srv/files/bundle.bin.drop

envsubst < k0sctl-files.yaml.tpl > k0sctl.yaml

echo "* Creating test user"
remoteCommand root@manager0 useradd test

echo "* Starting apply"
../k0sctl apply --config k0sctl.yaml --debug

echo "* Verifying uploads"
remoteCommand root@manager0 "apt-get update > /dev/null && apt-get install tree > /dev/null && tree -fp"

printf %s "  - Single file using destination file path and user:group .. "
remoteFileExist root@manager0 /root/singlefile/renamed.txt
printf %s "[exist]"
remoteCommand root@manager0 stat -c '%U:%G' /root/singlefile/renamed.txt | grep -q test:test
printf %s "[stat]"
echo "OK"

printf %s "  - File from inline data .. "
remoteFileExist root@manager0 /root/content/hello.sh
printf %s "[exist]"
remoteCommand root@manager0 stat -c '%U:%G' /root/content/hello.sh | grep -q test:test
printf %s "[stat]"
remoteCommand root@manager0 /root/content/hello.sh | grep -q hello
printf %s "[run] "
echo "OK"

printf %s "  - Single file using destination dir .. "
remoteFileExist root@manager0 /root/destdir/toplevel.txt
echo "OK"

printf %s "  - PermMode 644 .. "
remoteFileExist root@manager0 /root/chmod/script.sh
printf %s "[exist]"
remoteCommand root@manager0 stat -c '%a' /root/chmod/script.sh | grep -q 644
printf %s "[stat] "
echo "OK"

printf %s "  - PermMode transfer .."
remoteFileExist root@manager0 /root/chmod_exec/script.sh
printf %s "[exist] "
remoteCommand root@manager0 stat -c '%a' /root/chmod_exec/script.sh | grep -q 744
printf %s "[stat] "
remoteCommand root@manager0 /root/chmod_exec/script.sh | grep -q hello
printf %s "[run] "
echo "OK"

printf %s "  - Directory using destination dir .. "
remoteFileExist root@manager0 /root/dir/toplevel.txt
printf %s "[1] "
remoteFileExist root@manager0 /root/dir/nested/nested.txt
printf %s "[2] "
remoteFileExist root@manager0 /root/dir/nested/exclude-on-glob
printf %s "[3] "
echo "OK"

printf %s "  - Glob using destination dir .. "
remoteFileExist root@manager0 /root/glob/toplevel.txt
printf %s "[1] "
remoteFileExist root@manager0 /root/glob/nested/nested.txt
printf %s "[2] "
if remoteFileExist root@manager0 /root/glob/nested/exclude-on-glob; then exit 1; fi
printf %s "[3] "
remoteCommand root@manager0 stat -c '%a' /root/glob | grep -q 700
printf %s "[stat1]"
remoteCommand root@manager0 stat -c '%a' /root/glob/nested | grep -q 700
printf %s "[stat2]"
echo "OK"

printf %s "  - URL using destination file .. "
remoteFileExist root@manager0 /root/url/releases.json
printf %s "[exist] "
remoteFileContent root@manager0 /root/url/releases.json | grep -q html_url
printf %s "[content] "
echo "OK"

printf %s "  - URL using destination dir .. "
remoteFileExist root@manager0 /root/url_destdir/releases
printf %s "[exist] "
remoteFileContent root@manager0 /root/url_destdir/releases | grep -q html_url
printf %s "[content] "
echo "OK"

printf %s "  - URL from the file server .. "
[ "$(remoteSha256 root@manager0 /root/srv/plain.bin)" = "${PLAIN_SHA256}" ]
printf %s "[etag source] "
[ "$(remoteSha256 root@manager0 /root/srv/noetag.bin)" = "${PLAIN_SHA256}" ]
printf %s "[no-etag source] "
echo "OK"

printf %s "  - Interrupted download of a file with a sha256 was resumed .. "
serverLog | grep -q "^GET /files/bundle.bin 200 dropped$"
printf %s "[interrupted] "
serverLog | grep -q "^GET /files/bundle.bin 206$"
printf %s "[resumed] "
[ "$(remoteSha256 root@manager0 /root/srv/bundle.bin)" = "${BUNDLE_SHA256}" ]
printf %s "[checksum] "
echo "OK"

printf %s "  - A pre-seeded file with a sha256 is adopted, not downloaded .. "
assertRequestCount GET /files/adopt.bin "${adopt_gets}"
printf %s "[not downloaded] "
remoteCommand root@manager0 "ls /root/.cache/k0sctl/downloads | grep -q '^adopt.bin-'"
printf %s "[recorded] "
echo "OK"

# The download records are written as the connecting user, so they must not need
# sudo and must not end up beside the downloaded files.
printf %s "  - Downloads are recorded in the user's cache dir .. "
remoteCommand root@manager0 "test -d /root/.cache/k0sctl/downloads"
printf %s "[exist] "
remoteCommand root@manager0 "ls /root/.cache/k0sctl/downloads | grep -q '^bundle.bin-'"
printf %s "[named] "
if remoteCommand root@manager0 "ls /root/srv | grep -qv '\.bin$'"; then
  echo "FAIL"
  remoteCommand root@manager0 "ls -la /root/srv"
  exit 1
fi
printf %s "[no leftovers] "
echo "OK"

echo "* Re-applying to verify the uploads are idempotent"
plain_gets=$(serverRequests GET /files/plain.bin)
noetag_gets=$(serverRequests GET /noetag/plain.bin)
bundle_gets=$(serverRequests GET /files/bundle.bin)
adopt_gets=$(serverRequests GET /files/adopt.bin)

../k0sctl apply --config k0sctl.yaml --debug > reapply.log 2>&1 || { cat reapply.log; exit 1; }

# Host.FileChanged compares the local and remote size and mtime and logs which of
# the two differed, so a re-upload of an unchanged file always leaves a trace in
# the debug log.
printf %s "  - No unchanged file is considered changed .. "
if grep -qE "file (sizes|modtimes) for .* differ|(local|remote) stat failed" reapply.log; then
  echo "FAIL"
  grep -E "file (sizes|modtimes) for .* differ|(local|remote) stat failed" reapply.log
  exit 1
fi
echo "OK"

printf %s "  - Unchanged files are skipped on re-apply .. "
if ! grep -q "hasn.t been changed, skipping upload" reapply.log; then
  echo "FAIL"
  grep -i "upload" reapply.log
  exit 1
fi
echo "OK"

printf %s "  - Unchanged URL sources are not downloaded again .. "
assertRequestCount GET /files/plain.bin "${plain_gets}"
assertRequestCount GET /noetag/plain.bin "${noetag_gets}"
assertRequestCount GET /files/bundle.bin "${bundle_gets}"
assertRequestCount GET /files/adopt.bin "${adopt_gets}"
printf %s "[no requests] "
if ! grep -q "is already downloaded to /root/srv/plain.bin" reapply.log; then
  echo "FAIL"
  grep -i "download" reapply.log
  exit 1
fi
printf %s "[skipped] "
echo "OK"

# Only the file server sources are expected to be left alone here. The github API
# sources in this config answer with a weak etag and no last-modified, which is
# precisely the case k0sctl refuses to draw a conclusion from, so they are
# downloaded on every apply and do mark the host for upgrade - by design. Real
# release assets, which is what this feature is for, send a strong etag.
printf %s "  - A skipped URL source does not mark the host for upgrade .. "
if grep -q "marked for upgrade because /root/srv/" reapply.log; then
  echo "FAIL"
  grep "marked for upgrade because /root/srv/" reapply.log
  exit 1
fi
echo "OK"

echo "* Replacing a served file to verify changed sources are downloaded again"
remoteCommand "${FILESRV_HOST}" "head -c 1572864 </dev/urandom > /srv/files/plain.bin"
NEW_PLAIN_SHA256=$(remoteSha256 "${FILESRV_HOST}" /srv/files/plain.bin)

../k0sctl apply --config k0sctl.yaml --debug > changed.log 2>&1 || { cat changed.log; exit 1; }

printf %s "  - A changed URL source is downloaded again .. "
[ "$(remoteSha256 root@manager0 /root/srv/plain.bin)" = "${NEW_PLAIN_SHA256}" ]
printf %s "[etag] "
[ "$(remoteSha256 root@manager0 /root/srv/noetag.bin)" = "${NEW_PLAIN_SHA256}" ]
printf %s "[last-modified] "
assertRequestCount GET /files/bundle.bin "${bundle_gets}"
printf %s "[others untouched] "
echo "OK"

echo "* Corrupting a downloaded file in place"
plain_gets=$(serverRequests GET /files/plain.bin)
# Same length, different bytes: nothing the server says about the url can catch
# this, only the recorded state of the file itself.
remoteCommand root@manager0 "printf CORRUPT | dd of=/root/srv/plain.bin bs=1 seek=0 conv=notrunc 2>/dev/null"
../k0sctl apply --config k0sctl.yaml --debug > corrupt.log 2>&1 || { cat corrupt.log; exit 1; }

printf %s "  - A same-size local modification is downloaded again .. "
assertRequestCount GET /files/plain.bin "$((plain_gets + 1))"
printf %s "[downloaded] "
[ "$(remoteSha256 root@manager0 /root/srv/plain.bin)" = "${NEW_PLAIN_SHA256}" ]
printf %s "[restored] "
echo "OK"

echo "* Applying a file with a wrong sha256"
envsubst < k0sctl-files-badsum.yaml.tpl > k0sctl-badsum.yaml
printf %s "  - The apply fails and the file is not left behind .. "
if ../k0sctl apply --config k0sctl-badsum.yaml --debug > badsum.log 2>&1; then
  echo "FAIL"
  echo "  the apply succeeded with a wrong sha256"
  exit 1
fi
grep -q "checksum mismatch" badsum.log
printf %s "[failed] "
if remoteFileExist root@manager0 /root/srv/badsum.bin; then
  echo "FAIL"
  echo "  the mismatching file was left on the host"
  exit 1
fi
printf %s "[removed] "
# A download that does not end up at its destination must not leave its partial
# behind either: nothing continues it, and in a directory k0s reads it would be
# a truncated artifact for something else to trip over.
if remoteCommand root@manager0 "ls -a /root/srv | grep -q rigpart"; then
  echo "FAIL"
  echo "  a partial download was left on the host"
  remoteCommand root@manager0 "ls -la /root/srv"
  exit 1
fi
printf %s "[no partial] "
echo "OK"

echo "* Done"
