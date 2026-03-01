FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with version info
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X github.com/example/rate-limiter/internal/version.Version=${VERSION} \
              -X github.com/example/rate-limiter/internal/version.BuildTime=${BUILD_TIME} \
              -X github.com/example/rate-limiter/internal/version.GitCommit=${GIT_COMMIT}" \
    -o /rate-limiter ./cmd/rate-limiter

FROM alpine:3.19

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /rate-limiter /usr/local/bin/rate-limiter
COPY config.example.yaml /app/config.yaml

# Change ownership to non-root user
RUN chown -R appuser:appuser /app /usr/local/bin/rate-limiter

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/rate-limiter", "-config", "/app/config.yaml"]

