<div align="center">
  <img src="assets/logo.jpg" alt="PicoClaw" width="512">

  <h1>PicoClaw Fork</h1>

</div>

---

⚡️ Original repo [picoclaw](https://github.com/sipeed/picoclaw).

## ✨ New Features

* Context Window & Compaction Management
* Qdrant Vector Storage
* Session Management
* Forum topics (Telegram)

## 📦 Install

### Install from source (latest features, recommended for development)

```bash
git clone https://github.com/waplaysoft/picoclaw.git

cd picoclaw
make deps

# Build, no need to install
make build

# Build for multiple platforms
make build-all

# Build And Install
make install
```

## 🐳 Docker Compose

You can also run PicoClaw using Docker Compose without installing anything locally.

```bash
# 1. Clone this repo
git clone https://github.com/sipeed/picoclaw.git
cd picoclaw

# 2. Set your API keys
cp config/config.example.json config/config.json
vim config/config.json      # Set DISCORD_BOT_TOKEN, API keys, etc.

# 3. Build & Start
docker compose --profile gateway up -d

> [!TIP]
> **Docker Users**: By default, the Gateway listens on `127.0.0.1` which is not accessible from the host. If you need to access the health endpoints or expose ports, set `PICOCLAW_GATEWAY_HOST=0.0.0.0` in your environment or update `config.json`.


# 4. Check logs
docker compose logs -f picoclaw-gateway

# 5. Stop
docker compose --profile gateway down
```

### Agent Mode (One-shot)

```bash
# Ask a question
docker compose run --rm picoclaw-agent -m "What is 2+2?"

# Interactive mode
docker compose run --rm picoclaw-agent
```

### Rebuild

```bash
docker compose --profile gateway build --no-cache
docker compose --profile gateway up -d
```

### 🚀 Quick Start

> [!TIP]
> Set your API key in `~/.picoclaw/config.json`.
> Get API keys: [OpenRouter](https://openrouter.ai/keys) (LLM) · [Zhipu](https://open.bigmodel.cn/usercenter/proj-mgmt/apikeys) (LLM)
> Web Search is **optional** - get free [Tavily API](https://tavily.com) (1000 free queries/month) or [Brave Search API](https://brave.com/search/api) (2000 free queries/month) or use built-in auto fallback.

**1. Initialize**

```bash
picoclaw onboard
```

**2. Configure** (`~/.picoclaw/config.json`)

```json
{
  "defaults": {
      "workspace": "~/.picoclaw/workspace",
      "restrict_to_workspace": true,
      "model": "gpt4",
      "context_window": 200000,
      "max_tokens": 4096,
      "temperature": 0.7,
      "max_tool_iterations": 40,
      "compaction": {
        "reserve_tokens_floor": 20000,
        "keep_recent_tokens": 20000
      }
    }
  "model_list": [
    {
      "model_name": "gpt4",
      "model": "openai/gpt-4",
      "api_key": "your-api-key"
    },
    {
      "model_name": "claude-sonnet-4.6",
      "model": "anthropic/claude-sonnet-4.6",
      "api_key": "your-anthropic-key"
    }
  ],
  "tools": {
    "web": {
      "brave": {
        "enabled": false,
        "api_key": "YOUR_BRAVE_API_KEY",
        "max_results": 5
      },
      "tavily": {
        "enabled": false,
        "api_key": "YOUR_TAVILY_API_KEY",
        "max_results": 5
      },
      "duckduckgo": {
        "enabled": true,
        "max_results": 5
      }
    }
  }
}
```

> **Note**: See `config.example.json` for a complete configuration template.

**4. Chat**

```bash
picoclaw agent -m "What is 2+2?"
```

That's it! You have a working AI assistant in 2 minutes.

---

## 💬 Chat Apps

Talk to your picoclaw through Telegram, Discord, DingTalk, LINE, or WeCom

| Channel      | Setup                                   |
| ------------ | --------------------------------------- |
| **Telegram** | Easy (just a token) · Topics support ✅ |
| **Discord**  | Easy (bot token + intents)              |
| **QQ**       | Easy (AppID + AppSecret)                |
| **DingTalk** | Medium (app credentials)                |
| **LINE**     | Medium (credentials + webhook URL)      |
| **WeCom**    | Medium (CorpID + webhook setup)         |

<details>
<summary><b>Telegram</b> (Recommended)</summary>

**1. Create a bot**

* Open Telegram, search `@BotFather`
* Send `/newbot`, follow prompts
* Copy the token

**2. Configure**

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allow_from": ["YOUR_USER_ID"]
    }
  }
}
```

> Get your user ID from `@userinfobot` on Telegram.

**3. Run**

```bash
picoclaw gateway
```

</details>

<details>
<summary><b>Discord</b></summary>

**1. Create a bot**

* Go to <https://discord.com/developers/applications>
* Create an application → Bot → Add Bot
* Copy the bot token

**2. Enable intents**

* In the Bot settings, enable **MESSAGE CONTENT INTENT**
* (Optional) Enable **SERVER MEMBERS INTENT** if you plan to use allow lists based on member data

**3. Get your User ID**
* Discord Settings → Advanced → enable **Developer Mode**
* Right-click your avatar → **Copy User ID**

**4. Configure**

```json
{
  "channels": {
    "discord": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allow_from": ["YOUR_USER_ID"],
      "mention_only": false
    }
  }
}
```

**5. Invite the bot**

* OAuth2 → URL Generator
* Scopes: `bot`
* Bot Permissions: `Send Messages`, `Read Message History`
* Open the generated invite URL and add the bot to your server

**Optional: Mention-only mode**

Set `"mention_only": true` to make the bot respond only when @-mentioned. Useful for shared servers where you want the bot to respond only when explicitly called.

**6. Run**

```bash
picoclaw gateway
```

</details>

<details>
<summary><b>QQ</b></summary>

**1. Create a bot**

- Go to [QQ Open Platform](https://q.qq.com/#)
- Create an application → Get **AppID** and **AppSecret**

**2. Configure**

```json
{
  "channels": {
    "qq": {
      "enabled": true,
      "app_id": "YOUR_APP_ID",
      "app_secret": "YOUR_APP_SECRET",
      "allow_from": []
    }
  }
}
```

> Set `allow_from` to empty to allow all users, or specify QQ numbers to restrict access.

**3. Run**

```bash
picoclaw gateway
```

</details>

<details>
<summary><b>DingTalk</b></summary>

**1. Create a bot**

* Go to [Open Platform](https://open.dingtalk.com/)
* Create an internal app
* Copy Client ID and Client Secret

**2. Configure**

```json
{
  "channels": {
    "dingtalk": {
      "enabled": true,
      "client_id": "YOUR_CLIENT_ID",
      "client_secret": "YOUR_CLIENT_SECRET",
      "allow_from": []
    }
  }
}
```

> Set `allow_from` to empty to allow all users, or specify DingTalk user IDs to restrict access.

**3. Run**

```bash
picoclaw gateway
```
</details>

<details>
<summary><b>LINE</b></summary>

**1. Create a LINE Official Account**

- Go to [LINE Developers Console](https://developers.line.biz/)
- Create a provider → Create a Messaging API channel
- Copy **Channel Secret** and **Channel Access Token**

**2. Configure**

```json
{
  "channels": {
    "line": {
      "enabled": true,
      "channel_secret": "YOUR_CHANNEL_SECRET",
      "channel_access_token": "YOUR_CHANNEL_ACCESS_TOKEN",
      "webhook_host": "0.0.0.0",
      "webhook_port": 18791,
      "webhook_path": "/webhook/line",
      "allow_from": []
    }
  }
}
```

**3. Set up Webhook URL**

LINE requires HTTPS for webhooks. Use a reverse proxy or tunnel:

```bash
# Example with ngrok
ngrok http 18791
```

Then set the Webhook URL in LINE Developers Console to `https://your-domain/webhook/line` and enable **Use webhook**.

