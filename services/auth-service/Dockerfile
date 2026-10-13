FROM golang:1.26-alpine AS build

RUN apk add --no-cache git

WORKDIR /build

COPY . .

WORKDIR /build/services/auth-service

RUN go mod download
# CGO_ENABLED=0 обязателен: финальный образ distroless/static не содержит libc.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /auth-service ./cmd/server


# distroless: без shell, пакетного менеджера и busybox; внутри уже есть
# ca-certificates, tzdata и /etc/nsswitch.conf (нужен pure-Go резолверу).
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build --chown=65534:65534 /auth-service .
COPY --from=build --chown=65534:65534 /build/services/auth-service/configs ./configs
# ВНИМАНИЕ: каталог certs/ (приватный ключ подписи JWT) НЕ копируется в образ.
# Ключ монтируется из Kubernetes Secret'а (см. deploy/helm/ecommerce-shop),
# путь задаётся через AUTH_JWT_PRIVATE_KEY_PATH. Копирование ключа в образ
# означало бы, что любой, кто может забрать образ, может подделывать JWT.
# Для локального docker compose ключ монтируется с хоста и должен быть
# доступен на чтение пользователю 65534 (см. scripts/dev-init.sh).

# uid/gid совпадают с defaults.podSecurityContext в deploy/helm/ecommerce-shop.
# USER в образе — страховка для запусков вне чарта (docker run/compose, CI).
USER 65534:65534

CMD ["./auth-service"]
