FROM golang:1.22-alpine AS go-builder
WORKDIR /src
COPY frp-backend/ ./frp-backend/
WORKDIR /src/frp-backend
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ashan-frp ./cmd/ashan-frp

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=go-builder /out/ashan-frp /app/ashan-frp
RUN mkdir -p /app/data
ENV DATA_DIR=/app/data
ENV HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1 || exit 1
CMD ["/app/ashan-frp"]
