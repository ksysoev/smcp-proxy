FROM golang:1.24-alpine AS builder

# Set necessary environment variables for the build
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GO111MODULE=on

# Install git and ca-certificates for downloading modules
RUN apk add --no-cache git ca-certificates

# Create appuser for the final stage
RUN adduser -D -g '' appuser

# Set the working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies with verbose output
RUN go mod download -x

# Copy the entire codebase
COPY . .

# Build the application
RUN go build -ldflags="-s -w" -o /app/smcp-proxy ./cmd/smcp

# Final stage - using scratch (minimal image)
FROM scratch

# Import the user from builder
COPY --from=builder /etc/passwd /etc/passwd

# Create directories for certificates
COPY --from=builder /etc/ssl/certs /etc/ssl/certs

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