MODULES := api infra services/api-gateway services/auth-service services/inventory-service \
	services/notification-service services/order-service services/product-service services/user-service

ROOT := $(shell pwd)
GOLANGCI_CONFIG := $(ROOT)/.golangci.yaml

.PHONY: test
test:
	go test $(addsuffix /...,$(MODULES)) -count=1

.PHONY: test-race
test-race:
	@set -e; for m in $(MODULES); do \
		[ -n "$$(find $$m -name '*_test.go' -print -quit)" ] || { echo "==> skip (no tests): $$m"; continue; }; \
		echo "==> race: $$m"; \
		(cd $$m && go test -race ./... -count=1); \
	done

.PHONY: test-unit
test-unit:
	@set -e; for m in $(MODULES); do \
		[ -n "$$(find $$m -name '*_test.go' -print -quit)" ] || { echo "==> skip (no go files): $$m"; continue; }; \
		echo "==> unit: $$m"; \
		(cd $$m && go test ./... -count=1); \
	done

.PHONY: test-integration
test-integration:
	@set -e; for m in $(MODULES); do \
		if grep -rl --include='*_test.go' -e '^//go:build integration' $$m >/dev/null 2>&1; then \
			echo "==> integration: $$m"; \
			(cd $$m && go test ./... -tags=integration -count=1 -timeout=300s); \
		fi; \
	done

.PHONY: lint
lint:
	@set -e; for m in $(MODULES); do \
		[ -n "$$(find $$m -name '*.go' -print -quit)" ] || continue; \
		echo "==> lint: $$m"; \
		(cd $$m && golangci-lint run --config=$(GOLANGCI_CONFIG) --timeout=5m ./...); \
	done

.PHONY: fmt-check
fmt-check:
	@set -e; for m in $(MODULES); do \
		[ -n "$$(find $$m -name '*.go' -print -quit)" ] || continue; \
		echo "==> fmt-check: $$m"; \
		(cd $$m && golangci-lint fmt --config=$(GOLANGCI_CONFIG) --diff ./...); \
	done

.PHONY: fmt
fmt:
	@set -e; for m in $(MODULES); do \
		[ -n "$$(find $$m -name '*.go' -print -quit)" ] || continue; \
		echo "==> fmt: $$m"; \
		(cd $$m && golangci-lint fmt --config=$(GOLANGCI_CONFIG) ./...); \
	done