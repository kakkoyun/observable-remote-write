# Auto generated binary variables helper managed by https://github.com/bwplotka/bingo v0.2.2. DO NOT EDIT.
# All tools are designed to be build inside $GOBIN.
GOPATH ?= $(shell go env GOPATH)
GOBIN  ?= $(firstword $(subst :, ,${GOPATH}))/bin
GO     ?= $(shell which go)

# Bellow generated variables ensure that every time a tool under each variable is invoked, the correct version
# will be used; reinstalling only if needed.
# For example for goimports variable:
#
# In your main Makefile (for non array binaries):
#
#include .bingo/Variables.mk # Assuming -dir was set to .bingo .
#
#command: $(GOIMPORTS)
#	@echo "Running goimports"
#	@$(GOIMPORTS) <flags/args..>
#
GOIMPORTS := $(GOBIN)/goimports-v0.0.0-20200717024301-6ddee64345a6
$(GOIMPORTS): .bingo/goimports.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/goimports-v0.0.0-20200717024301-6ddee64345a6"
	@cd .bingo && $(GO) build -modfile=goimports.mod -o=$(GOBIN)/goimports-v0.0.0-20200717024301-6ddee64345a6 "golang.org/x/tools/cmd/goimports"

GOLANGCI_LINT := $(GOBIN)/golangci-lint-v1.27.0
$(GOLANGCI_LINT): .bingo/golangci-lint.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/golangci-lint-v1.27.0"
	@cd .bingo && $(GO) build -modfile=golangci-lint.mod -o=$(GOBIN)/golangci-lint-v1.27.0 "github.com/golangci/golangci-lint/cmd/golangci-lint"

GOTEST := $(GOBIN)/gotest-v0.0.5
$(GOTEST): .bingo/gotest.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/gotest-v0.0.5"
	@cd .bingo && $(GO) build -modfile=gotest.mod -o=$(GOBIN)/gotest-v0.0.5 "github.com/rakyll/gotest"

LICHE := $(GOBIN)/liche-v0.0.0-20200229003944-f57a5d1c5be4
$(LICHE): .bingo/liche.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/liche-v0.0.0-20200229003944-f57a5d1c5be4"
	@cd .bingo && $(GO) build -modfile=liche.mod -o=$(GOBIN)/liche-v0.0.0-20200229003944-f57a5d1c5be4 "github.com/raviqqe/liche"

UP := $(GOBIN)/up-v0.0.0-20200615121732-d763595ede50
$(UP): .bingo/up.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/up-v0.0.0-20200615121732-d763595ede50"
	@cd .bingo && $(GO) build -modfile=up.mod -o=$(GOBIN)/up-v0.0.0-20200615121732-d763595ede50 "github.com/observatorium/up/cmd/up"