**4. Run**

```bash
picoclaw gateway
```

> In group chats, the bot responds only when @mentioned. Replies quote the original message.

> **Docker Compose**: Add `ports: ["18791:18791"]` to the `picoclaw-gateway` service to expose the webhook port.

</details>

<details>
<summary><b>WeCom (企业微信)</b></summary>

PicoClaw supports two types of WeCom integration:

**Option 1: WeCom Bot (智能机器人)** - Easier setup, supports group chats
**Option 2: WeCom App (自建应用)** - More features, proactive messaging

See [WeCom App Configuration Guide](docs/wecom-app-configuration.md) for detailed setup instructions.

**Quick Setup - WeCom Bot:**

**1. Create a bot**

* Go to WeCom Admin Console → Group Chat → Add Group Bot
* Copy the webhook URL (format: `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx`)

**2. Configure**

```json
{
  "channels": {
    "wecom": {
      "enabled": true,
      "token": "YOUR_TOKEN",
      "encoding_aes_key": "YOUR_ENCODING_AES_KEY",
      "webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY",
      "webhook_host": "0.0.0.0",
      "webhook_port": 18793,
      "webhook_path": "/webhook/wecom",
      "allow_from": []
    }
  }
}
```

**Quick Setup - WeCom App:**

**1. Create an app**

