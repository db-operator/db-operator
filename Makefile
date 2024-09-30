# ---------------------------------------------------------------------
# -- Image URL to use all building/pushing image targets
# ---------------------------------------------------------------------
IMAGE_TAG ?= my-db-operator:v1.0.0-dev
# ---------------------------------------------------------------------
# -- ENVTEST_K8S_VERSION refers to the version of kubebuilder assets 
# --  to be downloaded by envtest binary.
# ---------------------------------------------------------------------
ENVTEST_K8S_VERSION = 1.28.0
# ---------------------------------------------------------------------
# -- CONTROLLER_GEN_VERSION a version of the controller-get tool
# ---------------------------------------------------------------------
CONTROLLER_GEN_VERSION = v0.14.0
# ---------------------------------------------------------------------
# -- K8s version to start a local kubernetes
# ---------------------------------------------------------------------
K8S_VERSION ?= v1.22.3
# ---------------------------------------------------------------------
# -- Helper tools version
# ---------------------------------------------------------------------
GOLANGCI_LINT_VERSION ?= v1.61.0
# ---------------------------------------------------------------------
# -- Get the currently used golang install path 
# --  (in GOPATH/bin, unless GOBIN is set)
# ---------------------------------------------------------------------
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif
# ---------------------------------------------------------------------
# -- Which container tool to use
# ---------------------------------------------------------------------
CONTAINER_TOOL ?= docker
# ---------------------------------------------------------------------
# -- Which compose tool to use
# -- For example
# -- $ export COMPOSE_TOOL="nerdctl compose"
# ---------------------------------------------------------------------
COMPOSE_TOOL ?= docker-compose
# ---------------------------------------------------------------------
# -- It's required when you want to use k3s and nerdctl
# -- $ export CONTAINER_TOOL_NAMESPACE_ARG="--namespace k8s.io"
# ---------------------------------------------------------------------
CONTAINER_TOOL_NAMESPACE_ARG ?=
# -- Set additional argumets to container tool
# -- To use, set an environment variable:
# -- $ export CONTAINER_TOOL_ARGS="--build-arg GOARCH=arm64 --platform=linux/arm64"
# ---------------------------------------------------------------------
CONTAINER_TOOL_ARGS ?=
# ---------------------------------------------------------------------
# -- A path to store binaries that are used in the Makefile
# ---------------------------------------------------------------------
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# ---------------------------------------------------------------------
# -- Rules
# ---------------------------------------------------------------------
help: ## show this help
	@echo 'usage: make [target] ...'
	@echo ''
	@echo 'targets:'
	@grep -E '^(.+)\:\ .*##\ (.+)' ${MAKEFILE_LIST} | sort | sed 's/:.*##/#/' | column -t -c 2 -s '#'

.PHONY: build
build: ## Build a container
	$(CONTAINER_TOOL) build ${CONTAINER_TOOL_ARGS} -t ${IMAGE_TAG} . ${CONTAINER_TOOL_NAMESPACE_ARG} --build-arg="OPERATOR_VERSION=v1.0.0-dev"
	$(CONTAINER_TOOL) save ${CONTAINER_TOOL_NAMESPACE_ARG} ${IMAGE_TAG} -o my-image.tar

# ---------------------------------------------------------------------
# -- Go related rules
# ---------------------------------------------------------------------
lint: ## lint go code
	echo "desired golangci-lint version is ${GOLANGCI_LINT_VERSION}"
	@go mod download
	test -s $(LOCALBIN)/golangci-lint || GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}
	$(LOCALBIN)/golangci-lint --version
	$(LOCALBIN)/golangci-lint run ./...  --timeout 240s

fmt: ## Format go code
	@test -s $(LOCALBIN)/gofumpt || GOBIN=$(LOCALBIN) go install mvdan.cc/gofumpt@latest
	@$(LOCALBIN)/gofumpt -l -w .

vet: ## go vet to find issues
	@go vet ./...

# ---------------------------------------------------------------------
# -- Tests are not passing when GOARCH is not amd64, so it's hardcoded
# ---------------------------------------------------------------------
unit: ## run go unit tests
	GOARCH=amd64 go test -tags tests -run "TestUnit" ./... -v

test: ## run go all test
	$(COMPOSE_TOOL) down
	$(COMPOSE_TOOL) up -d
	$(COMPOSE_TOOL) restart sqladmin
	sleep 10
	GOARCH=amd64 go test -count=1 -tags tests ./... -v -cover
	$(COMPOSE_TOOL) down

# ---------------------------------------------------------------------
# -- Kubebuilder realted rule
# ---------------------------------------------------------------------
addexamples: ## add examples via kubectl create -f examples/
	cd ./examples/; ls | while read line; do kubectl apply -f $$line; done

manifests: controller-gen ## generate custom resource definitions
	$(LOCALBIN)/controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
generate: controller-gen ## generate supporting code for custom resource types
	$(LOCALBIN)/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@${CONTROLLER_GEN_VERSION}

.PHONY: envtest
envtest: ## Download envtest-setup locally if necessary.
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	${LOCALBIN}/setup-envtest use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path

# ---------------------------------------------------------------------
# -- Additional helpers
# ---------------------------------------------------------------------

desired_go_version: ## get a desired go version from the Containerfile
	@grep -o -P '(?<=golang:).*(?=-alpine)' Containerfile

k3s_mac_lima_create: ## create local k8s using lima
	limactl start --tty=false ./resources/lima/k3s.yaml

k3s_mac_lima_start: ## start local lima k8s
	limactl start k3s

k3s_mac_deploy: build k3s_mac_image ## build image and import image to local lima k8s

k3s_mac_image: ## import built image to local lima k8s
	limactl copy my-image.tar k3s:/tmp/db.tar
	limactl shell k3s sudo k3s ctr images import --all-platforms /tmp/db.tar
	limactl shell k3s rm -f /tmp/db.tar

k3d_setup: k3d_install k3d_image ## install k3d and import image to your k3d cluster

k3d_install: ## install k3d cluster locally
	@curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
	@k3d cluster create myk3s -i rancher/k3s:$(K8S_VERSION)-k3s1
	@kubectl get pod

k3d_image: build ## rebuild the docker images and upload into your k3d cluster
	@k3d image import my-image.tar -c myk3s
