// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/storage"
)

// QdrantSearchTool provides semantic search through stored messages in Qdrant
type QdrantSearchTool struct {
	messageStore *storage.MessageStore
	sessionKey   string
	callback     AsyncCallback
}

// NewQdrantSearchTool creates a new Qdrant search tool
func NewQdrantSearchTool(messageStore *storage.MessageStore) *QdrantSearchTool {
	return &QdrantSearchTool{
		messageStore: messageStore,
	}
}

// Name returns the tool name
func (t *QdrantSearchTool) Name() string {
	return "qdrant_search_memory"
}

// Description returns the tool description
func (t *QdrantSearchTool) Description() string {
	return `Search for relevant messages in long-term memory using semantic search. 
Use this tool when you need to find past conversations or information stored in memory.
Supports filtering by role (user/assistant), session key, and time range.`
}

// Parameters returns the JSON schema for tool parameters
func (t *QdrantSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query_text": map[string]any{
				"type":        "string",
				"description": "The search query - describe what you're looking for in natural language",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default: 5, max: 20)",
				"default":     5,
			},
			"filters": map[string]any{
				"type": "object",
				"description": "Optional filters to narrow search results",
				"properties": map[string]any{
					"role": map[string]any{
						"type":        "string",
						"description": "Filter by message role: 'user', 'assistant', or 'system'",
						"enum":        []string{"user", "assistant", "system"},
					},
					"session_key": map[string]any{
						"type":        "string",
						"description": "Filter by specific session key (e.g., 'telegram:123456')",
					},
					"timestamp_from": map[string]any{
						"type":        "string",
						"description": "Filter messages from this timestamp (ISO 8601 format: 2024-01-01T00:00:00Z)",
					},
					"timestamp_to": map[string]any{
						"type":        "string",
						"description": "Filter messages until this timestamp (ISO 8601 format)",
					},
				},
			},
		},
		"required": []string{"query_text"},
	}
}

// SetSessionKey sets the current session key for context-aware search
func (t *QdrantSearchTool) SetSessionKey(sessionKey string) {
	t.sessionKey = sessionKey
}

// SetCallback sets the callback for async operations (not used for this sync tool)
func (t *QdrantSearchTool) SetCallback(cb AsyncCallback) {
	t.callback = cb
}

// Execute performs the search query
func (t *QdrantSearchTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	if t.messageStore == nil || !t.messageStore.IsEnabled() {
		return &ToolResult{
			ForLLM:  "Qdrant memory search is not configured. Enable it in config to search long-term memory.",
			IsError: true,
		}
	}

	// Extract query_text (required)
	queryText, ok := args["query_text"].(string)
	if !ok || queryText == "" {
		return &ToolResult{
			ForLLM:  "Error: query_text is required and must be a non-empty string",
			IsError: true,
		}
	}

	// Extract limit (optional, default 5)
	limit := 5
	if limitArg, ok := args["limit"]; ok {
		switch v := limitArg.(type) {
		case int:
			limit = v
		case float64:
			limit = int(v)
		case string:
			if parsed, err := strconv.Atoi(v); err == nil {
				limit = parsed
			}
		}
	}
	// Cap limit at 20
	if limit > 20 {
		limit = 20
	}
	if limit < 1 {
		limit = 1
	}

	// Extract filters (optional)
	var filters map[string]any
	if filtersArg, ok := args["filters"]; ok {
		filters, _ = filtersArg.(map[string]any)
	}

	// Determine session key to use
	searchSessionKey := t.sessionKey
	if filters != nil {
		if sessionKeyFilter, ok := filters["session_key"].(string); ok && sessionKeyFilter != "" {
			// Use filter's session key if provided
			searchSessionKey = sessionKeyFilter
		}
	}

	// Perform search
	messages, err := t.messageStore.SearchSimilarMessagesWithPayload(searchSessionKey, queryText, limit)
	if err != nil {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("Error searching memory: %v", err),
			IsError: true,
		}
	}

	// Apply client-side filters (role, timestamp)
	filteredMessages := t.applyFilters(messages, filters)

	// Format results
	if len(filteredMessages) == 0 {
		return &ToolResult{
			ForLLM: "No relevant messages found in memory.",
		}
	}

	result := t.formatResults(filteredMessages)
	return &ToolResult{
		ForLLM: result,
	}
}

// applyFilters applies role and timestamp filters to search results
func (t *QdrantSearchTool) applyFilters(messages []storage.MessagePayload, filters map[string]any) []storage.MessagePayload {
	if filters == nil || len(filters) == 0 {
		return messages
	}

	var filtered []storage.MessagePayload

	for _, msg := range messages {
		if t.matchesFilters(msg, filters) {
			filtered = append(filtered, msg)
		}
	}

	return filtered
}

// matchesFilters checks if a message matches all provided filters
func (t *QdrantSearchTool) matchesFilters(msg storage.MessagePayload, filters map[string]any) bool {
	// Role filter
	if roleFilter, ok := filters["role"].(string); ok {
		if !strings.EqualFold(msg.Role, roleFilter) {
			return false
		}
	}

	// Timestamp from filter
	if tsFrom, ok := filters["timestamp_from"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, tsFrom); err == nil {
			if msg.Timestamp.Before(parsed) {
				return false
			}
		}
	}

	// Timestamp to filter
	if tsTo, ok := filters["timestamp_to"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, tsTo); err == nil {
			if msg.Timestamp.After(parsed) {
				return false
			}
		}
	}

	return true
}

// formatResults formats search results as a readable string
func (t *QdrantSearchTool) formatResults(messages []storage.MessagePayload) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Found %d relevant message(s):\n\n", len(messages)))

	for i, msg := range messages {
		sb.WriteString(fmt.Sprintf("### Message %d\n", i+1))
		sb.WriteString(fmt.Sprintf("**Role:** %s\n", msg.Role))
		sb.WriteString(fmt.Sprintf("**Time:** %s\n", msg.Timestamp.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("**Content:** %s\n", msg.Content))
		if msg.SessionKey != "" {
			sb.WriteString(fmt.Sprintf("**Session:** %s\n", msg.SessionKey))
		}
		sb.WriteString("\n---\n\n")
	}

	return strings.TrimSuffix(sb.String(), "\n\n---\n\n")
}