* Go to WeCom Admin Console → App Management → Create App
* Copy **AgentId** and **Secret**
* Go to "My Company" page, copy **CorpID**
**2. Configure receive message**

* In App details, click "Receive Message" → "Set API"
* Set URL to `http://your-server:18792/webhook/wecom-app`
* Generate **Token** and **EncodingAESKey**

**3. Configure**

```json
{
  "channels": {
    "wecom_app": {
      "enabled": true,
      "corp_id": "wwxxxxxxxxxxxxxxxx",
      "corp_secret": "YOUR_CORP_SECRET",
      "agent_id": 1000002,
      "token": "YOUR_TOKEN",
      "encoding_aes_key": "YOUR_ENCODING_AES_KEY",
      "webhook_host": "0.0.0.0",
      "webhook_port": 18792,
      "webhook_path": "/webhook/wecom-app",
      "allow_from": []
    }
  }
}
```

**4. Run**

```bash
picoclaw gateway
```

> **Note**: WeCom App requires opening port 18792 for webhook callbacks. Use a reverse proxy for HTTPS.

</details>

## ⚙️ Configuration

Config file: `~/.picoclaw/config.json`

### Context Window Management

PicoClaw provides advanced context window management to balance memory usage and conversation quality.

| Parameter                | Default              | Description                                          |
| ------------------------ | --------------------- | ---------------------------------------------------- |
| `context_window`       | Model default         | Maximum input context tokens                |
| `max_tokens`           | Model default         | Maximum tokens in LLM response                |
| `reserve_tokens_floor` | 20000                 | Min tokens to leave free after compression  |
| `keep_recent_tokens`  | 20000                 | Tokens to keep after compression             |

#### Compaction Behavior

PicoClaw uses a smart token-based compaction strategy:

- **Trigger**: When context reaches 85-90% of `context_window`
- **Action**: Compress history and keep last `keep_recent_tokens` tokens
- **Reserve**: Always leave at least `reserve_tokens_floor` tokens free

This prevents aggressive compression and maintains longer conversation context compared to fixed message limits.

**Example configuration:**

```json
"agents": {
  "defaults": {
    "model": "glm-4.7",
    "context_window": 200000,
    "max_tokens": 4096,
    "compaction": {
      "reserve_tokens_floor": 20000,
      "keep_recent_tokens": 20000
    }
  }
}
```

> **Note**: `context_window` and `max_tokens` are separate concepts. `context_window` controls input size, `max_tokens` controls output size.

### Workspace Layout

PicoClaw stores data in your configured workspace (default: `~/.picoclaw/workspace`):

```
~/.picoclaw/workspace/
├── sessions/          # Conversation sessions and history
├── memory/           # Long-term memory (MEMORY.md)
├── state/            # Persistent state (last channel, etc.)
├── cron/             # Scheduled jobs database
├── skills/           # Custom skills
├── AGENTS.md         # Agent behavior guide
├── HEARTBEAT.md      # Periodic task prompts (checked every 30 min)
├── IDENTITY.md       # Agent identity
├── SOUL.md           # Agent soul
├── TOOLS.md          # Tool descriptions
└── USER.md           # User preferences
```

