GO ?= go
GO_TEST = $(GO) test -mod=vendor -tags "$(BUILDTAGS)"

# Version and commit are derived from the Go build info at compile time,
# see internal/version.
BUILDTAGS ?= exclude_graphdriver_btrfs containers_image_openpgp
GO_BUILD = $(GO) build -mod=vendor -tags "$(BUILDTAGS)"

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
	golangci-lint run --build-tags "$(BUILDTAGS)" ./...

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

container-%: container-image
	$(CONTAINER_RUNTIME) run --rm -v $(CURDIR):/src/skillctl $(CONTAINER_IMAGE) make $*

.PHONY: container-shell
container-shell: container-image
	$(CONTAINER_RUNTIME) run --rm -it -v $(CURDIR):/src/skillctl $(CONTAINER_IMAGE) bash
