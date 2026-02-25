# PicoClaw Qdrant Integration

This document describes how to configure PicoClaw to use Qdrant vector database for storing and searching chat messages with Mistral embeddings.

## Overview

PicoClaw can store all chat messages in a Qdrant vector database, enabling:
- **Persistent storage** of all conversations across sessions
- **Semantic search** through message history using vector embeddings
- **Context retrieval** for better AI responses

## Requirements

1. **Qdrant instance** - Running Qdrant vector database (local or cloud)
2. **Mistral API Key** - For generating embeddings using `mistral-embed` model

## Quick Start

### 1. Start Qdrant (Local)

```bash
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

### 2. Configure PicoClaw

Edit your `config.json` or use environment variables:

#### Using JSON config:

```json
{
  "storage": {
    "qdrant": {
      "enabled": true,
      "host": "localhost",
      "port": 6333,
      "collection": "picoclaw_messages",
      "vector_size": 1024
    }
  },
  "model_list": [
    {
      "model_name": "mistral-embed",
      "model": "mistral/mistral-embed",
      "api_base": "https://api.mistral.ai/v1",
      "api_key": "your-mistral-api-key"
    }
  ]
}
```

#### Using environment variables:

```bash
# Enable Qdrant storage
export PICOCLAW_STORAGE_QDRANT_ENABLED=true
export PICOCLAW_STORAGE_QDRANT_HOST=localhost
export PICOCLAW_STORAGE_QDRANT_PORT=6333
export PICOCLAW_STORAGE_QDRANT_COLLECTION=picoclaw_messages
export PICOCLAW_STORAGE_QDRANT_VECTOR_SIZE=1024

# Mistral API for embeddings
export PICOCLAW_EMBEDDING_API_KEY=your-mistral-api-key
export PICOCLAW_EMBEDDING_MODEL=mistral-embed
```

### 3. Get Mistral API Key

1. Visit [Mistral AI Platform](https://console.mistral.ai/api-keys)
2. Create a new API key
3. Add it to your configuration

## Configuration Options

### Qdrant Configuration

| Field | Environment Variable | Default | Description |
|-------|---------------------|---------|-------------|
| `enabled` | `PICOCLAW_STORAGE_QDRANT_ENABLED` | `false` | Enable/disable Qdrant storage |
| `host` | `PICOCLAW_STORAGE_QDRANT_HOST` | `localhost` | Qdrant server hostname |
| `port` | `PICOCLAW_STORAGE_QDRANT_PORT` | `6333` | Qdrant HTTP port |
| `grpc_port` | `PICOCLAW_STORAGE_QDRANT_GRPC_PORT` | `6334` | Qdrant gRPC port (optional) |
| `api_key` | `PICOCLAW_STORAGE_QDRANT_API_KEY` | `""` | API key for Qdrant Cloud |
| `collection` | `PICOCLAW_STORAGE_QDRANT_COLLECTION` | `picoclaw_messages` | Collection name |
| `vector_size` | `PICOCLAW_STORAGE_QDRANT_VECTOR_SIZE` | `1024` | Embedding dimension (mistral-embed = 1024) |
| `secure` | `PICOCLAW_STORAGE_QDRANT_SECURE` | `false` | Use HTTPS |

### Embedding Configuration

| Field | Environment Variable | Default | Description |
|-------|---------------------|---------|-------------|
| `enabled` | `PICOCLAW_EMBEDDING_ENABLED` | `false` | Enable embedding generation |
| `model` | `PICOCLAW_EMBEDDING_MODEL` | `mistral-embed` | Embedding model name |
| `api_base` | `PICOCLAW_EMBEDDING_API_BASE` | `https://api.mistral.ai/v1` | API endpoint |
| `api_key` | `PICOCLAW_EMBEDDING_API_KEY` | `""` | API key for embeddings |

## How It Works

1. **Message Storage**: When a message is received, PicoClaw:
   - Sends the message content to Mistral API for embedding generation
   - Stores the message with its embedding vector in Qdrant
   - Also saves locally in session JSON files

2. **Semantic Search**: The AI can search for similar messages using:
   - Vector similarity (cosine distance)
   - Session-based filtering
   - Configurable result limits

3. **Data Structure**: Each stored message contains:
   - `session_key`: Unique session identifier
   - `role`: Message role (user/assistant/system)
   - `content`: Message text
   - `tool_calls`: Associated tool calls (if any)
   - `timestamp`: When the message was stored
   - `message_index`: Position in conversation

## Qdrant Cloud

To use Qdrant Cloud instead of local instance:

```json
{
  "storage": {
    "qdrant": {
      "enabled": true,
      "host": "your-cluster-id.cloud.qdrant.io",
      "port": 443,
      "api_key": "your-qdrant-cloud-api-key",
      "secure": true,
      "collection": "picoclaw_messages",
      "vector_size": 1024
    }
  }
}
```

## Troubleshooting

### Connection Errors

- Ensure Qdrant is running: `curl http://localhost:6333`
- Check firewall settings for port 6333
- Verify host/port configuration

### Embedding Errors

- Verify Mistral API key is valid
- Check API quota limits at [Mistral Dashboard](https://console.mistral.ai/)
- Ensure network connectivity to `api.mistral.ai`

### Collection Not Created

- Qdrant collection is auto-created on first message
- Check PicoClaw logs for creation errors
- Verify Qdrant has sufficient disk space

## Performance Considerations

- **Embedding Generation**: Each message requires one API call to Mistral
- **Vector Storage**: ~4KB per message in Qdrant (1024 float32 + metadata)
- **Search Speed**: Typically <100ms for semantic search
- **Batch Operations**: Multiple messages can be stored in batch for efficiency

## Security Notes

- Store API keys securely, never commit to version control
- Use Qdrant Cloud with HTTPS for production
- Consider message content sensitivity before storing
- Regular backups of Qdrant data recommended

## Example Usage

After configuration, all messages are automatically stored. To search:

```go
// In your code, use the session manager
messages, err := sessionManager.SearchSimilarMessages(
    "session:123",     // session key
    "How do I install Docker?",  // search query
    5,  // limit results
)
```

## License

MIT License - See main project LICENSE for details