### 🔒 Security Sandbox

PicoClaw runs in a sandboxed environment by default. The agent can only access files and execute commands within the configured workspace.

#### Default Configuration

```json
"agents": {
  "defaults": {
    "workspace": "~/.picoclaw/workspace",
    "restrict_to_workspace": true
  }
}
```

| Option                  | Default                 | Description                               |
| ----------------------- | ----------------------- | ----------------------------------------- |
| `workspace`             | `~/.picoclaw/workspace` | Working directory for the agent           |
| `restrict_to_workspace` | `true`                  | Restrict file/command access to workspace |

#### Protected Tools

When `restrict_to_workspace: true`, the following tools are sandboxed:

| Tool          | Function         | Restriction                            |
| ------------- | ---------------- | -------------------------------------- |
| `read_file`   | Read files       | Only files within workspace            |
| `write_file`  | Write files      | Only files within workspace            |
| `list_dir`    | List directories | Only directories within workspace      |
| `edit_file`   | Edit files       | Only files within workspace            |
| `append_file` | Append to files  | Only files within workspace            |
| `exec`        | Execute commands | Command paths must be within workspace |

#### Additional Exec Protection

Even with `restrict_to_workspace: false`, the `exec` tool blocks these dangerous commands:

* `rm -rf`, `del /f`, `rmdir /s` — Bulk deletion
* `format`, `mkfs`, `diskpart` — Disk formatting
* `dd if=` — Disk imaging
* Writing to `/dev/sd[a-z]` — Direct disk writes
* `shutdown`, `reboot`, `poweroff` — System shutdown
* Fork bomb `:(){ :|:& };:`

#### Error Examples

```
[ERROR] tool: Tool execution failed
{tool=exec, error=Command blocked by safety guard (path outside working dir)}
```

```
[ERROR] tool: Tool execution failed
{tool=exec, error=Command blocked by safety guard (dangerous pattern detected)}
```

#### Disabling Restrictions (Security Risk)

If you need the agent to access paths outside the workspace:

**Config file**

```json
"agents": {
  "defaults": {
    "restrict_to_workspace": false
  }
}
```

> ⚠️ **Warning**: Disabling this restriction allows the agent to access any path on your system. Use with caution in controlled environments only.

#### Security Boundary Consistency

The `restrict_to_workspace` setting applies consistently across all execution paths:

| Execution Path   | Security Boundary            |
| ---------------- | ---------------------------- |
| Main Agent       | `restrict_to_workspace` ✅   |
| Subagent / Spawn | Inherits same restriction ✅ |
| Heartbeat tasks  | Inherits same restriction ✅ |

All paths share the same workspace restriction — there's no way to bypass the security boundary through subagents or scheduled tasks.

### Heartbeat (Periodic Tasks)

PicoClaw can perform periodic tasks automatically. Create a `HEARTBEAT.md` file in your workspace:

```markdown
# Periodic Tasks

- Check my email for important messages
- Review my calendar for upcoming events
- Check the weather forecast
```

The agent will read this file every 30 minutes (configurable) and execute any tasks using available tools.

#### Async Tasks with Spawn

For long-running tasks (web search, API calls), use the `spawn` tool to create a **subagent**:

```markdown
# Periodic Tasks

## Quick Tasks (respond directly)

- Report current time

## Long Tasks (use spawn for async)

- Search the web for AI news and summarize
- Check email and report important messages
```

**Key behaviors:**

| Feature                 | Description                                               |
| ----------------------- | --------------------------------------------------------- |
| **spawn**               | Creates async subagent, doesn't block heartbeat           |
| **Independent context** | Subagent has its own context, no session history          |
| **message tool**        | Subagent communicates with user directly via message tool |
| **Non-blocking**        | After spawning, heartbeat continues to next task          |

#### How Subagent Communication Works

```
Heartbeat triggers
    ↓
Agent reads HEARTBEAT.md
    ↓
For long task: spawn subagent
    ↓                           ↓
Continue to next task      Subagent works independently
    ↓                           ↓
All tasks done            Subagent uses "message" tool
    ↓                           ↓
Respond HEARTBEAT_OK      User receives result directly
```

