package tools

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/sipeed/picoclaw/pkg/providers"
)

type SessionTool struct {
	sessionManager SessionManager
	sessionKey     string // Current session key, set by context
	contextWindow  int    // Context window size for percentage calculation
}

// SessionManager defines the interface for session management.
// This allows the tool to work with the actual session manager without circular dependencies.
type SessionManager interface {
	GetHistory(key string) []providers.Message
	TruncateHistory(key string, keepLast int)
	GetSummary(key string) string
}

func NewSessionTool() *SessionTool {
	return &SessionTool{}
}

func (t *SessionTool) Name() string {
	return "session"
}

func (t *SessionTool) Description() string {
	return "Manage the current conversation session: clear history or get session stats. Use /clear to start a new session or /stats to see current session info."
}

func (t *SessionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"clear", "stats"},
				"description": "Action to perform: 'clear' to clear the current session history, 'stats' to show session information",
			},
		},
		"required": []string{"action"},
	}
}

// SetSessionManager sets the session manager for the tool.
// This should be called after the session manager is created.
func (t *SessionTool) SetSessionManager(sm SessionManager) {
	t.sessionManager = sm
}

// SetSessionKey sets the current session key.
// This should be called with the current session key before execution.
func (t *SessionTool) SetSessionKey(sessionKey string) {
	t.sessionKey = sessionKey
}

// SetContextWindow sets the context window size for percentage calculation.
// This should be called after the agent instance is created.
func (t *SessionTool) SetContextWindow(contextWindow int) {
	t.contextWindow = contextWindow
}

func (t *SessionTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, ok := args["action"].(string)
	if !ok {
		return &ToolResult{ForLLM: "action is required (clear or stats)", IsError: true}
	}

	if t.sessionManager == nil {
		return &ToolResult{ForLLM: "Session manager not available", IsError: true}
	}

	if t.sessionKey == "" {
		return &ToolResult{ForLLM: "No current session", IsError: true}
	}

	switch action {
	case "clear":
		return t.clearSession()
	case "stats":
		return t.sessionStats()
	default:
		return &ToolResult{ForLLM: fmt.Sprintf("Unknown action: %s. Use 'clear' or 'stats'", action), IsError: true}
	}
}

func (t *SessionTool) clearSession() *ToolResult {
	// Clear the session history
	t.sessionManager.TruncateHistory(t.sessionKey, 0)
	return &ToolResult{
		ForLLM: "✅ Session cleared successfully. Starting a new conversation!",
	}
}

// estimateTokens estimates the number of tokens in a message list.
// Uses a safe heuristic of 2.5 characters per token to account for CJK and other overheads.
func estimateTokens(messages []providers.Message) int {
	totalChars := 0
	for _, m := range messages {
		totalChars += utf8.RuneCountInString(m.Content)
	}
	// 2.5 chars per token = totalChars * 2 / 5
	return totalChars * 2 / 5
}

func (t *SessionTool) sessionStats() *ToolResult {
	// Get session history
	history := t.sessionManager.GetHistory(t.sessionKey)

	// Calculate stats
	messageCount := len(history)
	tokens := estimateTokens(history)

	// Calculate context percentage
	var contextPercent float64
	var contextMax string
	if t.contextWindow > 0 {
		contextPercent = float64(tokens) / float64(t.contextWindow) * 100
		contextMax = fmt.Sprintf(" / %d tokens", t.contextWindow)
	}

	// Build stats response
	var stats string
	if messageCount == 0 {
		stats = "📊 Session Stats\n\nMessages: 0\nTokens: 0 (est.)"
		if t.contextWindow > 0 {
			stats += fmt.Sprintf("\nContext: 0%% / %d tokens", t.contextWindow)
		}
	} else {
		stats = fmt.Sprintf("📊 Session Stats\n\nMessages: %d\nTokens: ~%d (est.)",
			messageCount, tokens)
		if t.contextWindow > 0 {
			stats += fmt.Sprintf("\nContext: %.1f%%%s", contextPercent, contextMax)
		}
	}

	return &ToolResult{
		ForLLM: stats,
	}
}
