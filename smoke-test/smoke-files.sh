#!/bin/bash

K0SCTL_TEMPLATE=${K0SCTL_TEMPLATE:-"k0sctl.yaml.tpl"}

set -e

. ./smoke.common.sh
trap cleanup EXIT

envsubst < k0sctl-files.yaml.tpl > k0sctl.yaml

deleteCluster
createCluster

remoteCommand() {
  local userhost="$1"
  shift
  local cmd="$@"
  footloose ssh "${userhost}" -- ${cmd}
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

echo "* Creating test user"
remoteCommand root@manager0 useradd test

echo "* Starting apply"
../k0sctl apply --config k0sctl.yaml --debug

echo "* Verifying uploads"
remoteCommand root@manager0 "apt-get update > /dev/null && apt-get install tree > /dev/null && tree -fp"

echo -n "  - Single file using destination file path and user:group .. "
remoteFileExist root@manager0 /root/singlefile/renamed.txt
echo -n "[exist]"
remoteCommand root@manager0 stat -c '%U:%G' /root/singlefile/renamed.txt | grep -q test:test
echo -n "[stat]"
echo "OK"

echo -n "  - Single file using destination dir .. "
remoteFileExist root@manager0 /root/destdir/toplevel.txt
echo "OK"

echo -n "  - PermMode 644 .. "
remoteFileExist root@manager0 /root/chmod/script.sh
echo -n "[exist]"
remoteCommand root@manager0 stat -c '%a' /root/chmod/script.sh | grep -q 644
echo -n "[stat] "
echo "OK"

echo -n "  - PermMode transfer .."
remoteFileExist root@manager0 /root/chmod_exec/script.sh
echo -n "[exist] "
remoteCommand root@manager0 stat -c '%a' /root/chmod_exec/script.sh | grep -q 744
echo -n "[stat] "
remoteCommand root@manager0 /root/chmod_exec/script.sh | grep -q hello
echo -n "[run] "
echo "OK"

echo -n "  - Directory using destination dir .. "
remoteFileExist root@manager0 /root/dir/toplevel.txt
echo -n "[1] "
remoteFileExist root@manager0 /root/dir/nested/nested.txt
echo -n "[2] "
remoteFileExist root@manager0 /root/dir/nested/exclude-on-glob
echo -n "[3] "
echo "OK"

echo -n "  - Glob using destination dir .. "
remoteFileExist root@manager0 /root/glob/toplevel.txt
echo -n "[1] "
remoteFileExist root@manager0 /root/glob/nested/nested.txt
echo -n "[2] "
! remoteFileExist root@manager0 /root/glob/nested/exclude-on-glob
echo -n "[3] "
remoteCommand root@manager0 stat -c '%a' /root/glob | grep -q 700
echo -n "[stat1]"
remoteCommand root@manager0 stat -c '%a' /root/glob/nested | grep -q 700
echo -n "[stat2]"
echo "OK"

echo -n "  - URL using destination file .. "
remoteFileExist root@manager0 /root/url/releases.json
echo -n "[exist] "
remoteFileContent root@manager0 /root/url/releases.json | grep -q html_url
echo -n "[content] "
echo "OK"

echo -n "  - URL using destination dir .. "
remoteFileExist root@manager0 /root/url_destdir/releases
echo -n "[exist] "
remoteFileContent root@manager0 /root/url_destdir/releases | grep -q html_url
echo -n "[content] "
echo "OK"

echo "* Done"

