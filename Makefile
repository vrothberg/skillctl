GO ?= go
GO_BUILD = $(GO) build -mod=vendor
GO_TEST = $(GO) test -mod=vendor

BINARY := bin/skillctl
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)
CONTAINER_IMAGE := skillctldev

.PHONY: build
build:
	$(GO_BUILD) -o $(BINARY) ./cmd/skillctl

.PHONY: test-unit
test-unit:
	$(GO_TEST) ./...

.PHONY: test-integration
test-integration: build
	bats test/*.bats

.PHONY: test
test: test-unit test-integration

.PHONY: clean
clean:
	rm -rf bin/

.PHONY: lint
lint:
	cd tools && $(GO) build -mod=vendor -o ../bin/golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint
	./bin/golangci-lint run ./...

.PHONY: vendor
vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

.PHONY: validate-vendor
validate-vendor: vendor
	./hack/tree_status.sh

# Container targets

.PHONY: container-image
container-image:
	$(CONTAINER_RUNTIME) build -f Containerfile -t $(CONTAINER_IMAGE) .

.PHONY: container-test
container-test: container-image
	$(CONTAINER_RUNTIME) run --rm -v $(CURDIR):/src/skillctl $(CONTAINER_IMAGE) make test

.PHONY: container-shell
container-shell: container-image
	$(CONTAINER_RUNTIME) run --rm -it -v $(CURDIR):/src/skillctl $(CONTAINER_IMAGE) bash
