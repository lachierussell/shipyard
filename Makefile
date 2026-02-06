# Shipyard Makefile

VERSION ?= 0.1.0
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BINARY_NAME = shipyard
BUILD_DIR = /tmp

# Server config - set SHIPYARD_ADMIN_KEY env var or pass on command line
SERVER_URL ?= https://shipyard.parkcedar.com/api
ADMIN_KEY ?= $(SHIPYARD_ADMIN_KEY)

LDFLAGS = -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT)"

.PHONY: build build-freebsd deploy run test clean web-dev web-build help

## Build for local platform
build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

## Build for FreeBSD arm64 (production target)
build-freebsd:
	GOOS=freebsd GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-freebsd .

## Build and deploy to test server
deploy: build-freebsd
ifndef ADMIN_KEY
	$(error SHIPYARD_ADMIN_KEY is not set. Export it or pass ADMIN_KEY=...)
endif
	curl -sk -X POST \
		-H 'X-Shipyard-Key: $(ADMIN_KEY)' \
		--data-binary @$(BUILD_DIR)/$(BINARY_NAME)-freebsd \
		$(SERVER_URL)/deploy/self

## Run locally
run: build
	$(BUILD_DIR)/$(BINARY_NAME) serve

## Run tests
test:
	go test ./...

## Clean build artifacts
clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_NAME)-freebsd

## Start web admin UI dev server
web-dev:
	cd web && npm run dev

## Build web admin UI
web-build:
	cd web && npm run build

## Install web dependencies
web-install:
	cd web && npm install

## Show help
help:
	@echo "Shipyard Makefile"
	@echo ""
	@echo "Usage: make [target] [VERSION=x.x.x]"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build for local platform"
	@echo "  build-freebsd  Build for FreeBSD arm64"
	@echo "  deploy         Build and deploy to test server (requires SHIPYARD_ADMIN_KEY)"
	@echo "  run            Build and run locally"
	@echo "  test           Run tests"
	@echo "  clean          Remove build artifacts"
	@echo "  web-dev        Start web admin UI dev server"
	@echo "  web-build      Build web admin UI for production"
	@echo "  web-install    Install web dependencies"
	@echo ""
	@echo "Environment:"
	@echo "  SHIPYARD_ADMIN_KEY  Admin API key for deploy (required)"
	@echo "  SERVER_URL          Override server URL (default: https://54.206.54.73:8443)"
	@echo ""
	@echo "Examples:"
	@echo "  export SHIPYARD_ADMIN_KEY=sk-admin-..."
	@echo "  make deploy VERSION=1.0.0"