The subagent has access to tools (message, web_search, etc.) and can communicate with the user independently without going through the main agent.

**Configuration:**

```json
"heartbeat": {
  "enabled": true,
  "interval": 30
}
```

| Option     | Default | Description                        |
| ---------- | ------- | ---------------------------------- |
| `enabled`  | `true`  | Enable/disable heartbeat           |
| `interval` | `30`    | Check interval in minutes (min: 5) |

### 🗄️ Qdrant Vector Storage

PicoClaw supports **persistent message storage** in [Qdrant](https://qdrant.io/) vector database with semantic search capabilities. All chat messages are automatically embedded using Mistral AI and stored for long-term memory and context retrieval.

#### Features

| Feature | Description |
|---------|-------------|
| **Vector Storage** | All messages stored in Qdrant with 1024-dimension embeddings |
| **Semantic Search** | Find relevant messages using natural language queries via `qdrant_search_memory` tool |
| **Mistral Embeddings** | Uses `mistral-embed` model for high-quality vector generation |
| **Session Filtering** | Search within specific sessions or across all conversations |
| **Time-based Filters** | Filter messages by timestamp range |
| **Role Filtering** | Search only user messages or assistant responses |

#### Quick Start

**1. Start Qdrant** (local or use Qdrant Cloud):

```bash
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

**2. Get Mistral API Key** from [console.mistral.ai](https://console.mistral.ai/api-keys)

**3. Configure PicoClaw** (`~/.picoclaw/config.json`):

```json
"storage": {
  "qdrant": {
    "enabled": true,
    "host": "localhost",
    "port": 6333,
    "collection": "picoclaw_messages",
    "vector_size": 1024,
    "secure": false,
    "api_key": ""
  },
  "embedding": {
    "enabled": true,
    "model": "mistral-embed",
    "api_base": "https://api.mistral.ai/v1",
    "api_key": "your-mistral-api-key"
  }
}
```

#### Configuration Options

##### Qdrant Settings

| Option | Default | Description |
|--------|---------|-------------|
| `enabled` | `false` | Enable Qdrant storage |
| `host` | `localhost` | Qdrant server hostname |
| `port` | `6333` | HTTP port |
| `grpc_port` | `6334` | gRPC port (optional) |
| `collection` | `picoclaw_messages` | Collection name |
| `vector_size` | `1024` | Embedding dimension (mistral-embed = 1024) |
| `secure` | `false` | Use HTTPS |
| `api_key` | `""` | API key for Qdrant Cloud |

##### Embedding Settings

| Option | Default | Description |
|--------|---------|-------------|
| `enabled` | `false` | Enable embedding generation |
| `model` | `mistral-embed` | Embedding model |
| `api_base` | `https://api.mistral.ai/v1` | Mistral API endpoint |
| `api_key` | `""` | Mistral API key |

#### Using the Search Tool

The `qdrant_search_memory` tool is automatically available when Qdrant is enabled.

**Example usage by agent:**

```json
"tool": "qdrant_search_memory",
"arguments": {
  "query_text": "How did the user configure Docker?",
  "limit": 5,
  "filters": {
    "role": "user",
    "timestamp_from": "2024-01-01T00:00:00Z"
  }
}
```

**Parameters:**

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `query_text` | ✅ Yes | - | Natural language search query |
| `limit` | ❌ No | 5 | Max results (max: 20) |
| `filters.role` | ❌ No | - | Filter by role: `user`, `assistant`, `system` |
| `filters.session_key` | ❌ No | - | Filter by session (e.g., `telegram:123456`) |
| `filters.timestamp_from` | ❌ No | - | Messages from this date (ISO 8601) |
| `filters.timestamp_to` | ❌ No | - | Messages until this date (ISO 8601) |

#### Qdrant Cloud Configuration

```json
"storage": {
  "qdrant": {
    "enabled": true,
    "host": "your-cluster-id.cloud.qdrant.io",
    "port": 443,
    "secure": true,
    "api_key": "your-qdrant-cloud-api-key",
    "collection": "picoclaw_messages",
    "vector_size": 1024
  },
  "embedding": {
    "enabled": true,
    "model": "mistral-embed",
    "api_base": "https://api.mistral.ai/v1",
    "api_key": "your-mistral-api-key"
  }
}
```

### Providers

> [!NOTE]
> Groq provides free voice transcription via Whisper. If configured, Telegram voice messages will be automatically transcribed.

| Provider                   | Purpose                                 | Get API Key                                                          |
| -------------------------- | --------------------------------------- | -------------------------------------------------------------------- |
| `gemini`                   | LLM (Gemini direct)                     | [aistudio.google.com](https://aistudio.google.com)                   |
| `zhipu`                    | LLM (Zhipu direct)                      | [bigmodel.cn](https://bigmodel.cn)                                   |
| `openrouter(To be tested)` | LLM (recommended, access to all models) | [openrouter.ai](https://openrouter.ai)                               |
| `anthropic(To be tested)`  | LLM (Claude direct)                     | [console.anthropic.com](https://console.anthropic.com)               |
| `openai(To be tested)`     | LLM (GPT direct)                        | [platform.openai.com](https://platform.openai.com)                   |
| `deepseek(To be tested)`   | LLM (DeepSeek direct)                   | [platform.deepseek.com](https://platform.deepseek.com)               |
| `qwen`                     | LLM (Qwen direct)                       | [dashscope.console.aliyun.com](https://dashscope.console.aliyun.com) |
| `groq`                     | LLM + **Voice transcription** (Whisper) | [console.groq.com](https://console.groq.com)                         |
| `cerebras`                 | LLM (Cerebras direct)                   | [cerebras.ai](https://cerebras.ai)                                   |
| `mistral`                 | LLM (Mistral direct)                   | [console.mistral.ai](https://console.mistral.ai)                                   |

### Model Configuration (model_list)

This design also enables **multi-agent support** with flexible provider selection:

- **Different agents, different providers**: Each agent can use its own LLM provider
- **Model fallbacks**: Configure primary and fallback models for resilience
- **Load balancing**: Distribute requests across multiple endpoints
- **Centralized configuration**: Manage all providers in one place

#### 📋 All Supported Vendors

| Vendor              | `model` Prefix    | Default API Base                                    | Protocol  | API Key                                                          |
| ------------------- | ----------------- | --------------------------------------------------- | --------- | ---------------------------------------------------------------- |
| **OpenAI**          | `openai/`         | `https://api.openai.com/v1`                         | OpenAI    | [Get Key](https://platform.openai.com)                           |
| **Anthropic**       | `anthropic/`      | `https://api.anthropic.com/v1`                      | Anthropic | [Get Key](https://console.anthropic.com)                         |
| **AI (GLM)**   | `zhipu/`          | `https://open.bigmodel.cn/api/paas/v4`              | OpenAI    | [Get Key](https://open.bigmodel.cn/usercenter/proj-mgmt/apikeys) |
| **DeepSeek**        | `deepseek/`       | `https://api.deepseek.com/v1`                       | OpenAI    | [Get Key](https://platform.deepseek.com)                         |
| **Google Gemini**   | `gemini/`         | `https://generativelanguage.googleapis.com/v1beta`  | OpenAI    | [Get Key](https://aistudio.google.com/api-keys)                  |
| **Groq**            | `groq/`           | `https://api.groq.com/openai/v1`                    | OpenAI    | [Get Key](https://console.groq.com)                              |
| **Moonshot**        | `moonshot/`       | `https://api.moonshot.cn/v1`                        | OpenAI    | [Get Key](https://platform.moonshot.cn)                          |
| **Qwen** | `qwen/`           | `https://dashscope.aliyuncs.com/compatible-mode/v1` | OpenAI    | [Get Key](https://dashscope.console.aliyun.com)                  |
| **NVIDIA**          | `nvidia/`         | `https://integrate.api.nvidia.com/v1`               | OpenAI    | [Get Key](https://build.nvidia.com)                              |
| **Ollama**          | `ollama/`         | `http://localhost:11434/v1`                         | OpenAI    | Local (no key needed)                                            |
| **OpenRouter**      | `openrouter/`     | `https://openrouter.ai/api/v1`                      | OpenAI    | [Get Key](https://openrouter.ai/keys)                            |
| **VLLM**            | `vllm/`           | `http://localhost:8000/v1`                          | OpenAI    | Local                                                            |
| **Cerebras**        | `cerebras/`       | `https://api.cerebras.ai/v1`                        | OpenAI    | [Get Key](https://cerebras.ai)                                   |
| **火山引擎**        | `volcengine/`     | `https://ark.cn-beijing.volces.com/api/v3`          | OpenAI    | [Get Key](https://console.volcengine.com)                        |
| **神算云**          | `shengsuanyun/`   | `https://router.shengsuanyun.com/api/v1`            | OpenAI    | -                                                                |
| **Antigravity**     | `antigravity/`    | Google Cloud                                        | Custom    | OAuth only                                                       |
| **GitHub Copilot**  | `github-copilot/` | `localhost:4321`                                    | gRPC      | -                                                                |
| **Mistral**        | `mistral/`       | `https://api.mistral.ai/v1`                        | OpenAI    | [Get Key](https://console.mistral.ai)                                   |

#### Basic Configuration

```json
"model_list": [
  {
    "model_name": "gpt-5.2",
    "model": "openai/gpt-5.2",
    "api_key": "sk-your-openai-key"
  },
  {
    "model_name": "claude-sonnet-4.6",
    "model": "anthropic/claude-sonnet-4.6",
    "api_key": "sk-ant-your-key"
  },
  {
    "model_name": "glm-4.7",
    "model": "zhipu/glm-4.7",
    "api_key": "your-zhipu-key"
  }
],
"agents": {
  "defaults": {
    "model": "gpt-5.2"
  }
}
```

#### Load Balancing

Configure multiple endpoints for the same model name—PicoClaw will automatically round-robin between them:

```json
{
  "model_list": [
    {
      "model_name": "gpt-5.2",
      "model": "openai/gpt-5.2",
      "api_base": "https://api1.example.com/v1",
      "api_key": "sk-key1"
    },
    {
      "model_name": "gpt-5.2",
      "model": "openai/gpt-5.2",
      "api_base": "https://api2.example.com/v1",
      "api_key": "sk-key2"
    }
  ]
}
```

## CLI Reference

| Command                   | Description                   |
| ------------------------- | ----------------------------- |
| `picoclaw onboard`        | Initialize config & workspace |
| `picoclaw agent -m "..."` | Chat with the agent           |
| `picoclaw agent`          | Interactive chat mode         |
| `picoclaw gateway`        | Start the gateway             |
| `picoclaw status`         | Show status                   |
| `picoclaw cron list`      | List all scheduled jobs       |
| `picoclaw cron add ...`   | Add a scheduled job           |

### Session Management

PicoClaw provides built-in session management through the `session` tool, accessible via commands:

| Command | Description |
|---------|-------------|
| `/clear` | Clear the current session history and start a fresh conversation |
| `/stats` | Display session statistics including message count, tokens, and context usage |

**Session Stats Example:**

```
📊 Session Stats

Messages: 42
Tokens: ~8,400 (est.)
Context: 4.2% / 200,000 tokens
```

**Key Metrics:**

- **Messages**: Total number of messages in the current session
- **Tokens**: Estimated token count using a 2.5 characters/token heuristic
- **Context**: Percentage of the context window currently used (helps monitor when history compression will trigger)

**Session Isolation:**

Each conversation context has its own isolated session:
- **Direct messages (DM)**: Isolated per user
- **Group chats**: Shared session for the entire group
- **Forum topics (Telegram)**: Each topic has its own session

Running `/clear` in one session will not affect others.


### Scheduled Tasks / Reminders

PicoClaw supports scheduled reminders and recurring tasks through the `cron` tool:

* **One-time reminders**: "Remind me in 10 minutes" → triggers once after 10min
* **Recurring tasks**: "Remind me every 2 hours" → triggers every 2 hours
* **Cron expressions**: "Remind me at 9am daily" → uses cron expression

Jobs are stored in `~/.picoclaw/workspace/cron/` and processed automatically.

## 🤝 Contribute

PRs welcome! The codebase is intentionally small and readable.
