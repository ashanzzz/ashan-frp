FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY frp-backend/ ./frp-backend/
WORKDIR /build/frp-backend
ENV GOFLAGS=-mod=mod
ENV CGO_ENABLED=0
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o ashan-frp ./cmd/ashan-frp
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget curl
WORKDIR /app
COPY --from=builder /build/frp-backend/ashan-frp .
RUN mkdir -p /app/data /app/data/frpc/bin /app/data/frpc/conf /app/data/frpc/logs /app/data/backups /app/data/tmp
EXPOSE 8080
ENV HTTP_ADDR=:8080
ENV DATA_DIR=/app/data
ENV DATABASE_DSN=file:/app/data/state.db
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1 || exit 1
ENTRYPOINT ["./ashan-frp"]
