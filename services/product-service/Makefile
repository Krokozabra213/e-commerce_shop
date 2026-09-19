.PHONY: generate
generate:
	@echo "Generating code from OpenAPI spec..."
	oapi-codegen -config api/codegen-config.yaml api/openapi.yaml

.PHONY: generate-check
generate-check:
	@echo "Checking if generated code is up to date..."
	@git diff --exit-code internal/handler/http/generated/ 2>/dev/null || \
		(echo "Generated code is outdated or untracked! Run 'make generate' and commit." && exit 1)

.PHONY: serve
serve:
	go run cmd/product-service/main.go

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: test
test:
	go test -v -race -coverprofile=coverage.out ./...