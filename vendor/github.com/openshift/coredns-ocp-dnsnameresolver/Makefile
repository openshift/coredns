.PHONY: all
all: build-coredns

GO_TEST_FLAGS ?= -v

.PHONY: verify
verify: ## Run go verify against code
	hack/verify-gofmt.sh

.PHONY: test
test: ## Run go test against code
	go test -mod=mod $(GO_TEST_FLAGS) ./

.PHONY: build-coredns
build-coredns: ## Build coredns using the local branch of coredns-ocp-dnsnameresolver
	hack/build-coredns.sh

.PHONY: clean
clean:
	go clean
	rm -f coredns

CONTAINER_ENGINE ?= podman
CONTAINER_IMAGE ?= coredns

.PHONY: local-image
local-image:
ifndef CONTAINER_IMAGE
	echo "  Please pass a container image ... "
else ifeq ($(CONTAINER_ENGINE), buildah)
	echo "  - Building with buildah ... "
	buildah bud -t $(CONTAINER_IMAGE) .
else ifeq ($(CONTAINER_ENGINE), docker)
	echo "  - Building with docker ... "
	docker build -t $(CONTAINER_IMAGE) .
else ifeq ($(CONTAINER_ENGINE), podman)
	echo "  - Building with podman ... "
	podman build -t $(CONTAINER_IMAGE) .
else
	echo "  Please pass a container engine ... "
endif
