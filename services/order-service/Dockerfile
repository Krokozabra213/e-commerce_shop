FROM golang:1.26-alpine AS build

RUN apk add --no-cache git

WORKDIR /build

COPY . .

WORKDIR /build/services/order-service

RUN go mod download
RUN GOOS=linux GOARCH=amd64 go build -o /order-service ./cmd/server


FROM alpine:3.20 AS final

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /order-service .
COPY --from=build /build/services/order-service/configs ./configs

CMD ["./order-service"]
