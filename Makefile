PKGS =  $(PKG) $(shell env GO111MODULE=on $(GO) list ./...)
GO = go

export GO111MODULE=on
export GOPRIVATE=github.com/k0sproject/* ## Allows us to pull dependencies from the private repo

help:
	@grep -hE '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

build: ## Builds the binary
	$(GO) build -o k0sctl

install: build ## Installs k0s to $GOPATH/bin
	mv ./k0sctl $(GOPATH)/bin/


lint: ## Run golint on all source files
	$(GO) vet ./...
	golint -set_exit_status ./...
	
fmt: lint ; $(info $(M) running gofmt…) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)

