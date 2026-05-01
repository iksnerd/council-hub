#!/bin/bash
#
# Council Hub MCP API Examples
# Use these as templates for curl requests to council-hub
#
# Prerequisites:
#   - council-hub running on http://localhost:3001
#   - jq installed (for parsing JSON responses)
#
# MCP transport: HTTP/SSE
# Docs: https://modelcontextprotocol.io/
#

BASE_URL="http://localhost:3001/mcp"

# =============================================================================
# ROOMS
# =============================================================================

# Create a new room
create_room() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "1",
      "method": "tools/call",
      "params": {
        "name": "create_room",
        "arguments": {
          "id": "research-auth-2024",
          "topic": "Researching modern authentication patterns for 2024",
          "project": "backend",
          "tech_stack": "Go, PostgreSQL, Redis",
          "tags": "security,auth,research",
          "system_prompt": "Focus on OAuth2, OIDC, and zero-trust patterns. Evaluate trade-offs between complexity and security."
        }
      }
    }' | jq .
}

# Get a room (with metadata and recent messages)
get_room() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "2",
      "method": "tools/call",
      "params": {
        "name": "get_or_create_room",
        "arguments": {
          "id": "research-auth-2024",
          "last_n": 10
        }
      }
    }' | jq .
}

# List rooms with filters
list_rooms() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "3",
      "method": "tools/call",
      "params": {
        "name": "list_rooms",
        "arguments": {
          "project": "backend",
          "tag": "security",
          "status": "active",
          "limit": 20
        }
      }
    }' | jq .
}

# =============================================================================
# MESSAGES
# =============================================================================

# Post a message to a room
post_message() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "4",
      "method": "tools/call",
      "params": {
        "name": "post_to_room",
        "arguments": {
          "room_id": "research-auth-2024",
          "author": "claude-research",
          "message": "I analyzed the current OAuth2 implementation. The token refresh flow is vulnerable to race conditions. Proposing we switch to refresh token rotation with sliding window.",
          "message_type": "thought"
        }
      }
    }' | jq .
}

# Search messages (keyword search)
search_messages_keyword() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "5",
      "method": "tools/call",
      "params": {
        "name": "search_messages",
        "arguments": {
          "query": "token refresh vulnerability",
          "room_id": "research-auth-2024",
          "limit": 10
        }
      }
    }' | jq .
}

# Search messages (semantic search - requires Ollama)
search_messages_semantic() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "6",
      "method": "tools/call",
      "params": {
        "name": "search_messages",
        "arguments": {
          "query": "how should we handle session expiration",
          "semantic": "true",
          "limit": 10
        }
      }
    }' | jq .
}

# Read full transcript
read_transcript() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "7",
      "method": "tools/call",
      "params": {
        "name": "read_transcript",
        "arguments": {
          "room_id": "research-auth-2024",
          "mode": "summary"
        }
      }
    }' | jq .
}

# =============================================================================
# DECISIONS & STATUS
# =============================================================================

# Post a decision message
post_decision() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "8",
      "method": "tools/call",
      "params": {
        "name": "post_to_room",
        "arguments": {
          "room_id": "research-auth-2024",
          "author": "claude-research",
          "message": "Decision: We will implement refresh token rotation with sliding window expiration. This mitigates race conditions and provides better UX than hard token expiry.",
          "message_type": "decision"
        }
      }
    }' | jq .
}

# Update room status
update_status() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "9",
      "method": "tools/call",
      "params": {
        "name": "signal_status",
        "arguments": {
          "room_id": "research-auth-2024",
          "status": "resolved"
        }
      }
    }' | jq .
}

# =============================================================================
# ANALYTICS
# =============================================================================

# Get room stats
room_stats() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "10",
      "method": "tools/call",
      "params": {
        "name": "room_stats",
        "arguments": {
          "room_id": "research-auth-2024"
        }
      }
    }' | jq .
}

# Get activity digest since timestamp
get_digest() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "11",
      "method": "tools/call",
      "params": {
        "name": "get_digest",
        "arguments": {
          "project": "backend",
          "since": "'$(date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%SZ)'"
        }
      }
    }' | jq .
}

# =============================================================================
# CLUSTERING
# =============================================================================

# List rooms across all cluster nodes
list_rooms_cluster_wide() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "12",
      "method": "tools/call",
      "params": {
        "name": "list_rooms",
        "arguments": {
          "cluster_wide": "true",
          "limit": 20
        }
      }
    }' | jq .
}

# Search across all nodes
search_cluster_wide() {
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": "13",
      "method": "tools/call",
      "params": {
        "name": "search_messages",
        "arguments": {
          "query": "security vulnerability",
          "cluster_wide": "true",
          "limit": 20
        }
      }
    }' | jq .
}

# =============================================================================
# USAGE
# =============================================================================
#
# Run any function from this script:
#   bash examples/api-samples.sh create_room
#   bash examples/api-samples.sh search_messages_keyword
#   bash examples/api-samples.sh read_transcript
#
# Or source it and call functions directly:
#   source examples/api-samples.sh
#   create_room
#   list_rooms
#   post_message
#
