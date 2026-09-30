MIGRATIONS=migrations

migrate-create:
	goose -dir $(MIGRATIONS) create $(name) sql

test-integration:
	go test ./... \
		-tags=integration \
		-v \
		-count=1 \
		-timeout=300s

test:
	go test ./... -v -count=1