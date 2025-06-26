# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install dependencies for building
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mapping-engine cmd/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN adduser -D -s /bin/sh mappinguser

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/mapping-engine .

# Copy configuration files
COPY --from=builder /app/configs/ ./configs/

# Change ownership to non-root user
RUN chown -R mappinguser:mappinguser /app

# Switch to non-root user
USER mappinguser

# Expose port (if needed for health checks or metrics)
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["./mapping-engine"]
