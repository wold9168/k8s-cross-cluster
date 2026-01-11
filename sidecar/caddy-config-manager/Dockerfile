# Build stage
FROM golang:1.25.5-alpine AS builder

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy the entire project first to satisfy the replace directive
COPY ./../.. /app/k8s-cross-cluster

# Set working directory to the caddy-config-manager directory
WORKDIR /app/k8s-cross-cluster/sidecar/caddy-config-manager

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -o caddy-config-manager .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for Kubernetes API communication
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/k8s-cross-cluster/sidecar/caddy-config-manager/caddy-config-manager .

# Set timezone
ENV TZ=Asia/Shanghai

# Run the application
ENTRYPOINT ["./caddy-config-manager"]
