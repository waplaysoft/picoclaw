package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/routing"
	"github.com/sipeed/picoclaw/pkg/session"
	"github.com/sipeed/picoclaw/pkg/storage"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// AgentInstance represents a fully configured agent with its own workspace,
// session manager, context builder, and tool registry.
type AgentInstance struct {
	ID             string
	Name           string
	Model          string
	Fallbacks      []string
	Workspace      string
	MaxIterations  int
	MaxTokens      int
	Temperature    float64
	ContextWindow  int
	Provider       providers.LLMProvider
	Sessions       *session.SessionManager
	ContextBuilder *ContextBuilder
	Tools          *tools.ToolRegistry
	Subagents      *config.SubagentsConfig
	SkillsFilter   []string
	Candidates     []providers.FallbackCandidate
}

// NewAgentInstance creates an agent instance from config.
func NewAgentInstance(
	agentCfg *config.AgentConfig,
	defaults *config.AgentDefaults,
	cfg *config.Config,
	provider providers.LLMProvider,
) *AgentInstance {
	workspace := resolveAgentWorkspace(agentCfg, defaults)
	os.MkdirAll(workspace, 0o755)

	model := resolveAgentModel(agentCfg, defaults)
	fallbacks := resolveAgentFallbacks(agentCfg, defaults)

	restrict := defaults.RestrictToWorkspace
	toolsRegistry := tools.NewToolRegistry()
	toolsRegistry.Register(tools.NewReadFileTool(workspace, restrict))
	toolsRegistry.Register(tools.NewWriteFileTool(workspace, restrict))
	toolsRegistry.Register(tools.NewListDirTool(workspace, restrict))
	toolsRegistry.Register(tools.NewExecToolWithConfig(workspace, restrict, cfg))
	toolsRegistry.Register(tools.NewEditFileTool(workspace, restrict))
	toolsRegistry.Register(tools.NewAppendFileTool(workspace, restrict))

	sessionsDir := filepath.Join(workspace, "sessions")
	sessionsManager := session.NewSessionManagerWithConfig(sessionsDir, cfg.Storage)

	// Note: sessionTool registration is deferred until after contextWindow is calculated
	// It needs the contextWindow value for percentage calculation

	contextBuilder := NewContextBuilder(workspace)
	contextBuilder.SetToolsRegistry(toolsRegistry)

	agentID := routing.DefaultAgentID
	agentName := ""
	var subagents *config.SubagentsConfig
	var skillsFilter []string

	if agentCfg != nil {
		agentID = routing.NormalizeAgentID(agentCfg.ID)
		agentName = agentCfg.Name
		subagents = agentCfg.Subagents
		skillsFilter = agentCfg.Skills
	}

	maxIter := defaults.MaxToolIterations
	if maxIter == 0 {
		maxIter = 20
	}

	maxTokens := defaults.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}

	temperature := 0.7
	if defaults.Temperature != nil {
		temperature = *defaults.Temperature
	}

	// Resolve ContextWindow: use from config if set, otherwise use maxTokens as fallback
	contextWindow := defaults.ContextWindow
	if contextWindow == 0 {
		contextWindow = maxTokens
	}

	// Register session management tool now that contextWindow is known
	sessionTool := tools.NewSessionTool()
	sessionTool.SetSessionManager(sessionsManager)
	sessionTool.SetContextWindow(contextWindow)
	toolsRegistry.Register(sessionTool)

	// Register Qdrant search tool if storage is enabled
	if cfg.Storage.Qdrant.Enabled {
		// Find Mistral API key from model_list for embeddings
		var mistralAPIKey string
		for _, modelCfg := range cfg.ModelList {
			if modelCfg.ModelName == "mistral-embed" || 
			   (modelCfg.Model != "" && strings.Contains(modelCfg.Model, "mistral-embed")) {
				mistralAPIKey = modelCfg.APIKey
				break
			}
		}

		// Set the embedding API key in storage config
		storageCfg := cfg.Storage
		if mistralAPIKey != "" {
			storageCfg.Embedding.APIKey = mistralAPIKey
			storageCfg.Embedding.APIBase = "https://api.mistral.ai/v1"
			storageCfg.Embedding.Model = "mistral-embed"
			storageCfg.Embedding.Enabled = true
		}

		// Create message store for the tool
		messageStore, err := storage.NewMessageStore(storageCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Qdrant] Failed to create message store: %v\n", err)
		} else if messageStore.IsEnabled() {
			fmt.Fprintf(os.Stderr, "[Qdrant] Enabled (collection: %s)\n", cfg.Storage.Qdrant.Collection)
			// Warn only if no API key found in either storage.embedding or model_list
			if storageCfg.Embedding.APIKey == "" {
				fmt.Fprintf(os.Stderr, "[Qdrant] WARNING: No Mistral API key found. Add to storage.embedding.api_key or model_list with mistral-embed.\n")
			}
			qdrantTool := tools.NewQdrantSearchTool(messageStore)
			qdrantTool.SetSessionKey("") // Will be set per-request
			toolsRegistry.Register(qdrantTool)
		}
	}

	// Resolve fallback candidates
	modelCfg := providers.ModelConfig{
		Primary:   model,
		Fallbacks: fallbacks,
	}
	candidates := providers.ResolveCandidates(modelCfg, defaults.Provider)

	return &AgentInstance{
		ID:             agentID,
		Name:           agentName,
		Model:          model,
		Fallbacks:      fallbacks,
		Workspace:      workspace,
		MaxIterations:  maxIter,
		MaxTokens:      maxTokens,
		Temperature:    temperature,
		ContextWindow:  contextWindow,
		Provider:       provider,
		Sessions:       sessionsManager,
		ContextBuilder: contextBuilder,
		Tools:          toolsRegistry,
		Subagents:      subagents,
		SkillsFilter:   skillsFilter,
		Candidates:     candidates,
	}
}

// resolveAgentWorkspace determines the workspace directory for an agent.
func resolveAgentWorkspace(agentCfg *config.AgentConfig, defaults *config.AgentDefaults) string {
	if agentCfg != nil && strings.TrimSpace(agentCfg.Workspace) != "" {
		return expandHome(strings.TrimSpace(agentCfg.Workspace))
	}
	if agentCfg == nil || agentCfg.Default || agentCfg.ID == "" || routing.NormalizeAgentID(agentCfg.ID) == "main" {
		return expandHome(defaults.Workspace)
	}
	home, _ := os.UserHomeDir()
	id := routing.NormalizeAgentID(agentCfg.ID)
	return filepath.Join(home, ".picoclaw", "workspace-"+id)
}

// resolveAgentModel resolves the primary model for an agent.
func resolveAgentModel(agentCfg *config.AgentConfig, defaults *config.AgentDefaults) string {
	if agentCfg != nil && agentCfg.Model != nil && strings.TrimSpace(agentCfg.Model.Primary) != "" {
		return strings.TrimSpace(agentCfg.Model.Primary)
	}
	return defaults.Model
}

// resolveAgentFallbacks resolves the fallback models for an agent.
func resolveAgentFallbacks(agentCfg *config.AgentConfig, defaults *config.AgentDefaults) []string {
	if agentCfg != nil && agentCfg.Model != nil && agentCfg.Model.Fallbacks != nil {
		return agentCfg.Model.Fallbacks
	}
	return defaults.ModelFallbacks
}

func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}
