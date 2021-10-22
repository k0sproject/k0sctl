footloose := $(shell which footloose)
ifeq ($(footloose),)
footloose := $(shell go env GOPATH)/bin/footloose
endif

envsubst := $(shell which envsubst)
ifeq ($(envsubst),)
$(error 'envsubst' NOT found in path, please install it and re-run)
endif

basic: $(footloose) id_rsa_k0s
	./smoke-basic.sh

$(footloose):
	go install github.com/weaveworks/footloose/...@0.6.3

id_rsa_k0s:
	ssh-keygen -t rsa -f ./id_rsa_k0s -N ""

init: $(footloose) id_rsa_k0s
	./smoke-init.sh

upgrade: $(footloose) id_rsa_k0s
	./smoke-upgrade.sh

reset: $(footloose) id_rsa_k0s
	./smoke-reset.sh

os-override: $(footloose) id_rsa_k0s
	FOOTLOOSE_TEMPLATE=footloose.yaml.osoverride.tpl OS_RELEASE_PATH=$(realpath os-release) OS_OVERRIDE="ubuntu" ./smoke-basic.sh

backup-restore: $(footloose) id_rsa_k0s
	./smoke-backup-restore.sh