
footloose := $(shell which footloose)
ifeq ($(footloose),)
footloose := $(shell go env GOPATH)/bin/footloose
endif

envsubst := $(shell which envsubst)
ifeq ($(envsubst),)
$(error 'envsubst' NOT found in path, please install it and re-run)
endif

$(footloose):
	go get github.com/weaveworks/footloose/...@0.6.3

id_rsa_k0s:
	ssh-keygen -t rsa -f ./id_rsa_k0s -N ""


smoke-basic: $(footloose) id_rsa_k0s
	./smoke-basic.sh

smoke-upgrade: $(footloose) id_rsa_k0s
	./smoke-upgrade.sh

smoke-reset: $(footloose) id_rsa_k0s
	./smoke-reset.sh

smoke-os-override: $(footloose) id_rsa_k0s
	FOOTLOOSE_TEMPLATE=footloose.yaml.osoverride.tpl OS_RELEASE_PATH=$(realpath os-release) K0SCTL_YAML=k0sctl_osoverride.yaml ./smoke-basic.sh