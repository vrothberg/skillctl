BINARY := bin/skillctl
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)
CONTAINER_IMAGE := skillctldev

.PHONY: build
build:
	go build -o $(BINARY) ./cmd/skillctl

.PHONY: test-unit
test-unit:
	go test ./...

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
	golangci-lint run ./...

# Container targets

.PHONY: container-image
container-image:
	$(CONTAINER_RUNTIME) build -f Containerfile -t $(CONTAINER_IMAGE) .

.PHONY: container-build
container-build: container-image
	$(CONTAINER_RUNTIME) run --rm -v $(CURDIR):/src/skillctl $(CONTAINER_IMAGE) make build

.PHONY: container-test
container-test: container-image
	$(CONTAINER_RUNTIME) run --rm -v $(CURDIR):/src/skillctl $(CONTAINER_IMAGE) make test

.PHONY: container-shell
container-shell: container-image
	$(CONTAINER_RUNTIME) run --rm -it -v $(CURDIR):/src/skillctl $(CONTAINER_IMAGE) bash
