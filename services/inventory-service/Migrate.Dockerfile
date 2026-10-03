FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

RUN go install github.com/pressly/goose/v3/cmd/goose@v3.22.1

COPY services/inventory-service/migrations /app/migrations


FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/migrations /app/migrations
COPY --from=builder /go/bin/goose /usr/local/bin/goose

RUN chmod +x /usr/local/bin/goose

ENV GOOSE_DRIVER=postgres
ENV GOOSE_DBSTRING=
ENV GOOSE_MIGRATION_DIR=/app/migrations

CMD ["goose", "up"]
