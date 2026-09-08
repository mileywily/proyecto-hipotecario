# Stage 1: Build static binary
FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies and certificates
RUN apk add --no-cache git ca-certificates

# Cache dependency layer
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary without CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /app/server ./cmd/server

# Stage 2: Minimal runtime image
FROM alpine:latest

WORKDIR /app

# Add SSL root certs and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Create non-root system group and user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy compiled binary from builder
COPY --from=builder /app/server /app/server

# Ensure ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose microservice port
EXPOSE 8003

# Default port configuration
ENV PORT=8003

ENTRYPOINT ["/app/server"]
