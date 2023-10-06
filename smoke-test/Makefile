
bootloose := $(shell which bootloose)
ifeq ($(bootloose),)
bootloose := $(shell go env GOPATH)/bin/bootloose
endif

envsubst := $(shell which envsubst)
ifeq ($(envsubst),)
$(error 'envsubst' NOT found in path, please install it and re-run)
endif

.PHONY: k0sctl
k0sctl:
	$(MAKE) -C .. k0sctl

$(bootloose):
	go install github.com/k0sproject/bootloose/cmd/bootloose@latest

id_rsa_k0s:
	ssh-keygen -t rsa -f ./id_rsa_k0s -N ""

smoke-basic: $(bootloose) id_rsa_k0s k0sctl
	./smoke-basic.sh

smoke-dynamic: $(bootloose) id_rsa_k0s k0sctl
	./smoke-dynamic.sh

smoke-files: $(bootloose) id_rsa_k0s k0sctl
	./smoke-files.sh

smoke-init: $(bootloose) id_rsa_k0s k0sctl
	./smoke-init.sh

smoke-upgrade: $(bootloose) id_rsa_k0s k0sctl
	./smoke-upgrade.sh

smoke-reset: $(bootloose) id_rsa_k0s k0sctl
	./smoke-reset.sh

smoke-os-override: $(bootloose) id_rsa_k0s k0sctl
	BOOTLOOSE_TEMPLATE=bootloose.yaml.osoverride.tpl K0SCTL_CONFIG=k0sctl-single.yaml OS_RELEASE_PATH=$(realpath os-release) OS_OVERRIDE="ubuntu" ./smoke-basic.sh

smoke-backup-restore: $(bootloose) id_rsa_k0s k0sctl
	./smoke-backup-restore.sh
