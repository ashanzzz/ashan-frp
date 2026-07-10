# syntax=docker/dockerfile:1.7
FROM golang:1.25.0-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates tzdata
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/ashan-frp ./cmd/ashan-frp

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -g '' app
WORKDIR /app
COPY --from=builder /out/ashan-frp /usr/local/bin/ashan-frp
EXPOSE 8080
ENV HTTP_ADDR=:8080
USER app
ENTRYPOINT ["/usr/local/bin/ashan-frp"]
