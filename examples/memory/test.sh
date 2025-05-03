#!/bin/bash
set -e

echo "Starting SMCP Proxy with Memory MCP..."
docker-compose up -d

echo "Waiting for services to be ready..."
sleep 5

echo "Checking service health..."
echo "SMCP Client: $(curl -s http://localhost:8081/health)"
echo "SMCP Server: $(curl -s http://localhost:8080/health)"

echo -e "\nChecking available models..."
curl -s http://localhost:8081/api/models | jq

echo -e "\nCreating a memory session..."
curl -s -X POST http://localhost:8081/v1/memory/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "test-session",
    "metadata": {
      "user_id": "test-user",
      "description": "Test memory session"
    }
  }' | jq

echo -e "\nAdding user message to memory..."
curl -s -X POST http://localhost:8081/v1/memory/sessions/test-session/messages \
  -H "Content-Type: application/json" \
  -d '{
    "role": "user",
    "content": "Hello, can you remember this message?"
  }' | jq

echo -e "\nAdding assistant message to memory..."
curl -s -X POST http://localhost:8081/v1/memory/sessions/test-session/messages \
  -H "Content-Type: application/json" \
  -d '{
    "role": "assistant",
    "content": "Yes, I will remember this message."
  }' | jq

echo -e "\nRetrieving all messages from memory..."
curl -s http://localhost:8081/v1/memory/sessions/test-session/messages | jq

echo -e "\nListing all sessions..."
curl -s http://localhost:8081/v1/memory/sessions | jq

echo -e "\nTest completed successfully!"
echo "To stop the services, run: docker-compose down"