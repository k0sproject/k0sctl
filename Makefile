GO_SRCS := $(shell find . -type f -name '*.go' -a ! \( -name 'zz_generated*' -o -name '*_test.go' \))
GO_TESTS := $(shell find . -type f -name '*_test.go')
GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)
PREFIX = /usr/local

goreleaser := $(shell which goreleaser)
ifeq ($(goreleaser),)
goreleaser := $(shell go env GOPATH)/bin/goreleaser
endif

golint := $(shell which golangci-lint)
ifeq ($(golint),)
golint := $(shell go env GOPATH)/bin/golangci-lint
endif

k0sctl: $(GO_SRCS) .goreleaser.yml $(goreleaser)
	$(goreleaser) build --single-target --snapshot --rm-dist
	mv bin/k0sctl_$(GOOS)_$(GOARCH)/k0sctl .

.PHONY: clean
clean:
	rm -rf bin/

$(golint):
	go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.31.0

$(goreleaser):
	go install github.com/goreleaser/goreleaser@latest

.PHONY: lint
lint: $(golint)
	$(golint) run ./...

.PHONY: build-all
build-all: $(goreleaser)
	$(goreleaser) build --snapshot --rm-dist

gotest := $(shell which gotest)
ifeq ($(gotest),)
gotest := go test
endif

.PHONY: test
test: $(GO_SRCS) $(GO_TESTS)
	$(gotest) -v ./...

.PHONY: install
install: k0sctl
	install -d $(DESTDIR)$(PREFIX)/bin/
	install -m 755 k0sctl $(DESTDIR)$(PREFIX)/bin/