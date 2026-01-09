.PHONY: build clean test lint install-deps run fmt vet

# Binary name
BINARY_NAME=loky

# Build directory
BUILD_DIR=bin

# Version
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

all: clean test build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/loky
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-all: build
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/loky
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/loky
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/loky
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/loky
	@echo "Cross-build complete"

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"

test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"
	@echo "Coverage:"
	$(GOCMD) tool cover -func=coverage.out | tail -1

test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi
	$(GOVET) ./...
	@echo "Lint complete"

fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .
	@echo "Format complete"

vet:
	@echo "Vetting code..."
	$(GOVET) ./...
	@echo "Vet complete"

install-deps:
	@echo "Installing dependencies..."
	$(GOMOD) tidy
	$(GOGET) -u ./...
	@echo "Dependencies installed"

run:
	@echo "Running $(BINARY_NAME)..."
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/loky
	$(BUILD_DIR)/$(BINARY_NAME)

docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .
	@echo "Docker image built: $(BINARY_NAME):$(VERSION)"

docker-run: docker-build
	@echo "Running Docker container..."
	docker run --rm $(BINARY_NAME):$(VERSION)

deps:
	@echo "Installing build dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Build dependencies installed"

help:
	@echo "Available targets:"
	@echo "  make build        - Build the binary"
	@echo "  make build-all    - Build for multiple platforms"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make test         - Run tests with coverage"
	@echo "  make test-coverage - Run tests and generate HTML coverage report"
	@echo "  make lint         - Run linters"
	@echo "  make fmt          - Format code"
	@echo "  make vet          - Vet code"
	@echo "  make install-deps - Install/update dependencies"
	@echo "  make run          - Build and run the binary"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-run   - Build and run Docker container"
	@echo "  make deps         - Install build dependencies"
	@echo "  make help         - Show this help message"
