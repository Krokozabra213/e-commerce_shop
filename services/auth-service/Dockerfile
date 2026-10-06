FROM golang:1.26-alpine AS build

RUN apk add --no-cache git

WORKDIR /build

COPY . .

WORKDIR /build/services/auth-service

RUN go mod download
RUN GOOS=linux GOARCH=amd64 go build -o /auth-service ./cmd/server


FROM alpine:3.20 AS final

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /auth-service .
COPY --from=build /build/services/auth-service/configs ./configs
# ВНИМАНИЕ: каталог certs/ (приватный ключ подписи JWT) НЕ копируется в образ.
# Ключ монтируется из Kubernetes Secret'а (см. deploy/helm/ecommerce-shop),
# путь задаётся через AUTH_JWT_PRIVATE_KEY_PATH. Копирование ключа в образ
# означало бы, что любой, кто может забрать образ, может подделывать JWT.

CMD ["./auth-service"]
