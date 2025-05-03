FROM golang:1.21-alpine AS builder

# Set necessary environment variables for the build
ENV CGO_ENABLED=0 \
    GOOS=linux

# Create appuser for the final stage
RUN adduser -D -g '' appuser

# Set the working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the entire codebase
COPY . .

# Build the application
RUN go build -ldflags="-s -w" -o /app/smcp-proxy ./cmd/smcp

# Final stage - using scratch (minimal image)
FROM scratch

# Import the user from builder
COPY --from=builder /etc/passwd /etc/passwd

# Copy CA certificates for HTTPS connections
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder /app/smcp-proxy /app/smcp-proxy

# Copy default configs directory
COPY configs /app/configs

# Set the user to run the application
USER appuser

# Set the working directory
WORKDIR /app

# Command to run
ENTRYPOINT ["/app/smcp-proxy"]

# Default command is to show help
CMD ["--help"]