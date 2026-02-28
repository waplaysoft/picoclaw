# PicoClaw - Project Context

## Project Overview

**PicoClaw** is an ultra-lightweight personal AI assistant framework written in Go. It enables users to deploy AI agents that can interact through multiple chat platforms (Telegram, Discord, QQ, LINE, WeCom, etc.) and execute tasks using various tools.

### Key Features

- **Multi-Channel Support**: Telegram, Discord, QQ, DingTalk, LINE, WeCom, Slack, WhatsApp, Feishu, OneBot
- **Multi-Provider LLM Support**: OpenAI, Anthropic, Zhipu, DeepSeek, Gemini, Groq, Qwen, Ollama, and more
- **Tool System**: File operations, shell execution, web search, session management, skills, subagents
- **Vector Storage**: Qdrant integration for persistent message memory with semantic search
- **Context Management**: Smart token-based context window management and compaction
- **Security Sandbox**: Workspace restrictions to limit file/command access
- **Heartbeat Tasks**: Periodic task execution with async subagent support
- **Session Management**: Persistent conversation sessions and history

### Architecture

```
cmd/picoclaw/          # CLI entry points
├── main.go            # Cobra CLI setup
├── internal/
│   ├── agent/         # Agent command
│   ├── gateway/       # Long-running gateway bot
│   ├── onboard/       # Initial setup wizard
│   ├── auth/          # Authentication commands
│   ├── cron/          # Scheduled tasks
│   ├── skills/        # Skills management
│   └── version/       # Version info

pkg/                   # Core packages
├── agent/             # Agent logic, context, loop
├── channels/          # Chat platform integrations
├── config/            # Configuration management
├── providers/         # LLM provider implementations
├── tools/             # Tool implementations
├── skills/            # Skill system
├── session/           # Session management
├── storage/           # Qdrant vector storage
├── heartbeat/         # Periodic task execution
├── cron/              # Cron job scheduling
└── utils/             # Utilities
```

## Building and Running

### Prerequisites

- Go 1.25 or later
- `make`

### Build Commands

```bash
# Download dependencies
make deps

# Build for current platform (runs go generate first)
make build

# Build for all platforms
make build-all

# Install to ~/.local/bin
make install

# Run with arguments
make run ARGS="agent -m 'Hello'"

# Full pre-commit check
make check

# Run tests
make test

# Run single test
go test -run TestName -v ./pkg/session/

# Run benchmarks
go test -bench=. -benchmem -run='^$' ./...
```

### Docker Compose

```bash
# Start gateway (long-running bot)
docker compose --profile gateway up -d

# Run agent (one-shot query)
docker compose run --rm picoclaw-agent -m "What is 2+2?"

# View logs
docker compose logs -f picoclaw-gateway

# Stop
docker compose --profile gateway down
```

### Quick Start

1. **Initialize configuration**:
   ```bash
   picoclaw onboard
   ```

2. **Configure** (`~/.picoclaw/config.json`):
   - Set LLM API keys
   - Configure chat channel tokens
   - Optional: Enable Qdrant vector storage

3. **Run**:
   ```bash
   # One-shot agent
   picoclaw agent -m "Your question"
   
   # Long-running gateway
   picoclaw gateway
   ```

## Configuration

Config file: `~/.picoclaw/config.json`

### Key Configuration Sections

- **`agents.defaults`**: Default agent settings (model, workspace, restrictions)
- **`model_list`**: LLM provider configurations with load balancing support
- **`channels`**: Chat platform configurations
- **`storage.qdrant`**: Vector database settings
- **`embedding`**: Embedding model for vector storage
- **`heartbeat`**: Periodic task configuration

### Workspace Layout

Default: `~/.picoclaw/workspace/`

```
workspace/
├── sessions/          # Conversation sessions
├── memory/           # Long-term memory (MEMORY.md)
├── state/            # Persistent state
├── cron/             # Scheduled jobs
├── skills/           # Custom skills
├── AGENTS.md         # Agent behavior guide
├── HEARTBEAT.md      # Periodic task prompts
├── IDENTITY.md       # Agent identity
├── SOUL.md           # Agent soul
├── TOOLS.md          # Tool descriptions
└── USER.md           # User preferences
```

## Development Conventions

### Code Style

- **Formatting**: Run `make fmt` before committing
- **Static Analysis**: Run `make vet` for Go vet checks
- **Linting**: Run `make lint` (golangci-lint)
- **Line Length**: 120 characters (configured in `.golangci.yaml`)
- **Imports**: Organized by gci (standard → default → localmodule)

### Testing Practices

- Tests use `github.com/stretchr/testify`
- Test files: `*_test.go`
- Run all tests: `make test`
- Benchmarks encouraged for performance-critical code

### Linter Configuration

See `.golangci.yaml` for detailed settings. Key points:
- Many linters disabled for pragmatic development
- `gofmt`, `goimports`, `golines` for formatting
- `gci` for import ordering
- Exclusions for generated code and test files

### Git Workflow

- Branch off `main` for features/fixes
- Descriptive branch names: `fix/telegram-timeout`, `feat/ollama-provider`
- Conventional commits preferred
- PRs require CI pass + maintainer approval
- Squash merge for clean history

### AI-Assisted Development

This project embraces AI-assisted development:
- AI-generated code is acceptable but must be reviewed
- PRs must disclose AI involvement level
- Contributors responsible for testing and security review
- See `CONTRIBUTING.md` for detailed guidelines

## Key Design Patterns

### Provider Abstraction

LLM providers use protocol-based classification (OpenAI-compatible, Anthropic-compatible, etc.) rather than vendor-specific implementations.

### Tool System

Tools are registered in a central registry (`pkg/tools/registry.go`) with:
- Type-safe argument validation via JSON Schema
- Sandbox enforcement for file/exec operations
- Result wrapping with error handling

### Channel Architecture

Each chat platform implements a common channel interface:
- Message receive/send
- User identification
- Attachment handling
- Webhook/long-polling support

### Security Model

- `restrict_to_workspace: true` limits file/command access
- Dangerous commands blocked (rm -rf, format, disk operations)
- Subagents inherit same security restrictions
- Path validation before all file operations

## Common Tasks

### Adding a New Tool

1. Create tool implementation in `pkg/tools/`
2. Register in `registry.go`
3. Add JSON Schema for arguments
4. Write tests

### Adding a New Channel

1. Create channel implementation in `pkg/channels/`
2. Implement channel interface
3. Add configuration struct
4. Register in gateway

### Adding a New Provider

1. Add provider config to `model_list`
2. Implement provider adapter in `pkg/providers/`
3. Handle protocol differences

## Environment Variables

See `.env.example` for available environment variables:
- `*_API_KEY`: Provider API keys
- `PICOCLAW_STORAGE_QDRANT_*`: Qdrant configuration
- `TZ`: Timezone setting

## Related Documentation

- `README.md`: User-facing documentation
- `CONTRIBUTING.md`: Contribution guidelines
- `ROADMAP.md`: Future development plans
- `docs/`: Additional guides (WeCom setup, etc.)
