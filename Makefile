.PHONY: build run test clean dev init-db

# Build variables
BINARY_NAME=server
BUILD_DIR=bin
CMD_DIR=./cmd/server

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@mkdir -p data
	@go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	@mkdir -p data
	@go run $(CMD_DIR)/main.go

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean
	@echo "Clean complete"

# Clean everything including database
clean-all: clean
	@echo "Removing database..."
	@rm -rf data/

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	@air

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@golangci-lint run

# Build for Linux
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)/main.go

# Build for Windows
build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)/main.go

# Build for macOS
build-mac:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)/main.go

# Build all platforms
build-all: build-linux build-windows build-mac

# Initialize database (automatically created by the app)
init-db:
	@echo "Database will be automatically created on first run at ./data/machinus.db"
	@mkdir -p data
