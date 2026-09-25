GO ?= go
BUF_VERSION ?= 1.50.0

BIN_DIR := $(CURDIR)/bin
BUF := $(BIN_DIR)/buf

$(BUF):
	@mkdir -p $(BIN_DIR)
	@[ -x $(BUF) ] || { \
		echo "📦 Устанавливаем Buf $(BUF_VERSION)..."; \
		BUF_VERSION=v$(BUF_VERSION) curl -sSL \
			"https://github.com/bufbuild/buf/releases/download/v$(BUF_VERSION)/buf-Linux-x86_64" \
			-o $(BUF); \
		chmod +x $(BUF); \
	}

proto-deps: $(BUF)
	$(BUF) dep update

proto-lint: $(BUF) proto-deps
	$(BUF) lint

proto-gen: $(BUF) proto-deps proto-lint
	$(BUF) generate

proto-all: proto-deps proto-lint proto-gen

.PHONY: proto-deps proto-lint proto-gen proto-all
