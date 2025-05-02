# SMCP Proxy

A secure reverse proxy for Model Context Protocol (MCP) services with OIDC authentication.

## Overview

SMCP Proxy provides a secure layer in front of Model Context Protocol (MCP) services, enabling enterprise-grade authentication and authorization using OIDC. MCP is a protocol designed for interacting with Large Language Models (LLMs) in a standardized way.

The proxy consists of two main components:

1. **Proxy Server**: Validates OIDC tokens from clients and forwards authenticated requests to MCP server(s)
2. **Proxy Client**: Implements client credentials flow to acquire tokens and acts as a local unauthenticated MCP service

The main goals of this project are:
- Provide secure access to MCP services using OIDC authentication
- Support scalable MCP infrastructure in enterprise environments
- Enable centralized authentication and authorization for MCP services
- Simplify client integration with authenticated MCP services

## Architecture

```
┌───────────────┐      ┌───────────────┐      ┌───────────────┐
│  Application  │      │ Proxy Client  │      │ Proxy Server  │      ┌───────────┐
│  (no auth)    │──────│ (local)       │──────│ (with auth)   │──────│ MCP Server│
└───────────────┘      └───────────────┘      └───────────────┘      └───────────┘
                         │                       ▲
                         │                       │
                         │                       │
                         ▼                       │
                    ┌────────────┐               │
                    │ OIDC       │───────────────┘
                    │ Provider   │
                    └────────────┘
```

## Features

### Server-side (Proxy Server)
- Validates OIDC tokens from clients
- Forwards authenticated requests to MCP server(s)
- Configurable trusted issuer and token claim validation
- Supports multiple MCP backends with Anthropic-style configuration
- Multiple transport types (HTTP and stdio) for backend servers
- Path-based routing with optional path prefix stripping
- Local MCP process management with stdio communication
- Models API for discovering available models
- Supports scalable deployment
- Health check endpoints
- Structured logging and metrics

### Client-side (Proxy Client)
- Implements client credentials flow to acquire tokens
- Acts as a local unauthenticated MCP service
- Proxies requests to the server-side component
- Automatic token refresh
- Health check endpoints
- Structured logging

## Installation

### Requirements
- Go 1.24 or higher

### Building from Source

```sh
# Build the server component
go build -o smcp-proxy-server ./cmd/proxy-server

# Build the client component
go build -o smcp-proxy-client ./cmd/proxy-client
```

## Configuration

Configuration is provided via YAML files. Sample configurations are available in the `configs` directory.

### Server Configuration

Server configuration is specified in `configs/proxy-server.yml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  shutdown_timeout: "10s"

mcp:
  # Global timeout for all backends (can be overridden per backend)
  timeout: "60s"
  
  # Configure multiple MCP backends
  backends:
    # HTTP backend for Claude 3 Opus
    - id: "claude-3-opus"
      name: "Claude 3 Opus"
      model: "claude-3-opus-20240229"
      max_tokens: 200000
      transport: "http"
      url: "http://mcp-opus.example.com"
      path: "/v1/opus"
      strip_path: true
      timeout: "120s"  # Override global timeout
    
    # HTTP backend for Claude 3 Sonnet
    - id: "claude-3-sonnet"
      name: "Claude 3 Sonnet"
      model: "claude-3-sonnet-20240229"
      max_tokens: 180000
      transport: "http"
      url: "http://mcp-sonnet.example.com"
      path: "/v1/sonnet"
      strip_path: true
    
    # Local stdio backend for Claude 3 Haiku with subprocess
    - id: "claude-3-haiku"
      name: "Claude 3 Haiku"
      model: "claude-3-haiku-20240307"
      max_tokens: 150000
      transport: "stdio"
      path: "/v1/haiku"
      strip_path: true
      stdio:
        command: "python"
        args: ["-m", "anthropic.mcp_server", "--model", "claude-3-haiku-20240307"]
        working_dir: "/opt/anthropic"
        env:
          ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
          PYTHONPATH: "/opt/anthropic"
        stdio_timeout: "60s"
    
    # Default backend for all other paths
    - id: "default-mcp"
      name: "Default MCP"
      transport: "http"
      url: "http://mcp-default.example.com"
      path: "/"
      strip_path: false

oidc:
  issuers:
    - "https://your-identity-provider.com"  # Replace with your actual OIDC issuer URL
  audience: "your-api-audience"            # Replace with your API audience
  required_claims:
    # Define required claims that must be present in the token
    # For example:
    # roles: "admin"
  optional_claims:
    # Define optional claims that if present must match specific values
    # For example:
    # scope: "read:data"

tls:
  enabled: false
  # cert_file: "/path/to/cert.pem"
  # key_file: "/path/to/key.pem"

metrics:
  enabled: true
  path: "/metrics"
```

### Client Configuration

Client configuration is specified in `configs/proxy-client.yml`:

