FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
# 创设临时空壳以供嵌入式合规
RUN mkdir -p dist && echo "<h1>Ashan FRP Native Dashboard</h1>" > dist/index.html

FROM golang:1.22-alpine AS go-builder
WORKDIR /app
COPY frp-backend/ ./
COPY --from=frontend-builder /app/frontend/dist ./dist
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ashan-frp-bin .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /app/ashan-frp-bin /app/ashan-frp
RUN mkdir -p /app/data
ENV DATA_DIR=/app/data
EXPOSE 8080
CMD ["/app/ashan-frp"]
