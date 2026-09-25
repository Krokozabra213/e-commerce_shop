FROM golang:1.26-alpine AS build

RUN apk add --no-cache git

WORKDIR /build

COPY . .

WORKDIR /build/services/product-service

RUN go mod download
RUN GOOS=linux GOARCH=amd64 go build -o /product-service ./cmd/server


FROM alpine:3.20 AS final

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /product-service .
COPY --from=build /build/services/product-service/configs ./configs
COPY --from=build /build/services/product-service/api ./api

CMD ["./product-service"]
