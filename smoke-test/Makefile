
footloose := $(shell which footloose)
ifeq ($(footloose),)
footloose := $(shell go env GOPATH)/bin/footloose
endif

$(footloose):
	go get github.com/weaveworks/footloose/...@0.6.3

id_rsa_k0s:
	ssh-keygen -t rsa -f ./id_rsa_k0s -N ""


smoke-basic: $(footloose) id_rsa_k0s
	./smoke-basic.sh