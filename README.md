# SMCP Proxy

A secure reverse proxy for Model Customization Platform (MCP) services with OIDC authentication.

## Overview

SMCP Proxy provides a secure layer in front of MCP services, enabling enterprise-grade authentication and authorization using OIDC. It consists of two main components:

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
- Supports multiple MCP backends exposed under different paths
- Path-based routing with optional path prefix stripping
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
  
  # Configure multiple MCP backends with different paths
  backends:
    # Models API backend
    - name: "models-api"
      url: "http://mcp-models.example.com"
      path: "/models"
      strip_path: true
      timeout: "120s"  # Override global timeout
    
    # Inference API backend
    - name: "inference-api"
      url: "http://mcp-inference.example.com"
      path: "/inference"
      strip_path: true
    
    # Default backend for all other paths
    - name: "default"
      url: "http://mcp-default.example.com"
      path: "/"
      strip_path: false
      
  # Legacy configuration (deprecated, use backends instead)
  # endpoints:
  #  - "http://localhost:9000"

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

### Path-Based Routing

The proxy server supports multiple MCP backends exposed under different paths:

```
┌─────────────┐                  ┌───────────────────────────────┐
│ Request to  │                  │        Proxy Server           │
│ /models/... ├─────────────────►│                               │──► Models MCP Backend
└─────────────┘                  │                               │
                                 │                               │
┌─────────────┐                  │                               │
│ Request to  │                  │    OIDC Authentication +      │
│ /inference/ ├─────────────────►│     Path-Based Routing        │──► Inference MCP Backend
└─────────────┘                  │                               │
                                 │                               │
┌─────────────┐                  │                               │
│ Request to  │                  │                               │
│ /other/...  ├─────────────────►│                               │──► Default MCP Backend
└─────────────┘                  └───────────────────────────────┘
```

With the `strip_path` option enabled, the proxy will remove the path prefix before forwarding the request to the backend:

- Request to `/models/my-model` → Forwarded to Models MCP as `/my-model`
- Request to `/inference/predict` → Forwarded to Inference MCP as `/predict`
- Request to `/other/endpoint` → Forwarded to Default MCP as `/other/endpoint` (no stripping)

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