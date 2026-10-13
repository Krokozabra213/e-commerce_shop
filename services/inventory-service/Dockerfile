FROM golang:1.26-alpine AS build

RUN apk add --no-cache git

WORKDIR /build

COPY . .

WORKDIR /build/services/inventory-service

RUN go mod download
# CGO_ENABLED=0 обязателен: финальный образ distroless/static не содержит libc.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /inventory-service ./cmd/server


# distroless: без shell, пакетного менеджера и busybox; внутри уже есть
# ca-certificates, tzdata и /etc/nsswitch.conf (нужен pure-Go резолверу).
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build --chown=65534:65534 /inventory-service .
COPY --from=build --chown=65534:65534 /build/services/inventory-service/configs ./configs

# uid/gid совпадают с defaults.podSecurityContext в deploy/helm/ecommerce-shop.
# USER в образе — страховка для запусков вне чарта (docker run/compose, CI).
USER 65534:65534

CMD ["./inventory-service"]
