// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package tools

import (
	"context"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/storage"
)

func TestQdrantSearchTool_Parameters(t *testing.T) {
	tool := NewQdrantSearchTool(nil)
	params := tool.Parameters()

	// Check required fields
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("parameters should have required field")
	}

	found := false
	for _, r := range required {
		if r == "query_text" {
			found = true
			break
		}
	}
	if !found {
		t.Error("query_text should be required")
	}

	// Check properties
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("parameters should have properties field")
	}

	if _, ok := props["query_text"]; !ok {
		t.Error("query_text should be in properties")
	}
	if _, ok := props["limit"]; !ok {
		t.Error("limit should be in properties")
	}
	if _, ok := props["filters"]; !ok {
		t.Error("filters should be in properties")
	}
}

func TestQdrantSearchTool_Name(t *testing.T) {
	tool := NewQdrantSearchTool(nil)
	name := tool.Name()
	if name != "qdrant_search_memory" {
		t.Errorf("expected name 'qdrant_search_memory', got '%s'", name)
	}
}

func TestQdrantSearchTool_Description(t *testing.T) {
	tool := NewQdrantSearchTool(nil)
	desc := tool.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
}

func TestQdrantSearchTool_Execute_NoStore(t *testing.T) {
	tool := NewQdrantSearchTool(nil)
	result := tool.Execute(context.Background(), map[string]any{
		"query_text": "test query",
	})

	if !result.IsError {
		t.Error("should return error when store is nil")
	}
	if result.ForLLM == "" {
		t.Error("should have error message")
	}
}

func TestQdrantSearchTool_Execute_MissingQuery(t *testing.T) {
	// Create a disabled store
	store, _ := storage.NewMessageStore(config.StorageConfig{})
	tool := NewQdrantSearchTool(store)

	result := tool.Execute(context.Background(), map[string]any{})

	if !result.IsError {
		t.Error("should return error when query_text is missing")
	}
}

func TestQdrantSearchTool_Execute_EmptyQuery(t *testing.T) {
	store, _ := storage.NewMessageStore(config.StorageConfig{})
	tool := NewQdrantSearchTool(store)

	result := tool.Execute(context.Background(), map[string]any{
		"query_text": "",
	})

	if !result.IsError {
		t.Error("should return error when query_text is empty")
	}
}

func TestQdrantSearchTool_Execute_LimitValidation(t *testing.T) {
	// Create a disabled store - it will return "not configured" error
	store, _ := storage.NewMessageStore(config.StorageConfig{})
	tool := NewQdrantSearchTool(store)

	// Test limit > 20 (should be capped)
	result := tool.Execute(context.Background(), map[string]any{
		"query_text": "test",
		"limit":      100,
	})

	// Store is disabled, should return error about not configured
	if !result.IsError {
		t.Error("should return error when store is disabled")
	}

	// Test limit < 1 (should be set to 1)
	result = tool.Execute(context.Background(), map[string]any{
		"query_text": "test",
		"limit":      0,
	})

	if !result.IsError {
		t.Error("should return error when store is disabled")
	}
}

func TestQdrantSearchTool_SetSessionKey(t *testing.T) {
	// Create a disabled store
	store, _ := storage.NewMessageStore(config.StorageConfig{})
	tool := NewQdrantSearchTool(store)

	tool.SetSessionKey("test-session:123")

	// Verify session key is set (indirectly through execution)
	result := tool.Execute(context.Background(), map[string]any{
		"query_text": "test",
	})

	// Store is disabled, should return error
	if !result.IsError {
		t.Error("should return error when store is disabled")
	}
}

func TestQdrantSearchTool_MatchesFilters(t *testing.T) {
	store, _ := storage.NewMessageStore(config.StorageConfig{})
	tool := NewQdrantSearchTool(store)

	testTime := time.Now()

	tests := []struct {
		name    string
		msg     storage.MessagePayload
		filters map[string]any
		want    bool
	}{
		{
			name: "no filters",
			msg: storage.MessagePayload{
				Role:    "user",
				Content: "test",
			},
			filters: map[string]any{},
			want:    true,
		},
		{
			name: "role match",
			msg: storage.MessagePayload{
				Role:    "user",
				Content: "test",
			},
			filters: map[string]any{
				"role": "user",
			},
			want: true,
		},
		{
			name: "role mismatch",
			msg: storage.MessagePayload{
				Role:    "user",
				Content: "test",
			},
			filters: map[string]any{
				"role": "assistant",
			},
			want: false,
		},
		{
			name: "timestamp from match",
			msg: storage.MessagePayload{
				Timestamp: testTime,
			},
			filters: map[string]any{
				"timestamp_from": testTime.Add(-time.Hour).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "timestamp from mismatch",
			msg: storage.MessagePayload{
				Timestamp: testTime,
			},
			filters: map[string]any{
				"timestamp_from": testTime.Add(time.Hour).Format(time.RFC3339),
			},
			want: false,
		},
		{
			name: "timestamp to match",
			msg: storage.MessagePayload{
				Timestamp: testTime,
			},
			filters: map[string]any{
				"timestamp_to": testTime.Add(time.Hour).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "timestamp to mismatch",
			msg: storage.MessagePayload{
				Timestamp: testTime,
			},
			filters: map[string]any{
				"timestamp_to": testTime.Add(-time.Hour).Format(time.RFC3339),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tool.matchesFilters(tt.msg, tt.filters)
			if got != tt.want {
				t.Errorf("matchesFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQdrantSearchTool_ApplyFilters(t *testing.T) {
	store, _ := storage.NewMessageStore(config.StorageConfig{})
	tool := NewQdrantSearchTool(store)

	messages := []storage.MessagePayload{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "msg2"},
		{Role: "user", Content: "msg3"},
	}

	// Filter by role
	filters := map[string]any{
		"role": "user",
	}

	filtered := tool.applyFilters(messages, filters)

	if len(filtered) != 2 {
		t.Errorf("expected 2 messages, got %d", len(filtered))
	}

	for _, msg := range filtered {
		if msg.Role != "user" {
			t.Errorf("expected role 'user', got '%s'", msg.Role)
		}
	}
}

func TestQdrantSearchTool_FormatResults(t *testing.T) {
	store, _ := storage.NewMessageStore(config.StorageConfig{})
	tool := NewQdrantSearchTool(store)

	messages := []storage.MessagePayload{
		{
			SessionKey:   "test:123",
			Role:         "user",
			Content:      "Hello",
			Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			MessageIndex: 0,
		},
		{
			SessionKey:   "test:123",
			Role:         "assistant",
			Content:      "Hi there!",
			Timestamp:    time.Date(2024, 1, 1, 12, 1, 0, 0, time.UTC),
			MessageIndex: 1,
		},
	}

	result := tool.formatResults(messages)

	// Check result contains expected content
	if len(result) == 0 {
		t.Error("result should not be empty")
	}

	// Check formatting
	expectedSubstrings := []string{
		"Found 2 relevant message",
		"### Message 1",
		"### Message 2",
		"**Role:** user",
		"**Role:** assistant",
		"**Content:** Hello",
		"**Content:** Hi there!",
		"**Session:** test:123",
	}

	for _, substr := range expectedSubstrings {
		if !contains(result, substr) {
			t.Errorf("result should contain '%s', got: %s", substr, result)
		}
	}
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return findSubstring(s, substr)
}
