VERSION      ?= 0.1.1-beta.1
PROJECT_NAME := archivus
BACKEND_DIR  := backend
FRONTEND_DIR := archivus-svelte

DIST_DIR := dist
PKG_DIR  := $(DIST_DIR)/packages

# The frontend is served by the Go binary on the same origin, so the API base
# URL is relative. Override for a frontend hosted separately, e.g.
#   make build PUBLIC_API_URL=https://archivus.example.com
PUBLIC_API_URL ?=

# cgo is required: the store uses mattn/go-sqlite3. This also means release
# builds must run on the target OS/arch (or under a cross toolchain), so only
# the host platform is built here.
GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
PLATFORM  := $(GOOS)_$(GOARCH)
STAGE_DIR := $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-$(GOOS)-$(GOARCH)

# Release builds resolve config and data under ~/.archivus instead of the
# working directory.
LDFLAGS := -s -w -X archivus/internal/config.debugMode=false

.PHONY: all build build-backend build-cron build-frontend package clean dev-backend dev-cron dev-frontend test

all: build

build: build-backend build-cron build-frontend

build-backend:
	@echo "==> Building backend ($(PLATFORM))"
	mkdir -p $(STAGE_DIR)
	cd $(BACKEND_DIR) && CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -trimpath -ldflags "$(LDFLAGS)" \
		-o ../$(STAGE_DIR)/$(PROJECT_NAME) ./cmd/archivus

# Long-running companion process to the server: runs the periodic jobs
# (thumbnail generation) on a cron schedule.
build-cron:
	@echo "==> Building cron scheduler ($(PLATFORM))"
	mkdir -p $(STAGE_DIR)
	cd $(BACKEND_DIR) && CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -trimpath -ldflags "$(LDFLAGS)" \
		-o ../$(STAGE_DIR)/$(PROJECT_NAME)-cron ./cmd/celery

build-frontend:
	@echo "==> Building frontend"
	rm -rf $(FRONTEND_DIR)/build $(STAGE_DIR)/static
	cd $(FRONTEND_DIR) && npm ci && PUBLIC_API_URL="$(PUBLIC_API_URL)" npm run build
	mkdir -p $(STAGE_DIR)
	cp -r $(FRONTEND_DIR)/build $(STAGE_DIR)/static

package: build
	@echo "==> Packaging $(PROJECT_NAME)-$(VERSION)-$(GOOS)-$(GOARCH)"
	install -m 0755 install.sh $(STAGE_DIR)/install.sh
	mkdir -p $(PKG_DIR)
	tar -czf $(PKG_DIR)/$(PROJECT_NAME)-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz \
		-C $(DIST_DIR) $(notdir $(STAGE_DIR))
	@echo "==> $(PKG_DIR)/$(PROJECT_NAME)-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz"

test:
	cd $(BACKEND_DIR) && go test ./...

dev-backend:
	cd $(BACKEND_DIR) && go run ./cmd/archivus server -m home

dev-cron:
	cd $(BACKEND_DIR) && go run ./cmd/celery -m home

dev-frontend:
	cd $(FRONTEND_DIR) && npm run dev

clean:
	rm -rf $(DIST_DIR) $(FRONTEND_DIR)/build
