.PHONY: help build test test-coverage test-race lint fmt vet security clean deps docker-build docker-run
.DEFAULT_GOAL := help

# Colors for output
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
BLUE := \033[34m
RESET := \033[0m

# Build variables
BINARY_NAME := hotel-reviews
DOCKER_IMAGE := gkbiswas/hotel-reviews
VERSION := $(shell git describe --tags --always --dirty)
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Coverage settings
COVERAGE_THRESHOLD := 42
COVERAGE_TARGET := 80

help: ## Display this help message
	@echo "$(BLUE)Hotel Reviews Microservice$(RESET)"
	@echo "============================="
	@echo ""
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-20s$(RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Install dependencies
	@echo "$(YELLOW)Installing dependencies...$(RESET)"
	go mod download
	go mod verify
	go mod tidy

build: deps ## Build the application
	@echo "$(YELLOW)Building application...$(RESET)"
	CGO_ENABLED=0 GOOS=linux go build \
		-ldflags="-X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)" \
		-o bin/$(BINARY_NAME) \
		./cmd/api

test: ## Run unit tests
	@echo "$(YELLOW)Running unit tests...$(RESET)"
	go test -v -short ./...

test-race: ## Run tests with race detection
	@echo "$(YELLOW)Running tests with race detection...$(RESET)"
	go test -race -short ./...

test-coverage: ## Run tests with coverage
	@echo "$(YELLOW)Running tests with coverage...$(RESET)"
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./... -short
	go tool cover -html=coverage.out -o coverage.html
	@$(MAKE) check-coverage

check-coverage: ## Check if coverage meets threshold
	@echo "$(YELLOW)Checking test coverage...$(RESET)"
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Current Coverage: $${COVERAGE}%"; \
	echo "Minimum Required: $(COVERAGE_THRESHOLD)%"; \
	echo "Long-term Target: $(COVERAGE_TARGET)%"; \
	if [ $$(echo "$${COVERAGE} < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
		echo "$(RED)❌ Coverage $${COVERAGE}% is below minimum threshold $(COVERAGE_THRESHOLD)%$(RESET)"; \
		exit 1; \
	fi; \
	if [ $$(echo "$${COVERAGE} < $(COVERAGE_TARGET)" | bc -l) -eq 1 ]; then \
		REMAINING=$$(echo "$(COVERAGE_TARGET) - $${COVERAGE}" | bc -l); \
		echo "$(GREEN)✅ Coverage meets minimum threshold$(RESET)"; \
		echo "$(BLUE)💡 $${REMAINING}% more coverage needed to reach target$(RESET)"; \
	else \
		echo "$(GREEN)🎉 Excellent! Coverage exceeds target of $(COVERAGE_TARGET)%$(RESET)"; \
	fi

test-integration: ## Run integration tests
	@echo "$(YELLOW)Running integration tests...$(RESET)"
	go test -v -tags=integration ./... -coverprofile=integration_coverage.out

lint: ## Run golangci-lint
	@echo "$(YELLOW)Running linters...$(RESET)"
	golangci-lint run --config=.golangci.yml --out-format=colored-line-number
	@echo "$(GREEN)✅ All linting checks passed!$(RESET)"

fmt: ## Format code
	@echo "$(YELLOW)Formatting code...$(RESET)"
	gofmt -w .
	goimports -w .

vet: ## Run go vet
	@echo "$(YELLOW)Running go vet...$(RESET)"
	go vet ./...

security: ## Run security scan
	@echo "$(YELLOW)Running security scan...$(RESET)"
	gosec ./...

benchmark: ## Run benchmarks
	@echo "$(YELLOW)Running benchmarks...$(RESET)"
	go test -bench=. -benchmem -count=3 -timeout=30m ./... > benchmark_results.txt
	cat benchmark_results.txt

quality-check: deps fmt vet lint test-race test-coverage ## Run all quality checks
	@echo "$(GREEN)🎯 All quality checks passed!$(RESET)"

ci-local: quality-check security benchmark ## Run CI pipeline locally
	@echo "$(GREEN)🚀 Local CI pipeline completed successfully!$(RESET)"

docker-build: ## Build Docker image
	@echo "$(YELLOW)Building Docker image...$(RESET)"
	docker build -f docker/Dockerfile -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .

docker-run: docker-build ## Run Docker container
	@echo "$(YELLOW)Running Docker container...$(RESET)"
	docker run -p 8080:8080 --rm $(DOCKER_IMAGE):latest

docker-scan: docker-build ## Scan Docker image for vulnerabilities
	@echo "$(YELLOW)Scanning Docker image for vulnerabilities...$(RESET)"
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
		aquasec/trivy image $(DOCKER_IMAGE):$(VERSION)

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	rm -rf bin/
	rm -f coverage.out coverage.html integration_coverage.out
	rm -f benchmark_results.txt
	go clean -cache -testcache -modcache

dev-setup: deps ## Setup development environment
	@echo "$(YELLOW)Setting up development environment...$(RESET)"
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest; \
	}
	@command -v gosec >/dev/null 2>&1 || { \
		echo "Installing gosec..."; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
	}
	@command -v goimports >/dev/null 2>&1 || { \
		echo "Installing goimports..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
	}
	@echo "$(GREEN)✅ Development environment setup complete!$(RESET)"

status: ## Show project status
	@echo "$(BLUE)Hotel Reviews Microservice Status$(RESET)"
	@echo "=================================="
	@echo "Version: $(VERSION)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Go Version: $$(go version)"
	@echo ""
	@echo "Quality Metrics:"
	@if [ -f coverage.out ]; then \
		COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
		echo "  Test Coverage: $${COVERAGE}%"; \
	else \
		echo "  Test Coverage: Not measured (run 'make test-coverage')"; \
	fi
	@echo "  Coverage Threshold: $(COVERAGE_THRESHOLD)%"
	@echo "  Coverage Target: $(COVERAGE_TARGET)%"
	@echo ""
	@echo "Available Commands: make help"