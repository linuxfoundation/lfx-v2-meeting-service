# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

# Variables
BINARY_NAME=meeting-api
BINARY_PATH=bin/$(BINARY_NAME)
GO_MODULE=github.com/linuxfoundation/lfx-v2-meeting-service
CMD_PATH=$(GO_MODULE)/cmd/meeting-api
DESIGN_MODULE=$(GO_MODULE)/design
GO_FILES=$(shell find . -name '*.go' -not -path './gen/*' -not -path './vendor/*')
GOA_VERSION=v3

# Docker variables
DOCKER_IMAGE=linuxfoundation/lfx-v2-meeting-service
DOCKER_TAG=latest

# Helm variables
HELM_CHART_PATH=./charts/lfx-v2-meeting-service
HELM_RELEASE_NAME=lfx-v2-meeting-service
HELM_NAMESPACE=lfx
HELM_VALUES_FILE=./charts/lfx-v2-meeting-service/values.local.yaml

# Build variables
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Test variables
TEST_FLAGS=-v -race -cover
TEST_TIMEOUT=5m

.PHONY: all help deps apigen build run debug test test-verbose test-coverage clean lint fmt check verify docker-build helm-install helm-install-local helm-templates helm-templates-local helm-uninstall

# Default target
all: clean deps apigen fmt lint test build

# Help target
help:
	@echo "Available targets:"
	@echo "  all            - Run clean, deps, apigen, fmt, lint, test, and build"
	@echo "  deps           - Install dependencies including goa CLI"
	@echo "  apigen         - Generate API code from design files"
	@echo "  build          - Build the binary"
	@echo "  run            - Run the service"
	@echo "  debug          - Run the service with debug logging"
	@echo "  test           - Run unit tests"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  clean          - Remove generated files and binaries"
	@echo "  lint           - Run golangci-lint"
	@echo "  fmt            - Format Go code"
	@echo "  check          - Run fmt and lint without modifying files"
	@echo "  verify         - Verify API generation is up to date"
	@echo "  docker-build   - Build Docker image"
	@echo "  helm-install   - Install Helm chart"
	@echo "  helm-install-local - Install Helm chart with local values file"
	@echo "  helm-templates   - Print templates for Helm chart"
	@echo "  helm-templates-local - Print templates for Helm chart with local values file"
	@echo "  helm-uninstall - Uninstall Helm chart"

# Install dependencies
deps:
	@echo "==> Installing dependencies..."
	go mod download
	go install goa.design/goa/$(GOA_VERSION)/cmd/goa@latest
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "==> Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	}

# Generate API code from design files
apigen: deps
	@echo "==> Generating API code..."
	goa gen $(DESIGN_MODULE)
	@echo "==> API generation complete"

# Build the binary
build: clean
	@echo "==> Building $(BINARY_NAME)..."
	@mkdir -p bin
	go build $(LDFLAGS) -o $(BINARY_PATH) $(CMD_PATH)
	@echo "==> Build complete: $(BINARY_PATH)"

# Run the service
run: apigen
	@echo "==> Running $(BINARY_NAME)..."
	go run $(LDFLAGS) $(CMD_PATH) .

# Run with debug logging
debug: apigen
	@echo "==> Running $(BINARY_NAME) in debug mode..."
	go run $(LDFLAGS) $(CMD_PATH) . -d

# Run tests
test:
	@echo "==> Running tests..."
	go test $(TEST_FLAGS) -timeout $(TEST_TIMEOUT) ./...

# Run tests with verbose output
test-verbose:
	@echo "==> Running tests (verbose)..."
	go test $(TEST_FLAGS) -v -timeout $(TEST_TIMEOUT) ./...

# Run tests with coverage
test-coverage:
	@echo "==> Running tests with coverage..."
	@mkdir -p coverage
	go test $(TEST_FLAGS) -timeout $(TEST_TIMEOUT) -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "==> Coverage report: coverage/coverage.html"

