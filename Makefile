# SenSimul Makefile
# Go-based Environment Sensor Simulator

# Go environment
export PATH := /workspace/go/go/bin:$(PATH)
export GOPATH := /workspace/gopath
export GO111MODULE := on

# Binary name and paths
BINARY_NAME := sensimul
BUILD_DIR := build
CMD_PATH := ./cmd/sensimul

# Build targets
.PHONY: build test race test-all clean docker-build docker-up docker-down docker-logs

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-web:
	@echo "Building web binary..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/sensimul-web ./cmd/web
	@echo "Build complete: $(BUILD_DIR)/sensimul-web"

test:
	@echo "Running tests..."
	go test ./...

test-race:
	@echo "Running tests with race detector..."
	go test -race ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	go clean

# Docker targets
docker-build:
	@echo "Building Docker image..."
	docker build -t sensimul/sensimul:latest .

docker-up:
	@echo "Starting Docker Compose stack..."
	docker compose up -d

docker-up-mqtt:
	@echo "Starting MQTT broker only..."
	docker compose -f docker-compose.mqtt.yml up -d

docker-down:
	@echo "Stopping Docker Compose stack..."
	docker compose down

docker-logs:
	docker compose logs -f

# Development targets
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin)
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

# Help
help:
	@echo "SenSimul Makefile targets:"
	@echo "  build         - Build the binary"
	@echo "  build-web     - Build the web binary"
	@echo "  test          - Run tests"
	@echo "  test-race     - Run tests with race detector"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-up     - Start full Docker Compose stack"
	@echo "  docker-up-mqtt - Start MQTT broker only"
	@echo "  docker-down   - Stop Docker Compose stack"
	@echo "  docker-logs   - Follow Docker Compose logs"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
