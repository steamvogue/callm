SHELL := /usr/bin/env bash
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.PHONY: all help build test models info install uninstall check-env lint clean

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
	@go build -ldflags="-s -w" -o bin/callm ./cmd/callm
	@ln -sf callm bin/straitly
	@printf "\033[32mOK:\033[0m Built bin/callm successfully.\n"

check-env: ## Verify required environment variables
	@if [ -z "$$STRAITLY_API_KEY" ] && [ -z "$$CALLM_API_KEY" ] && [ -z "$$OPENAI_API_KEY" ] && [ ! -f .env ]; then \
		printf "\033[31mError:\033[0m No API key configured and .env does not exist.\n" >&2; \
		printf "Export CALLM_API_KEY, STRAITLY_API_KEY, or OPENAI_API_KEY, or create .env\n" >&2; \
		exit 1; \
	else \
		printf "\033[32mOK:\033[0m API key configured.\n"; \
	fi

test: build check-env ## Run sanity completion test using default model (deepseek/deepseek-v4-flash-0731)
	@echo "Testing gateway with deepseek/deepseek-v4-flash-0731..."
	@./bin/callm --stats --no-reasoning "Reply with exactly: OK_CALLM_READY"

models: build check-env ## List DeepSeek models available on the gateway
	@./bin/callm models deepseek

info: build check-env ## Inspect specifications and pricing for default model
	@./bin/callm info deepseek/deepseek-v4-flash-0731

lint: ## Run go vet on all packages
	@go vet ./... && printf "\033[32mOK:\033[0m Go code passes vet checks.\n"

clean: ## Remove compiled binary
	@rm -f bin/callm bin/straitly
	@printf "\033[32mOK:\033[0m Cleaned bin/.\n"

install: build ## Install callm into $(BINDIR)
	@install -d $(DESTDIR)$(BINDIR)
	@install -m 755 bin/callm $(DESTDIR)$(BINDIR)/callm
	@ln -sf callm $(DESTDIR)$(BINDIR)/straitly
	@echo "Installed callm to $(DESTDIR)$(BINDIR)/callm"

uninstall: ## Remove callm from $(BINDIR)
	@rm -f $(DESTDIR)$(BINDIR)/callm $(DESTDIR)$(BINDIR)/straitly
	@echo "Removed $(DESTDIR)$(BINDIR)/callm"
