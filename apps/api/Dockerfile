FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/server && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/demo ./cmd/demo

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata curl && addgroup -g 10001 app && adduser -D -u 10001 -G app app && mkdir -p /app/data /app/credentials && chown -R app:app /app
COPY --from=builder /out/server /out/demo /app/
USER 10001:10001
ENV HOST=0.0.0.0 PORT=4820 ENVIRONMENT=production DB_PATH=/app/data/p3.db MANAGER_KEY_PATH=/app/credentials/manager.key
EXPOSE 4820
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=6 CMD curl -fsS http://127.0.0.1:4820/healthz || exit 1
ENTRYPOINT ["/app/server"]
