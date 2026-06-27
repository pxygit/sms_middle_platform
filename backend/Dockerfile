# syntax=docker/dockerfile:1.7
ARG GO_VERSION=1.25.0

FROM golang:${GO_VERSION}-alpine AS builder
RUN apk add --no-cache ca-certificates git tzdata
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sunnysms-api ./cmd/api

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S app -G app
WORKDIR /app
COPY --from=builder /out/sunnysms-api /usr/local/bin/sunnysms-api
RUN mkdir -p /app/storage/card_exports \
    && chown -R app:app /app
USER app
EXPOSE 8080
ENTRYPOINT ["sunnysms-api"]