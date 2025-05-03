# Memory MCP Server Example

This example demonstrates how to set up SMCP Proxy with a Memory MCP server. The Memory MCP server provides conversation history retention capabilities, allowing LLM applications to retrieve and update conversation state.

## Architecture

```
┌───────────────┐      ┌───────────────┐      ┌───────────────┐      ┌───────────────┐
│  Application  │      │ SMCP Client   │      │ SMCP Server   │      │   Memory MCP  │
│  (curl/apps)  │──────│ at :8081      │──────│ at :8080      │──────│   Server      │
└───────────────┘      └───────────────┘      └───────────────┘      └───────────────┘
                                                                       │
                                                                       │ 
                                                                       ▼
                                                                    ┌─────────┐
                                                                    │ Volume  │
                                                                    │ Storage │
                                                                    └─────────┘
```

## Prerequisites

- Docker and Docker Compose installed
- Basic understanding of HTTP APIs
- Free ports 8080 and 8081 on your host machine

## Setup

1. Navigate to this example directory:
   ```bash
   cd examples/memory
   ```

2. Start the Docker Compose stack:
   ```bash
   docker-compose up -d
   ```

3. Check that all services are running:
   ```bash
   docker-compose ps
   ```

## Configuration

The setup includes:

1. **Memory MCP Server**: Runs the official Memory MCP server container
2. **SMCP Proxy Server**: Connects to the Memory MCP server and exposes it at `/v1/memory/*`
3. **SMCP Proxy Client**: Provides an unauthenticated endpoint for applications

The server configuration (`configs/server.yml`) is set up for HTTP transport to communicate with the Memory MCP server running as a separate container.

## Testing the Setup

### Check Available Models

First, verify that the SMCP Proxy is correctly exposing the Memory MCP server:

```bash
curl http://localhost:8081/api/models | jq
```

You should see output similar to:

```json
{
  "object": "list",
  "data": [
    {
      "ID": "memory",
      "Name": "Memory",
      "Model": "memory",
      "MaxTokens": 100000,
      "Path": "/v1/memory"
    }
  ]
}
```

### Health Checks

Verify that both the client and server are healthy:

```bash
curl http://localhost:8081/health
curl http://localhost:8080/health
```

Both should return `OK`.

### Creating a Memory Session

Create a new memory session:

```bash
curl -X POST http://localhost:8081/v1/memory/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "test-session",
    "metadata": {
      "user_id": "test-user",
      "description": "Test memory session"
    }
  }'
```

You should receive a response confirming the session creation.

### Adding Messages to Memory

Add a message to the memory session:

```bash
curl -X POST http://localhost:8081/v1/memory/sessions/test-session/messages \
  -H "Content-Type: application/json" \
  -d '{
    "role": "user",
    "content": "Hello, can you remember this message?"
  }'
```

Add an assistant response:

```bash
curl -X POST http://localhost:8081/v1/memory/sessions/test-session/messages \
  -H "Content-Type: application/json" \
  -d '{
    "role": "assistant",
    "content": "Yes, I will remember this message."
  }'
```

### Retrieving Memory

Get all messages from the session:

```bash
curl http://localhost:8081/v1/memory/sessions/test-session/messages | jq
```

You should see both messages you added.

### Updating Metadata

Update session metadata:

```bash
curl -X PATCH http://localhost:8081/v1/memory/sessions/test-session \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {
      "user_id": "test-user",
      "description": "Updated memory session description"
    }
  }'
```

### Listing Sessions

List all available sessions:

```bash
curl http://localhost:8081/v1/memory/sessions | jq
```

## Real-world Integration

To integrate this with an LLM application:

1. Configure your LLM application to use the Memory MCP endpoint at `http://localhost:8081/v1/memory`
2. Create a session for each user or conversation
3. Before each LLM request, retrieve the conversation history
4. After each LLM request, store the new messages

## Cleanup

To stop all services and remove containers:

```bash
docker-compose down
```

To completely clean up, including the persistent volume:

```bash
docker-compose down -v
```

## Troubleshooting

- **Client can't connect to server**: Check that the server is running and ports are correct
- **Server can't connect to Memory MCP**: Check Docker network connectivity
- **HTTP 404 errors**: Ensure you're using the correct path prefix (/v1/memory)
- **Data not persisting**: Check the Docker volume is mounted correctly