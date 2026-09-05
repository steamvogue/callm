SHELL := /usr/bin/env bash
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.Date=$(DATE)

.PHONY: all help build version test test-unit test-live models info install uninstall check-env lint clean cross-compile

all: build

help: ## Show this help message
	@echo "callm (call - llm) — Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Quick start:"
	@echo "  ./bin/callm \"Explain OKLCH in one sentence\""
	@echo "  ./bin/callm models deepseek"
	@echo "  ./bin/callm --or \"Use OpenRouter preset\""
	@echo "  ./bin/callm --ds \"Use DeepSeek direct preset\""
	@echo ""

build: ## Compile Go binary into bin/callm
	@mkdir -p bin
	@go build -ldflags="$(LDFLAGS)" -o bin/callm ./cmd/callm
	@ln -sf callm bin/straitly
	@printf "\033[32mOK:\033[0m Built bin/callm ($(VERSION)) successfully.\n"

version: build ## Print built binary version
	@./bin/callm --version

check-env: ## Verify required environment variables
	@if [ -z "$$STRAITLY_API_KEY" ] && [ -z "$$CALLM_API_KEY" ] && [ -z "$$OPENAI_API_KEY" ] && [ ! -f .env ]; then \
		printf "\033[31mError:\033[0m No API key configured and .env does not exist.\n" >&2; \
		printf "Export CALLM_API_KEY, STRAITLY_API_KEY, or OPENAI_API_KEY, or create .env\n" >&2; \
		exit 1; \
	else \
		printf "\033[32mOK:\033[0m API key configured.\n"; \
	fi

test: test-unit ## Run all unit tests including mock API and reasoning streaming tests

test-unit: ## Run unit tests with race detection and verbose output
	@go test -v ./...

test-live: build ## Run live minimal and reasoning tests across all configured providers
	@./scripts/test_live.sh

models: build check-env ## List DeepSeek models available on the gateway
	@./bin/callm models deepseek

info: build check-env ## Inspect specifications and pricing for default model
	@./bin/callm info deepseek/deepseek-v4-flash-0731

lint: ## Run go vet on all packages
	@go vet ./... && printf "\033[32mOK:\033[0m Go code passes vet checks.\n"

clean: ## Remove compiled binary and dist archives
	@rm -rf bin/ dist/
	@printf "\033[32mOK:\033[0m Cleaned bin/ and dist/.\n"

cross-compile: ## Test cross-compiling for Linux, macOS, and Windows
	@mkdir -p dist
	@echo "Cross-compiling callm for all targets..."
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/callm-linux-amd64 ./cmd/callm
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/callm-linux-arm64 ./cmd/callm
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/callm-darwin-amd64 ./cmd/callm
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/callm-darwin-arm64 ./cmd/callm
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/callm-windows-amd64.exe ./cmd/callm
	@printf "\033[32mOK:\033[0m Cross-compiled all 5 targets into dist/.\n"

install: build ## Install callm into $(BINDIR)
	@install -d $(DESTDIR)$(BINDIR)
	@install -m 755 bin/callm $(DESTDIR)$(BINDIR)/callm
	@ln -sf callm $(DESTDIR)$(BINDIR)/straitly
	@echo "Installed callm to $(DESTDIR)$(BINDIR)/callm"

uninstall: ## Remove callm from $(BINDIR)
	@rm -f $(DESTDIR)$(BINDIR)/callm $(DESTDIR)$(BINDIR)/straitly
	@echo "Removed $(DESTDIR)$(BINDIR)/callm"
