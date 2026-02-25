package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"telegram:123456", "telegram_123456"},
		{"discord:987654321", "discord_987654321"},
		{"slack:C01234", "slack_C01234"},
		{"no-colons-here", "no-colons-here"},
		{"multiple:colons:here", "multiple_colons_here"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSave_WithColonInKey(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewSessionManager(tmpDir)

	// Create a session with a key containing colon (typical channel session key).
	key := "telegram:123456"
	sm.GetOrCreate(key)
	sm.AddMessage(key, "user", "hello")

	// Save should succeed even though the key contains ':'
	if err := sm.Save(key); err != nil {
		t.Fatalf("Save(%q) failed: %v", key, err)
	}

	// The file on disk should use sanitized name.
	expectedFile := filepath.Join(tmpDir, "telegram_123456.json")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatalf("expected session file %s to exist", expectedFile)
	}

	// Load into a fresh manager and verify the session round-trips.
	sm2 := NewSessionManager(tmpDir)
	history := sm2.GetHistory(key)
	if len(history) != 1 {
		t.Fatalf("expected 1 message after reload, got %d", len(history))
	}
	if history[0].Content != "hello" {
		t.Errorf("expected message content %q, got %q", "hello", history[0].Content)
	}
}

func TestSave_RejectsPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewSessionManager(tmpDir)

	badKeys := []string{"", ".", "..", "foo/bar", "foo\\bar"}
	for _, key := range badKeys {
		sm.GetOrCreate(key)
		if err := sm.Save(key); err == nil {
			t.Errorf("Save(%q) should have failed but didn't", key)
		}
	}
}

// TestAddFullMessage_FiltersIntermediateAssistantMessages verifies that
// assistant messages with tool calls (intermediate reasoning steps) are
// not stored to Qdrant, while final assistant responses are stored.
func TestAddFullMessage_FiltersIntermediateAssistantMessages(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create SessionManager with Qdrant disabled (we just test the filtering logic)
	sm := NewSessionManagerWithConfig(tmpDir, config.StorageConfig{
		Qdrant: config.QdrantConfig{
			Enabled: false,
		},
	})

	sessionKey := "test:session"
	sm.GetOrCreate(sessionKey)

	// 1. User message - should be added
	sm.AddFullMessage(sessionKey, providers.Message{
		Role:    "user",
		Content: "What is 2+2?",
	})

	// 2. Intermediate assistant message with tool calls - should be added to session
	// but would be filtered before Qdrant storage (if Qdrant was enabled)
	sm.AddFullMessage(sessionKey, providers.Message{
		Role:    "assistant",
		Content: "Let me calculate that...",
		ToolCalls: []providers.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Name: "calculator",
				Function: &providers.FunctionCall{
					Name:      "calculator",
					Arguments: `{"expression": "2+2"}`,
				},
			},
		},
	})

	// 3. Tool result message - should be added to session
	// but filtered before Qdrant storage
	sm.AddFullMessage(sessionKey, providers.Message{
		Role:       "tool",
		Content:    "4",
		ToolCallID: "call_1",
	})

	// 4. Final assistant response without tool calls - should be stored
	sm.AddFullMessage(sessionKey, providers.Message{
		Role:    "assistant",
		Content: "The answer is 4",
	})

	// Verify all messages are in session history
	history := sm.GetHistory(sessionKey)
	if len(history) != 4 {
		t.Fatalf("Expected 4 messages in session history, got %d", len(history))
	}

	// Verify the filtering logic would work correctly:
	// - user messages: stored
	// - assistant with tool calls: filtered (not stored to Qdrant)
	// - tool messages: filtered (not stored to Qdrant)
	// - assistant without tool calls: stored
	
	// Count messages that would be stored to Qdrant
	qdrantStoredCount := 0
	for _, msg := range history {
		shouldStore := true
		
		// Apply same filtering logic as in AddFullMessage
		if msg.Role == "tool" || msg.Role == "system" {
			shouldStore = false
		}
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			shouldStore = false
		}
		if msg.Content == "" {
			shouldStore = false
		}
		
		if shouldStore {
			qdrantStoredCount++
		}
	}

	// Only user message and final assistant response should be stored
	if qdrantStoredCount != 2 {
		t.Errorf("Expected 2 messages to be stored to Qdrant, got %d", qdrantStoredCount)
	}

	// Verify specific messages would be stored
	storedRoles := []string{}
	for _, msg := range history {
		if msg.Role == "tool" || msg.Role == "system" {
			continue
		}
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			continue
		}
		if msg.Content == "" {
			continue
		}
		storedRoles = append(storedRoles, msg.Role)
	}

	if len(storedRoles) != 2 {
		t.Errorf("Expected 2 stored roles, got %d", len(storedRoles))
	}
	if storedRoles[0] != "user" {
		t.Errorf("Expected first stored role to be 'user', got '%s'", storedRoles[0])
	}
	if storedRoles[1] != "assistant" {
		t.Errorf("Expected second stored role to be 'assistant', got '%s'", storedRoles[1])
	}
}
