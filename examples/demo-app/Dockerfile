# Multi-stage build for demo-app
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy just the demo app
COPY examples/demo-app/main.go ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o demo-app main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/demo-app .

# Expose HTTP port
EXPOSE 8080

# Run as non-root user
RUN adduser -D -u 1000 hyena
USER hyena

ENTRYPOINT ["/app/demo-app"]
