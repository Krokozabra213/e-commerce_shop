FROM golang:1.26-alpine AS build

RUN apk add --no-cache git

WORKDIR /build

COPY . .

WORKDIR /build/services/api-gateway

RUN go mod download
RUN GOOS=linux GOARCH=amd64 go build -o /api-gateway ./cmd/server


FROM alpine:3.20 AS final

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /api-gateway .
COPY --from=build /build/services/api-gateway/configs ./configs
COPY --from=build /build/services/api-gateway/api ./api

CMD ["./api-gateway"]
