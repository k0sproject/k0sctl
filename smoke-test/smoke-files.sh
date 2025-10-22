#!/usr/bin/env sh

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

echo "* Done"