# Clean build artifacts
clean:
	@echo "==> Cleaning build artifacts..."
	@rm -rf bin/ coverage/
	@go clean -cache
	@echo "==> Clean complete"

# Run linter
lint:
	@echo "==> Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Run 'make deps' to install it."; \
		exit 1; \
	fi

# Format code
fmt:
	@echo "==> Formatting code..."
	@go fmt ./...
	@gofmt -s -w $(GO_FILES)

# Check license headers (basic validation - full check runs in CI)
.PHONY: license-check
license-check:
	@echo "==> Checking license headers (basic validation)..."
	@missing_files=$$(find . -name "*.go" \
		-not -path "./gen/*" \
		-not -path "./vendor/*" \
		-exec sh -c 'head -10 "$$1" | grep -q "Copyright The Linux Foundation and each contributor to LFX" && head -10 "$$1" | grep -q "SPDX-License-Identifier: MIT" || echo "$$1"' _ {} \;); \
	if [ -n "$$missing_files" ]; then \
		echo "Files missing required license headers:"; \
		echo "$$missing_files"; \
		echo "Required headers:"; \
		echo "  # Copyright The Linux Foundation and each contributor to LFX."; \
		echo "  # SPDX-License-Identifier: MIT"; \
		echo "Note: Full license validation runs in CI"; \
		exit 1; \
	fi
	@echo "==> Basic license header check passed"

# Check formatting and linting without modifying files
check:
	@echo "==> Checking code format..."
	@if [ -n "$$(gofmt -l $(GO_FILES))" ]; then \
		echo "The following files need formatting:"; \
		gofmt -l $(GO_FILES); \
		exit 1; \
	fi
	@echo "==> Code format check passed"
	@$(MAKE) lint
	@$(MAKE) license-check

# Verify that generated code is up to date
verify: apigen
	@echo "==> Verifying generated code is up to date..."
	@if [ -n "$$(git status --porcelain gen/)" ]; then \
		echo "Generated code is out of date. Run 'make apigen' and commit the changes."; \
		git status --porcelain gen/; \
		exit 1; \
	fi
	@echo "==> Generated code is up to date"

# Build Docker image
docker-build:
	@echo "==> Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f ./Dockerfile .
	@echo "==> Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Install Helm chart
helm-install:
	@echo "==> Installing Helm chart..."
	helm upgrade --force --install $(HELM_RELEASE_NAME) $(HELM_CHART_PATH) --namespace $(HELM_NAMESPACE)
	@echo "==> Helm chart installed: $(HELM_RELEASE_NAME)"

# Install Helm chart with local values file
helm-install-local:
	@echo "==> Installing Helm chart with local values file..."
	helm upgrade --force --install $(HELM_RELEASE_NAME) $(HELM_CHART_PATH) --namespace $(HELM_NAMESPACE) --values $(HELM_VALUES_FILE)
	@echo "==> Helm chart installed: $(HELM_RELEASE_NAME)"

# Print templates for Helm chart
helm-templates:
	@echo "==> Printing templates for Helm chart..."
	helm template $(HELM_RELEASE_NAME) $(HELM_CHART_PATH) --namespace $(HELM_NAMESPACE)
	@echo "==> Templates printed for Helm chart: $(HELM_RELEASE_NAME)"

# Print templates for Helm chart with local values file
helm-templates-local:
	@echo "==> Printing templates for Helm chart with local values file..."
	helm template $(HELM_RELEASE_NAME) $(HELM_CHART_PATH) --namespace $(HELM_NAMESPACE) --values $(HELM_VALUES_FILE)
	@echo "==> Templates printed for Helm chart: $(HELM_RELEASE_NAME)"

# Uninstall Helm chart
helm-uninstall:
	@echo "==> Uninstalling Helm chart..."
	helm uninstall $(HELM_RELEASE_NAME) --namespace $(HELM_NAMESPACE)
	@echo "==> Helm chart uninstalled: $(HELM_RELEASE_NAME)"
