MODULES := api infra services/api-gateway services/auth-service services/inventory-service \
	services/notification-service services/order-service services/product-service services/user-service

ROOT := $(shell pwd)
GOLANGCI_CONFIG := $(ROOT)/.golangci.yaml

.DEFAULT_GOAL := help

.PHONY: help
help: ## Показать список целей
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_.-]+:.*?## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: test
test: ## Запустить все тесты
	go test $(addsuffix /...,$(MODULES)) -count=1

.PHONY: test-race
test-race: ## Тесты с race-детектором
	@set -e; for m in $(MODULES); do \
		[ -n "$$(find $$m -name '*_test.go' -print -quit)" ] || { echo "==> skip (no tests): $$m"; continue; }; \
		echo "==> race: $$m"; \
		(cd $$m && go test -race ./... -count=1); \
	done

.PHONY: test-unit
test-unit: ## Unit-тесты по модулям
	@set -e; for m in $(MODULES); do \
		[ -n "$$(find $$m -name '*_test.go' -print -quit)" ] || { echo "==> skip (no go files): $$m"; continue; }; \
		echo "==> unit: $$m"; \
		(cd $$m && go test ./... -count=1); \
	done

.PHONY: test-integration
test-integration: ## Интеграционные тесты (нужен docker)
	@set -e; for m in $(MODULES); do \
		if grep -rl --include='*_test.go' -e '^//go:build integration' $$m >/dev/null 2>&1; then \
			echo "==> integration: $$m"; \
			(cd $$m && go test ./... -tags=integration -count=1 -timeout=300s); \
		fi; \
	done

.PHONY: lint
lint: ## golangci-lint по всем модулям
	@set -e; for m in $(MODULES); do \
		[ -n "$$(find $$m -name '*.go' -print -quit)" ] || continue; \
		echo "==> lint: $$m"; \
		(cd $$m && golangci-lint run --config=$(GOLANGCI_CONFIG) --timeout=5m ./...); \
	done

.PHONY: fmt-check
fmt-check: ## Проверить форматирование
	@set -e; for m in $(MODULES); do \
		[ -n "$$(find $$m -name '*.go' -print -quit)" ] || continue; \
		echo "==> fmt-check: $$m"; \
		(cd $$m && golangci-lint fmt --config=$(GOLANGCI_CONFIG) --diff ./...); \
	done

.PHONY: fmt
fmt: ## Отформатировать код
	@set -e; for m in $(MODULES); do \
		[ -n "$$(find $$m -name '*.go' -print -quit)" ] || continue; \
		echo "==> fmt: $$m"; \
		(cd $$m && golangci-lint fmt --config=$(GOLANGCI_CONFIG) ./...); \
	done

# -----------------------------------------------------------------------------
# Локальный запуск в docker compose
# -----------------------------------------------------------------------------

.PHONY: dev-init
dev-init: ## Подготовить .env и ключ JWT для docker compose
	./scripts/dev-init.sh

.PHONY: dev-up
dev-up: dev-init ## Поднять окружение в docker compose (первый запуск собирает образы)
	docker compose up -d --build

.PHONY: dev-down
dev-down: ## Остановить окружение (данные в томах сохраняются)
	docker compose down

.PHONY: dev-reset
dev-reset: ## Остановить окружение и удалить данные (тома)
	docker compose down -v

.PHONY: dev-ps
dev-ps: ## Статус контейнеров
	docker compose ps

.PHONY: dev-logs
dev-logs: ## Логи сервиса: make dev-logs SVC=auth-service
	docker compose logs -f $(SVC)