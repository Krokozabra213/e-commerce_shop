FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

# Версия goose определяет зависимости бинаря: у v3.22.1 в транзитивных
# зависимостях были уязвимости (pgx, x/crypto, x/net, grpc), которые видит
# `trivy image`. v3.29.0 подтягивает пропатченные версии.
RUN CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@v3.29.0

COPY services/inventory-service/migrations /app/migrations


# distroless: без shell и пакетного менеджера; ca-certificates внутри.
# RUN chmod больше не нужен: COPY сохраняет режим 0755 из builder'а,
# а в distroless нет shell, чтобы выполнить RUN.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder --chown=65534:65534 /app/migrations /app/migrations
COPY --from=builder --chown=65534:65534 /go/bin/goose /usr/local/bin/goose

ENV GOOSE_DRIVER=postgres
ENV GOOSE_DBSTRING=
ENV GOOSE_MIGRATION_DIR=/app/migrations

# uid/gid совпадают с securityContext Job'а миграций в deploy/helm/ecommerce-shop.
USER 65534:65534

CMD ["goose", "up"]