```yaml
client:
  host: "127.0.0.1"
  port: 8081
  read_timeout: "30s"
  write_timeout: "30s"
  shutdown_timeout: "10s"

server:
  url: "http://localhost:8080"  # URL of the proxy server
  timeout: "60s"

oidc:
  issuer: "https://your-identity-provider.com"  # Replace with your actual OIDC issuer URL
  client_id: "your-client-id"                   # Replace with your client ID
  client_secret: "your-client-secret"           # Replace with your client secret
  audience: "your-api-audience"                 # Replace with your API audience
  scopes:
    - "openid"
    # Add additional scopes as needed
  cache_ttl: "5m"         # Cache tokens for 5 minutes
  token_ttl_delta: "30s"  # Refresh tokens 30 seconds before they expire

tls:
  enabled: false
  # cert_file: "/path/to/cert.pem"
  # key_file: "/path/to/key.pem"

metrics:
  enabled: true
  path: "/metrics"
```

Both configurations can be overridden using environment variables with the prefix `SMCP_PROXY_` for the server and `SMCP_CLIENT_` for the client.

## Usage

### Starting the Server

```sh
# Build the server
go build -o smcp-proxy ./cmd/proxy-server

# Run the server
./smcp-proxy server --config=configs/proxy-server.yml --log-level=debug
```

### Starting the Client

```sh
# Build the client
go build -o smcp-proxy ./cmd/proxy-client

# Run the client
./smcp-proxy client --config=configs/proxy-client.yml --log-level=debug
```

Once both components are running:
1. Applications can connect to the client component (by default at http://localhost:8081)
2. The client component authenticates with the OIDC provider and forwards requests to the server
3. The server component validates tokens and forwards authenticated requests to the appropriate MCP backend based on the request path

### Multiple Backend Support

The proxy server supports multiple MCP backends with different transport types:

```
┌─────────────┐                  ┌───────────────────────────────┐
│ Request to  │                  │        Proxy Server           │
│ /v1/opus/.. ├─────────────────►│                               │──► HTTP Claude 3 Opus Backend
└─────────────┘                  │                               │
                                 │                               │
┌─────────────┐                  │                               │
│ Request to  │                  │    OIDC Authentication +      │
│ /v1/sonnet/ ├─────────────────►│     Path-Based Routing        │──► HTTP Claude 3 Sonnet Backend
└─────────────┘                  │                               │
                                 │                               │
┌─────────────┐                  │                               │
│ Request to  │                  │                               │
│ /v1/haiku/  ├─────────────────►│                               │──► Stdio Claude 3 Haiku (Local Process)
└─────────────┘                  └───────────────────────────────┘
```

#### Path-Based Routing

With the `strip_path` option enabled, the proxy will remove the path prefix before forwarding the request to the backend:

- Request to `/v1/opus/completions` → Forwarded to Opus backend as `/completions`
- Request to `/v1/sonnet/messages` → Forwarded to Sonnet backend as `/messages`
- Request to `/v1/haiku/chat` → Processed by local Haiku process as `/chat`

#### Transport Types

The proxy supports two types of backends:

1. **HTTP Backends** (`transport: "http"`):
   - Remote MCP servers accessible via HTTP
   - Configured with a URL and standard proxy settings
   - Example: `url: "http://mcp-opus.example.com"`

2. **Stdio Backends** (`transport: "stdio"`):
   - Local MCP servers running as subprocesses
   - Communication via standard input/output
   - Useful for running local model servers
   - Example:
     ```yaml
     stdio:
       command: "python"
       args: ["-m", "anthropic.mcp_server", "--model", "claude-3-haiku"]
       working_dir: "/opt/anthropic"
       env:
         ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
     ```

#### Models API

The proxy provides a `/api/models` endpoint that returns information about all configured backends, following the Anthropic API models format. This allows clients to discover available models and their capabilities.

## Development

### Project Structure

```
.
├── cmd/                    # Application entry points
│   ├── proxy-server/       # Server binary entry point
│   └── proxy-client/       # Client binary entry point
├── configs/                # Configuration files
│   ├── proxy-server.yml    # Server configuration
│   └── proxy-client.yml    # Client configuration
├── internal/               # Private application code
│   ├── middleware/         # HTTP middleware
│   │   ├── logging.go      # Request logging middleware
│   │   └── recovery.go     # Panic recovery middleware
│   └── metrics/            # Metrics implementation (placeholder)
├── pkg/                    # Public API
│   ├── auth/               # Authentication components
│   │   ├── validator.go    # OIDC token validation
│   │   └── client.go       # OIDC client credentials flow
│   ├── cmd/                # Command line interface
│   │   ├── root.go         # Root command
│   │   ├── server.go       # Server command
│   │   └── client.go       # Client command
│   ├── config/             # Configuration handling
│   │   ├── server_config.go # Server configuration
│   │   └── client_config.go # Client configuration
│   └── proxy/              # Proxy implementation
│       ├── server.go       # Server-side proxy
│       └── client.go       # Client-side proxy
├── go.mod                  # Go module definition
└── README.md               # This file
```

### Design Principles

1. **Security First**: All authentication and authorization are implemented with security best practices.
2. **Scalability**: The proxy is designed to handle multiple MCP servers and clients.
3. **Configurability**: Extensive configuration options allow customization for different environments.
4. **Resilience**: The system handles failures gracefully, with proper error handling and recovery.
5. **Observability**: Comprehensive logging and metrics support to monitor the system's behavior.

## License

[MIT](LICENSE)