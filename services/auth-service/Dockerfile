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
COPY --from=build /build/services/auth-service/certs ./certs

CMD ["./auth-service"]
