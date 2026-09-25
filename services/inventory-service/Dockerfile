FROM golang:1.26-alpine AS build

RUN apk add --no-cache git

WORKDIR /build

COPY . .

WORKDIR /build/services/inventory-service

RUN go mod download
RUN GOOS=linux GOARCH=amd64 go build -o /inventory-service ./cmd/server


FROM alpine:3.20 AS final

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /inventory-service .
COPY --from=build /build/services/inventory-service/configs ./configs

CMD ["./inventory-service"]
