include .bingo/Variables.mk

OS   ?= $(shell uname -s | tr '[A-Z]' '[a-z]')
ARCH ?= $(shell uname -m)

VERSION    := $(strip $(shell [ -d .git ] && git describe --always --tags --dirty))
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%S%Z")
VCS_REF    := $(strip $(shell [ -d .git ] && git rev-parse --short HEAD))
BRANCH     := $(strip $(shell git rev-parse --abbrev-ref HEAD))
VERSION    := $(strip $(shell [ -d .git ] && git describe --always --tags --dirty))
USER       ?= $(shell id -u -n)
HOST       ?= $(shell hostname)

LDFLAGS := -s -w \
	-X github.com/prometheus/common/version.Version="$(VERSION)" \
	-X github.com/prometheus/common/version.Revision="$(VCS_REF)" \
	-X github.com/prometheus/common/version.Branch="$(BRANCH)" \
	-X github.com/prometheus/common/version.BuildUser="${USER}"@"${HOST}" \
	-X github.com/prometheus/common/version.BuildDate="$(BUILD_DATE)"

GO_FILES   := $(shell find . -name \*.go -print)
M          =  $(shell printf "\033[34;1m▶\033[0m")
BIN_DIR    ?= ./tmp/bin
SHELLCHECK ?= $(BIN_DIR)/shellcheck

.PHONY: all
all: format build

.PHONY: build
build: ## Build binaries
build: deps backend proxy

backend: ## Build backend binary
backend: cmd/backend/main.go
	@go build -a -tags netgo -ldflags '${LDFLAGS}' -o $@ $?

proxy: ## Build proxy binary
proxy: cmd/proxy/main.go
	@go build -a -tags netgo -ldflags '${LDFLAGS}' -o $@ $?

.PHONY: container
container: ## Builds latest container images
container: container-backend container-proxy

.PHONY: container-push
container-push: ## Pushes latest container images to repository
container-push: container ; $(info $(M) running container-push )
	@docker push kakkoyun/observable-remote-write-backend:$(VERSION)
	@docker push kakkoyun/observable-remote-write-proxy:$(VERSION)

.PHONY: container-backend
container-backend: docker/backend.dockerfile
	@docker build -t observable-remote-write-backend .

.PHONY: container-proxy
container-proxy: docker/proxy.dockerfile
	@docker build -t observable-remote-write-proxy .

.PHONY: deps
deps: ## Install dependencies
deps: go.mod go.sum
	@go mod tidy
	@go mod verify

.PHONY: setup
setup: ## Setups dev environment
setup: deps ; $(info $(M) running setup for development )
	make $(GOTEST) $(LICHE) $(GOLANGCI_LINT) $(BINGO)

.PHONY: format
format: ## Runs gofmt and goimports
format: ; $(info $(M) running format )
	@gofmt -w -s $(GO_FILES)
	@goimports -w $(GO_FILES)

.PHONY: lint
lint: ## Runs golangci-lint analysis
lint: $(GOLANGCI_LINT) shellcheck ; $(info $(M) running lint )
	# Check .golangci.yml for configuration
	$(GOLANGCI_LINT) run -v --enable-all --skip-dirs tmp -c .golangci.yml

.PHONY: fix
fix: ## Runs golangci-lint fix
fix: $(GOLANGCI_LINT) format ; $(info $(M) running fix )
	$(GOLANGCI_LINT) run --fix --enable-all --skip-dirs tmp -c .golangci.yml

.PHONY: test
test: ## Runs tests
test: $(GOTEST) ; $(info $(M) running test)
	-$(GOTEST) -race -short -cover -failfast ./...

.PHONY: shellcheck
shellcheck: ## Check shell scripts
shellcheck: $(SHELLCHECK)
	$(SHELLCHECK) $(shell find . -type f -name "*.sh" -not -path "*vendor*" -not -path "${TMP_DIR}/*")

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(SHELLCHECK): $(BIN_DIR) ; $(info $(M) downloading Shellcheck)
	curl -sNL "https://github.com/koalaman/shellcheck/releases/download/stable/shellcheck-stable.$(OS).$(ARCH).tar.xz" | tar --strip-components=1 -xJf - -C $(BIN_DIR)

.PHONY: help
help: ## Shows this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m\t %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
