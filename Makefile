GO ?= go
GO_BUILD = $(GO) build -mod=vendor
GO_TEST = $(GO) test -mod=vendor
GO_RUN = $(GO) run -mod=vendor

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
	$(GO_RUN) github.com/golangci/golangci-lint/v2/cmd/golangci-lint run ./...

.PHONY: vendor
vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

.PHONY: validate-vendor
validate-vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify
	./hack/tree_status.sh

# Container targets

.PHONY: container-image
container-image:
	$(CONTAINER_RUNTIME) build -f Containerfile -t $(CONTAINER_IMAGE) .

.PHONY: container-build
container-build: container-image
	$(CONTAINER_RUNTIME) run --rm -v $(CURDIR):/src/skillctl $(CONTAINER_IMAGE) make build

.PHONY: container-test
container-test: container-image
	$(CONTAINER_RUNTIME) run --rm -v $(CURDIR):/src/skillctl $(CONTAINER_IMAGE) make validate-vendor lint test

.PHONY: container-shell
container-shell: container-image
	$(CONTAINER_RUNTIME) run --rm -it -v $(CURDIR):/src/skillctl $(CONTAINER_IMAGE) bash
