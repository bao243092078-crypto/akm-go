.PHONY: all build clean web install test help

# Variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X github.com/baobao/akm-go/internal/cli.Version=$(VERSION)"
BINARY = akm
PYTHON_WEB = ../apikey-manager/web
WEB_DEST = cmd/akm/web/dist

all: build

# Build the binary (without embedded web UI)
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/akm

# Build the binary with embedded web UI
build-full: web
	go build $(LDFLAGS) -o $(BINARY) ./cmd/akm

# Build web UI from Python project
web:
	@echo "Building web UI..."
	@PYTHON_WEB_ABS="$$(cd $(PYTHON_WEB) 2>/dev/null && pwd)"; \
	if [ -n "$$PYTHON_WEB_ABS" ]; then \
		(cd $$PYTHON_WEB_ABS && npm install && npm run build); \
		rm -rf $(WEB_DEST); \
		mkdir -p $(WEB_DEST); \
		cp -r $$PYTHON_WEB_ABS/dist/* $(WEB_DEST)/; \
		echo "Web UI built successfully"; \
	else \
		echo "Warning: Python web directory not found at $(PYTHON_WEB)"; \
		mkdir -p $(WEB_DEST); \
		echo '<!DOCTYPE html><html><body><h1>Web UI not available</h1></body></html>' > $(WEB_DEST)/index.html; \
	fi

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -rf $(WEB_DEST)

# Install to /usr/local/bin
install: build
	cp $(BINARY) /usr/local/bin/

# Run tests
test:
	go test -v ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  build      - Build binary without web UI"
	@echo "  build-full - Build binary with embedded web UI"
	@echo "  web        - Build web UI from Python project"
	@echo "  clean      - Remove build artifacts"
	@echo "  install    - Install to /usr/local/bin"
	@echo "  test       - Run tests"
