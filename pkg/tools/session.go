package tools

import (
	"context"
	"fmt"

	"github.com/sipeed/picoclaw/pkg/providers"
)

type SessionTool struct {
	sessionManager SessionManager
	sessionKey     string // Current session key, set by context
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

// SetContext sets the current session key.
// This should be called with the current session key before execution.
func (t *SessionTool) SetSessionKey(sessionKey string) {
	t.sessionKey = sessionKey
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

func (t *SessionTool) sessionStats() *ToolResult {
	// Get session history
	history := t.sessionManager.GetHistory(t.sessionKey)
	summary := t.sessionManager.GetSummary(t.sessionKey)

	// Calculate stats
	messageCount := len(history)
	var userMsgs, assistantMsgs int

	// Count message types
	for _, msg := range history {
		// providers.Message is a struct with Role field
		switch msg.Role {
		case "user":
			userMsgs++
		case "assistant":
			assistantMsgs++
		}
	}

	// Build stats response
	var stats string
	if messageCount == 0 {
		stats = "📊 **Session Stats**\n\n🆕 **New session** - No messages yet\n\n💬 Start a conversation to begin!"
	} else {
		stats = fmt.Sprintf("📊 **Session Stats**\n\n📨 **Messages:** %d total\n   👤 User: %d\n   🤖 Assistant: %d\n\n📝 **Summary:**",
			messageCount, userMsgs, assistantMsgs)

		if summary != "" {
			stats += fmt.Sprintf(" %s", summary)
		} else {
			stats += " No summary available"
		}
	}

	return &ToolResult{
		ForLLM: stats,
	}
}
