
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
	go install github.com/k0sproject/bootloose@latest

id_rsa_k0s:
	ssh-keygen -t rsa -f ./id_rsa_k0s -N ""

smoke-basic: $(bootloose) id_rsa_k0s k0sctl
	./smoke-basic.sh

smoke-basic-rootless: $(bootloose) id_rsa_k0s k0sctl
	./smoke-basic-rootless.sh

smoke-basic-openssh: $(bootloose) id_rsa_k0s k0sctl
	./smoke-basic-openssh.sh

smoke-dynamic: $(bootloose) id_rsa_k0s k0sctl
	./smoke-dynamic.sh

smoke-reinstall: $(bootloose) id_rsa_k0s k0sctl
	./smoke-reinstall.sh

smoke-files: $(bootloose) id_rsa_k0s k0sctl
	./smoke-files.sh

smoke-init: $(bootloose) id_rsa_k0s k0sctl
	./smoke-init.sh

smoke-upgrade: $(bootloose) id_rsa_k0s k0sctl
	./smoke-upgrade.sh

smoke-dryrun: $(bootloose) id_rsa_k0s k0sctl
	./smoke-dryrun.sh

smoke-reset: $(bootloose) id_rsa_k0s k0sctl
	./smoke-reset.sh

smoke-os-override: $(bootloose) id_rsa_k0s k0sctl
	BOOTLOOSE_TEMPLATE=bootloose.yaml.osoverride.tpl K0SCTL_CONFIG=k0sctl-single.yaml OS_RELEASE_PATH=$(realpath os-release) OS_OVERRIDE="ubuntu" ./smoke-basic.sh

smoke-downloadurl: $(bootloose) id_rsa_k0s k0sctl
	BOOTLOOSE_TEMPLATE=bootloose.yaml.single.tpl K0SCTL_CONFIG=k0sctl-downloadurl.yaml ./smoke-basic.sh

smoke-backup-restore: $(bootloose) id_rsa_k0s k0sctl
	./smoke-backup-restore.sh

smoke-controller-swap: $(bootloose) id_rsa_k0s k0sctl
	BOOTLOOSE_TEMPLATE=bootloose-controller-swap.yaml.tpl K0SCTL_CONFIG=k0sctl-controller-swap.yaml ./smoke-controller-swap.sh

%.iid: Dockerfile.%
	docker build --iidfile '$@' - < '$<'
