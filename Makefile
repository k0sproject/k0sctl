GO_SRCS := $(shell find . -type f -name '*.go' -a ! \( -name 'zz_generated*' -o -name '*_test.go' \))
GO_TESTS := $(shell find . -type f -name '*_test.go')
TAG_NAME = $(shell git diff-index --quiet HEAD && git name-rev --name-only --tags --no-undefined HEAD 2>/dev/null | sed -n 's/^\([^^~]\{1,\}\)\(\^0\)\{0,1\}$/\1/p')
GIT_COMMIT = $(shell git rev-parse --short=7 HEAD)
K0SCTL_VERSION = $(or ${TAG_NAME},dev)
ifdef TAG_NAME
	ENVIRONMENT = production
endif
ENVIRONMENT ?= development
PREFIX = /usr/local

LD_FLAGS = -s -w -X github.com/k0sproject/k0sctl/version.Environment=$(ENVIRONMENT) -X github.com/k0sproject/k0sctl/version.GitCommit=$(GIT_COMMIT) -X github.com/k0sproject/k0sctl/version.Version=$(K0SCTL_VERSION)
BUILD_FLAGS = -trimpath -a -tags "netgo,osusergo,static_build" -installsuffix netgo -ldflags "$(LD_FLAGS) -extldflags '-static'"

k0sctl: $(GO_SRCS)
	go build $(BUILD_FLAGS) -o k0sctl main.go

bin/k0sctl-linux-x64: $(GO_SRCS)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o bin/k0sctl-linux-x64 main.go

bin/k0sctl-linux-arm64: $(GO_SRCS)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o bin/k0sctl-linux-arm64 main.go

bin/k0sctl-linux-arm: $(GO_SRCS)
	GOOS=linux GOARCH=arm CGO_ENABLED=0 go build $(BUILD_FLAGS) -o bin/k0sctl-linux-arm main.go

bin/k0sctl-win-x64.exe: $(GO_SRCS)
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/k0sctl-win-x64.exe main.go

bin/k0sctl-darwin-x64: $(GO_SRCS)
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/k0sctl-darwin-x64 main.go

bin/k0sctl-darwin-arm64: $(GO_SRCS)
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o bin/k0sctl-darwin-arm64 main.go

bins := k0sctl-linux-x64 k0sctl-linux-arm64 k0sctl-linux-arm k0sctl-win-x64.exe k0sctl-darwin-x64 k0sctl-darwin-arm64

bin/checksums.txt: $(addprefix bin/,$(bins))
	sha256sum -b $(addprefix bin/,$(bins)) | sed 's:bin/::' > $@

.PHONY: build-all
build-all: $(addprefix bin/,$(bins)) bin/checksums.txt

.PHONY: clean
clean:
	rm -rf bin/ k0sctl

github_release := $(shell which github-release)
ifeq ($(github_release),)
github_release := $(shell go env GOPATH)/bin/github-release
endif

$(github_release):
	go install github.com/github-release/github-release/...@latest

upload-%: bin/% $(github_release)
	$(github_release) upload \
		--user k0sproject \
		--repo k0sctl \
		--tag "${TAG_NAME}" \
		--name "`basename $<`" \
		--file "$<"; \

.PHONY: upload
upload: $(addprefix upload-,$(bins)) bin/checksums.txt

smoketests := smoke-basic smoke-files smoke-upgrade smoke-reset smoke-os-override smoke-init smoke-backup-restore
.PHONY: $(smoketests)
$(smoketests): k0sctl
	$(MAKE) -C smoke-test $@

golint := $(shell which golangci-lint)
ifeq ($(golint),)
golint := $(shell go env GOPATH)/bin/golangci-lint
endif

gotest := $(shell which gotest)
ifeq ($(gotest),)
gotest := go test
endif

$(golint):
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: lint
lint: $(golint)
	$(golint) run ./...

.PHONY: test
test: $(GO_SRCS) $(GO_TESTS)
	$(gotest) -v ./...

.PHONY: install
install: k0sctl
	install -d $(DESTDIR)$(PREFIX)/bin/
	install -m 755 k0sctl $(DESTDIR)$(PREFIX)/bin/