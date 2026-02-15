.PHONY: build test quality install clean run hooks lefthook

help: ## Outputs this help screen
	@grep -E '(^[a-zA-Z0-9_-]+:.*?##.*$$)|(^##)' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}{printf "\033[32m%-30s\033[0m %s\n", $$1, $$2}' | sed -e 's/\[32m##/[33m/'

# Build variables
BINARY_NAME := creareview
BUILD_DIR := ./build
CMD_DIR := ./cmd/creareview
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
BUILD_FLAGS := -trimpath -v
LDFLAGS := -ldflags "-s -w -X github.com/valksor/go-toolkit/version.Version=$(VERSION) -X github.com/valksor/go-toolkit/version.Commit=$(COMMIT) -X github.com/valksor/go-toolkit/version.BuildTime=$(BUILD_TIME)"

# Default target
all: build ## Build the binary (default target)

build: ## Compile the binary
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

test: ## Run tests with coverage
	${MAKE} quality
	go test -v -cover ./...

race: ## Run race tests
	${MAKE} quality
	go test -v -race ./...

coverage: ## Run tests with race detection and coverage profile
	go test -race -covermode atomic -coverprofile=covprofile ./...

coverage-html: coverage ## Generate HTML coverage report
	@mkdir -p .coverage
	go tool cover -html=covprofile -o .coverage/coverage.html

quality: ## Run linter (golangci-lint)
	${MAKE} fmt
	golangci-lint run ./... --fix
	${MAKE} check-alias

fmt: ## Format code with go fmt, goimports, and gofumpt
	go fmt ./...
	goimports -w .
	gofumpt -l -w .

install: build ## Install binary locally
	@INSTALL_DIR=""; \
	for dir in "$$HOME/.local/bin" "$$HOME/bin" "/usr/local/bin"; do \
		if [ -d "$$dir" ] && [ -w "$$dir" ]; then \
			INSTALL_DIR="$$dir"; \
			break; \
		fi; \
		if [ "$$dir" = "$$HOME/.local/bin" ] || [ "$$dir" = "$$HOME/bin" ]; then \
			if mkdir -p "$$dir" 2>/dev/null; then \
				INSTALL_DIR="$$dir"; \
				break; \
			fi; \
		fi; \
	done; \
	if [ -z "$$INSTALL_DIR" ]; then \
		INSTALL_DIR="/usr/local/bin"; \
	fi; \
	INSTALL_PATH="$$INSTALL_DIR/$(BINARY_NAME)"; \
	echo "Installing to $$INSTALL_PATH..."; \
	if pgrep -x $(BINARY_NAME) >/dev/null 2>&1; then \
		echo "Stopping running $(BINARY_NAME) processes..."; \
		pkill -x $(BINARY_NAME) 2>/dev/null || true; \
		sleep 0.5; \
	fi; \
	if [ -w "$$INSTALL_DIR" ]; then \
		cp $(BUILD_DIR)/$(BINARY_NAME) "$$INSTALL_PATH"; \
	else \
		echo "Requesting sudo access to install to $$INSTALL_DIR..."; \
		sudo cp $(BUILD_DIR)/$(BINARY_NAME) "$$INSTALL_PATH"; \
	fi; \
	echo "Installed $(BINARY_NAME) to $$INSTALL_PATH"

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -rf .coverage covprofile

run: build ## Run the binary (for development)
	$(BUILD_DIR)/$(BINARY_NAME)

run-args: build ## Run the binary with arguments (use ARGS=...)
	$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

tidy: clean ## Clean and tidy dependencies
	go mod tidy -e
	go get -d -v ./...

deps: ## Download dependencies
	go mod download

version: build ## Show version info
	$(BUILD_DIR)/$(BINARY_NAME) version

hooks: ## Configure git to use versioned hooks
	git config core.hooksPath .github/.githooks
	@echo "Git hooks configured to use .githooks/"

lefthook: ## Install and configure Lefthook pre-commit hooks
	go install github.com/evilmartians/lefthook@latest
	lefthook install
	@echo "Lefthook installed. Pre-commit hooks active."

check-alias:
	@alias_issues="$$(./.github/alias.sh || true)"; \
	if [ -n "$$alias_issues" ]; then \
		echo "Unnecessary import alias detected:"; \
		echo "$$alias_issues"; \
		exit 1; \
	fi
