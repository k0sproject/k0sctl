GO_SRCS := $(shell find . -type f -name '*.go' -a ! -name 'zz_generated*')
GIT_COMMIT = $(shell git rev-parse --short=7 HEAD)
K0SCTL_VERSION ?= $(or ${TAG_NAME},dev)
ifdef TAG_NAME
	ENVIRONMENT = "production"
endif
ENVIRONMENT ?= "development"

LD_FLAGS = -s -w -X github.com/k0sproject/k0sctl/version.Environment=$(ENVIRONMENT) -X github.com/k0sproject/k0sctl/version.GitCommit=$(GIT_COMMIT) -X github.com/k0sproject/k0sctl/version.Version=$(K0SCTL_VERSION)
BUILD_FLAGS = -trimpath -a -tags "netgo static_build" -installsuffix netgo -ldflags "$(LD_FLAGS) -extldflags '-static'"

bin/k0sctl-linux-x64: $(GO_SRCS)
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/k0sctl-linux-x64 main.go

bin/k0sctl-linux-arm64: $(GO_SRCS)
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o bin/k0sctl-linux-arm64 main.go

bin/k0sctl-linux-arm: $(GO_SRCS)
	GOOS=linux GOARCH=arm go build $(BUILD_FLAGS) -o bin/k0sctl-linux-arm main.go

bin/k0sctl-win-x64.exe: $(GO_SRCS)
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/k0sctl-win-x64.exe main.go

bin/k0sctl-darwin-x64: $(GO_SRCS)
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/k0sctl-darwin-x64 main.go

bin/k0sctl-darwin-arm64: $(GO_SRCS)
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o bin/k0sctl-darwin-arm64 main.go

bin/%.sha256: bin/%
	sha256sum -b $< | sed 's/bin\///' > $@.tmp
	mv $@.tmp $@

bins := k0sctl-linux-x64 k0sctl-linux-arm64 k0sctl-linux-arm k0sctl-win-x64.exe k0sctl-darwin-x64 k0sctl-darwin-arm64
checksums := $(addsuffix .sha256,$(bins))

.PHONY: build-all
build-all: $(addprefix bin/,$(bins) $(checksums))

k0sctl: $(GO_SRCS)
	go build $(BUILD_FLAGS) -o k0sctl main.go

.PHONY: clean
clean:
	rm -rf bin/

github_release := $(shell which github-release)
ifeq ($(github_release),)
github_release := $(shell go env GOPATH)/bin/github-release
endif

$(github_release):
	go get github.com/github-release/github-release/...@v0.10.0

upload-%: bin/% $(github_release)
	$(github_release) upload \
		--user k0sproject \
		--repo k0sctl \
		--tag "${TAG_NAME}" \
		--name "`basename $<`" \
		--file "$<"; \

.PHONY: upload
upload: $(addprefix upload-,$(bins) $(checksums))

smoketests := smoke-basic smoke-upgrade smoke-reset smoke-os-override smoke-init smoke-backup-restore
.PHONY: $(smoketests)
$(smoketests): k0sctl
	$(MAKE) -C smoke-test $@

golint := $(shell which golangci-lint)
ifeq ($(golint),)
golint := $(shell go env GOPATH)/bin/golangci-lint
endif

$(golint):
	go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.31.0

.PHONY: lint
lint: $(golint)
	$(golint) run ./...