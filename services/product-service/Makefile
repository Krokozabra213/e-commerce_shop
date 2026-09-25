.PHONY: generate
generate:
	@echo "Generating code from OpenAPI spec..."
	@oapi-codegen -config api/codegen-config.yaml api/openapi.yaml 2>&1
	@echo "Generated successfully."

.PHONY: generate-check
generate-check:
	@echo "Checking if generated code is up to date..."
	@git diff --exit-code internal/handler/http/generated/ 2>/dev/null || \
		(echo "Generated code is outdated or untracked! Run 'make generate' and commit." && exit 1)

.PHONY: test-integration
test-integration:
	go test ./... \
		-tags=integration \
		-v \
		-count=1 \
		-timeout=300s

.PHONY: test
test:
	go test ./... -v -count=1