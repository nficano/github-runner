BINARY     := github-runner
MODULE     := github.com/nficano/github-runner
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE       ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS    := -s -w \
              -X $(MODULE)/internal/version.Version=$(VERSION) \
              -X $(MODULE)/internal/version.Commit=$(COMMIT) \
              -X $(MODULE)/internal/version.Date=$(DATE)
GO         := go
GOFLAGS    ?=
GOTESTFLAGS ?= -race

.DEFAULT_GOAL := build

.PHONY: build
build: ## Build the binary
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/github-runner

.PHONY: install
install: ## Install to GOPATH/bin
	$(GO) install $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cmd/github-runner

.PHONY: test
test: ## Run unit tests with race detection
	$(GO) test $(GOTESTFLAGS) ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	$(GO) test $(GOTESTFLAGS) -tags=integration ./...

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format code
	gofumpt -w .
	goimports -w .

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: coverage
coverage: ## Generate coverage report
	$(GO) test $(GOTESTFLAGS) -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: bench
bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./...

.PHONY: generate
generate: ## Run go generate
	$(GO) generate ./...

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(BINARY):$(VERSION) .

.PHONY: goreleaser
goreleaser: ## Snapshot release build
	goreleaser release --snapshot --clean

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/ dist/ coverage.out coverage.html

.PHONY: tidy
tidy: ## Tidy go modules
	$(GO) mod tidy

.PHONY: check
check: vet lint test ## Run all checks

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
