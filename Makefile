GO_SRCS := $(shell find . -type f -name '*.go' -a ! \( -name 'zz_generated*' -o -name '*_test.go' \))
GO_TESTS := $(shell find . -type f -name '*_test.go')
TAG_NAME = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_COMMIT = $(shell git rev-parse --short=7 HEAD)
ifdef TAG_NAME
	ENVIRONMENT = production
endif
ENVIRONMENT ?= development
PREFIX = /usr/local

LD_FLAGS = -s -w -X github.com/k0sproject/k0sctl/version.Environment=$(ENVIRONMENT) -X github.com/carlmjohnson/versioninfo.Revision=$(GIT_COMMIT) -X github.com/carlmjohnson/versioninfo.Version=$(TAG_NAME)
BUILD_FLAGS = -trimpath -a -tags "netgo,osusergo,static_build" -installsuffix netgo -ldflags "$(LD_FLAGS) -extldflags '-static'"

k0sctl: $(GO_SRCS)
	go build $(BUILD_FLAGS) -o k0sctl main.go

bin/k0sctl-linux-amd64: $(GO_SRCS)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o bin/k0sctl-linux-amd64 main.go

bin/k0sctl-linux-arm64: $(GO_SRCS)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o bin/k0sctl-linux-arm64 main.go

bin/k0sctl-linux-arm: $(GO_SRCS)
	GOOS=linux GOARCH=arm CGO_ENABLED=0 go build $(BUILD_FLAGS) -o bin/k0sctl-linux-arm main.go

bin/k0sctl-win-amd64.exe: $(GO_SRCS)
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/k0sctl-win-amd64.exe main.go

bin/k0sctl-darwin-amd64: $(GO_SRCS)
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o bin/k0sctl-darwin-amd64 main.go

bin/k0sctl-darwin-arm64: $(GO_SRCS)
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o bin/k0sctl-darwin-arm64 main.go

bins := k0sctl-linux-amd64 k0sctl-linux-arm64 k0sctl-linux-arm k0sctl-win-amd64.exe k0sctl-darwin-amd64 k0sctl-darwin-arm64

bin/checksums.txt: $(addprefix bin/,$(bins))
	sha256sum -b $(addprefix bin/,$(bins)) | sed 's/bin\///' > $@

bin/checksums.md: bin/checksums.txt
	@echo "### SHA256 Checksums" > $@
	@echo >> $@
	@echo "\`\`\`" >> $@
	@cat $< >> $@
	@echo "\`\`\`" >> $@

.PHONY: build-all
build-all: $(addprefix bin/,$(bins)) bin/checksums.md

.PHONY: clean
clean:
	rm -rf bin/ k0sctl

smoketests := smoke-basic smoke-basic-rootless smoke-files smoke-upgrade smoke-reset smoke-os-override smoke-init smoke-backup-restore smoke-dynamic smoke-basic-openssh smoke-dryrun smoke-downloadurl smoke-controller-swap smoke-reinstall
.PHONY: $(smoketests)
$(smoketests): k0sctl
	$(MAKE) -C smoke-test $@

golint := $(shell which golangci-lint 2>/dev/null)
ifeq ($(golint),)
golint := $(shell go env GOPATH)/bin/golangci-lint
endif

$(golint):
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: lint
lint: $(golint)
	$(golint) run ./...

.PHONY: test
test: $(GO_SRCS) $(GO_TESTS)
	go test -v ./...

.PHONY: install
install: k0sctl
	install -d $(DESTDIR)$(PREFIX)/bin/
	install -m 755 k0sctl $(DESTDIR)$(PREFIX)/bin/
