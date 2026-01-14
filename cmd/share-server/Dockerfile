# Multi-stage build for share-server
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o share-server ./cmd/share-server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/share-server .

# Expose gRPC port
EXPOSE 9000

# Run as non-root user
RUN adduser -D -u 1000 hyena
USER hyena

ENTRYPOINT ["/app/share-server"]
