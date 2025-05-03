# SMCP Proxy Examples

This directory contains practical examples for using SMCP Proxy in various scenarios.

## Available Examples

### [Memory MCP](./memory)

A complete example demonstrating how to use SMCP Proxy with a Memory MCP server. This setup allows:
- Creation and management of memory sessions
- Storing and retrieving conversation history
- Integration with LLMs that need conversation context

The example includes:
- Docker Compose configuration
- Server configuration
- Test script
- Detailed instructions with curl examples

## Running the Examples

Each example directory contains its own README with specific instructions. The examples use Docker Compose for ease of setup and testing.

## Creating Your Own Examples

To create your own SMCP Proxy setup:

1. Start with the server configuration (modify an existing `server.yml`)
2. Configure one or more MCP backends
3. Choose the appropriate transport (HTTP or stdio)
4. Select the authentication mode (none or OIDC)
5. Launch both the server and client components

Refer to the main [README](../README.md) for detailed configuration options.