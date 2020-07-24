include .bingo/Variables.mk

OS   ?= $(shell uname -s | tr '[A-Z]' '[a-z]')
ARCH ?= $(shell uname -m)

VERSION         := $(strip $(shell [ -d .git ] && git describe --always --tags --dirty))
BUILD_DATE      := $(shell date -u +"%Y-%m-%d")
BUILD_TIMESTAMP := $(shell date -u +"%Y-%m-%dT%H:%M:%S%Z")
VCS_REF         := $(strip $(shell [ -d .git ] && git rev-parse --short HEAD))
BRANCH          := $(strip $(shell git rev-parse --abbrev-ref HEAD))
USER            ?= $(shell id -u -n)
HOST            ?= $(shell hostname)

LDFLAGS := -s -w \
	-X github.com/prometheus/common/version.Version="$(VERSION)" \
	-X github.com/prometheus/common/version.Revision="$(VCS_REF)" \
	-X github.com/prometheus/common/version.Branch="$(BRANCH)" \
	-X github.com/prometheus/common/version.BuildUser="${USER}"@"${HOST}" \
	-X github.com/prometheus/common/version.BuildDate="$(BUILD_TIMESTAMP)"

GO_FILES   := $(shell find . -name \*.go -print)
M          =  $(shell printf "\033[34;1m▶\033[0m")
BIN_DIR    ?= $(PWD)/bin
SHELLCHECK ?= $(BIN_DIR)/shellcheck

PROMETHEUS ?= $(BIN_DIR)/prometheus
PROMETHEUS_VERSION ?= 2.20.0

LOKI ?= $(BIN_DIR)/loki
LOKI_VERSION ?= 1.5.0
PROMTAIL ?= $(BIN_DIR)/promtail

.PHONY: all
all: format build

.PHONY: demo
demo: ## Runs demo
demo: setup build ; $(info $(M) running demo)
	PATH=$$PATH:$(BIN_DIR) PROMETHEUS=$(PROMETHEUS) ./demo/local.sh

.PHONY: build
build: ## Build binaries
build: deps ${BIN_DIR}/backend ${BIN_DIR}/proxy

backend: ## Build backend binary
${BIN_DIR}/backend: cmd/backend/main.go
	@go build -a -tags netgo -ldflags '${LDFLAGS}' -o $@ $?

proxy: ## Build proxy binary
${BIN_DIR}/proxy: cmd/proxy/main.go
	@go build -a -tags netgo -ldflags '${LDFLAGS}' -o $@ $?

.PHONY: container
container: ## Builds latest container images
container: container-backend container-proxy

.PHONY: container-push
container-push: ## Pushes latest container images to repository
container-push: container ; $(info $(M) running container-push )
	@docker push kakkoyun/observable-remote-write-backend:$(BRANCH)-$(BUILD_DATE)-$(VERSION)
	@docker push kakkoyun/observable-remote-write-backend:latest
	@docker push kakkoyun/observable-remote-write-proxy:$(BRANCH)-$(BUILD_DATE)-$(VERSION)
	@docker push kakkoyun/observable-remote-write-proxy:latest

.PHONY: container-backend
container-backend: docker/backend.dockerfile
	@docker build -t kakkoyun/observable-remote-write-backend:latest -f $? .
	@docker tag kakkoyun/observable-remote-write-backend:latest kakkoyun/observable-remote-write-backend:$(BRANCH)-$(BUILD_DATE)-$(VERSION)

.PHONY: container-proxy
container-proxy: docker/proxy.dockerfile
	@docker build -t kakkoyun/observable-remote-write-proxy:latest -f $? .
	@docker tag kakkoyun/observable-remote-write-proxy:latest kakkoyun/observable-remote-write-proxy:$(BRANCH)-$(BUILD_DATE)-$(VERSION)

.PHONY: deps
deps: ## Install dependencies
deps: go.mod go.sum
	@go mod tidy
	@go mod verify

.PHONY: setup
setup: ## Setups dev environment
setup: deps ; $(info $(M) running setup for development )
	make $(BINGO) $(GOTEST) $(LICHE) $(GOLANGCI_LINT) $(UP) $(CONPROF) $(PROMETHEUS) $(LOKI) $(PROMTAIL) $(JAEGER)

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

.PHONY: test-integration
test-integration: ## Runs integration tests
test-integration: setup build ; $(info $(M) running integration tests)
	PATH=$$PATH:$$(BIN_DIR) ./test/integration.sh

.PHONY: test-unit
test-unit: ## Runs unit tests
test-unit: $(GOTEST) ; $(info $(M) running unit tests)
	-$(GOTEST) -race -short -cover -failfast ./...

.PHONY: test
test: ## Runs tests
test: $(GOTEST) test-unit test-integration ; $(info $(M) running tests)

.PHONY: shellcheck
shellcheck: ## Check shell scripts
shellcheck: $(SHELLCHECK)
	$(SHELLCHECK) $(shell find . -type f -name "*.sh" -not -path "*vendor*" -not -path "${TMP_DIR}/*")

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(SHELLCHECK): | $(BIN_DIR) ; $(info $(M) downloading Shellcheck)
	@echo "Downloading Shellcheck"
	curl -sNL "https://github.com/koalaman/shellcheck/releases/download/stable/shellcheck-stable.$(OS).$(ARCH).tar.xz" | tar --strip-components=1 -xJf - -C $(BIN_DIR)

$(PROMETHEUS): | $(BIN_DIR)
	@echo "Downloading Prometheus"
	curl -L "https://github.com/prometheus/prometheus/releases/download/v$(PROMETHEUS_VERSION)/prometheus-$(PROMETHEUS_VERSION).$$(go env GOOS)-$$(go env GOARCH).tar.gz" | tar --strip-components=1 -xzf - -C $(BIN_DIR)

$(LOKI): | $(BIN_DIR)
	@echo "Downloading Loki"
	(loki_pkg="loki-$$(go env GOOS)-$$(go env GOARCH)" && \
	cd $(BIN_DIR) && curl -O -L "https://github.com/grafana/loki/releases/download/v$(LOKI_VERSION)/$$loki_pkg.zip" && \
	unzip $$loki_pkg.zip && \
	mv $$loki_pkg loki && \
	rm $$loki_pkg.zip)

$(PROMTAIL): | $(BIN_DIR)
	@echo "Downloading Promtail"
	(promtail_pkg="promtail-$$(go env GOOS)-$$(go env GOARCH)" && \
	cd $(BIN_DIR) && curl -O -L "https://github.com/grafana/loki/releases/download/v$(LOKI_VERSION)/$$promtail_pkg.zip" && \
	unzip $$promtail_pkg.zip && \
	mv $$promtail_pkg promtail && \
	rm $$promtail_pkg.zip)

.PHONY: help
help: ## Shows this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m\t %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
