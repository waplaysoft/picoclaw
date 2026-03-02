// PicoClaw - Ultra-lightweight personal AI agent
// WebUI API Handlers
//
// Copyright (c) 2026 PicoClaw contributors

package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/session"
)

type Handlers struct {
	agentLoop      *agent.AgentLoop
	sessionManager *session.SessionManager
	sessionMutex   sync.Map // map[string]*sync.Mutex
}

func NewHandlers(agentLoop *agent.AgentLoop, sessionManager *session.SessionManager) *Handlers {
	return &Handlers{
		agentLoop:      agentLoop,
		sessionManager: sessionManager,
	}
}

type ChatRequest struct {
	Message string `json:"message"`
	Session string `json:"session,omitempty"`
	Stream  bool   `json:"stream,omitempty"`
}

type ChatResponse struct {
	Content string `json:"content"`
	Session string `json:"session"`
	Done    bool   `json:"done"`
}

type SessionInfo struct {
	Key          string    `json:"key"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
	Preview      string    `json:"preview"`
}

type SessionsResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

type HistoryResponse struct {
	Messages   []providers.Message `json:"messages"`
	TotalCount int                 `json:"total_count"`
	HasMore    bool                `json:"has_more"`
}

func (h *Handlers) getSessionMutex(session string) *sync.Mutex {
	actual, _ := h.sessionMutex.LoadOrStore(session, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

// ChatHandler handles chat messages with optional SSE streaming
func (h *Handlers) ChatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Generate session key if not provided
	session := req.Session
	if session == "" {
		session = fmt.Sprintf("webui:%d", time.Now().UnixNano())
	}

	// Get session-specific mutex to prevent concurrent modifications
	mu := h.getSessionMutex(session)
	mu.Lock()
	defer mu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	if req.Stream {
		h.handleStreamChat(w, ctx, req.Message, session)
	} else {
		h.handleSimpleChat(w, ctx, req.Message, session)
	}
}

func (h *Handlers) handleSimpleChat(w http.ResponseWriter, ctx context.Context, message, session string) {
	response, err := h.agentLoop.ProcessDirectWithChannel(ctx, message, session, "webui", session, "user", false)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	resp := ChatResponse{
		Content: response,
		Session: session,
		Done:    true,
	}

	json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) handleStreamChat(w http.ResponseWriter, ctx context.Context, message, session string) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send session info first
	sessionData := map[string]string{"session": session}
	sessionJSON, _ := json.Marshal(sessionData)
	fmt.Fprintf(w, "data: %s\n\n", sessionJSON)
	flusher.Flush()

	// Send thinking_start event to show typing indicator
	thinkingStartData := map[string]string{"type": "thinking_start"}
	thinkingStartJSON, _ := json.Marshal(thinkingStartData)
	fmt.Fprintf(w, "data: %s\n\n", thinkingStartJSON)
	flusher.Flush()

	// Process the message
	response, err := h.agentLoop.ProcessDirectWithChannel(ctx, message, session, "webui", session, "user", false)
	if err != nil {
		// Send thinking_end before error
		thinkingEndData := map[string]string{"type": "thinking_end"}
		thinkingEndJSON, _ := json.Marshal(thinkingEndData)
		fmt.Fprintf(w, "data: %s\n\n", thinkingEndJSON)

		errorData := map[string]string{"error": err.Error()}
		errorJSON, _ := json.Marshal(errorData)
		fmt.Fprintf(w, "data: %s\n\n", errorJSON)
		flusher.Flush()
		return
	}

	// Send thinking_end event to hide typing indicator
	thinkingEndData := map[string]string{"type": "thinking_end"}
	thinkingEndJSON, _ := json.Marshal(thinkingEndData)
	fmt.Fprintf(w, "data: %s\n\n", thinkingEndJSON)
	flusher.Flush()

	// Send response in chunks for simulated streaming
	runes := []rune(response)
	chunkSize := 50
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])

		chunkData := map[string]interface{}{
			"content": chunk,
			"done":    end >= len(runes),
		}
		chunkJSON, _ := json.Marshal(chunkData)
		fmt.Fprintf(w, "data: %s\n\n", chunkJSON)
		flusher.Flush()

		// Small delay to simulate streaming
		time.Sleep(10 * time.Millisecond)
	}
}

// SessionsHandler lists all active sessions
func (h *Handlers) SessionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodDelete {
		// Clear all sessions - would need agentLoop session access
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Get all sessions from session manager
	sessions := h.getAllSessions()

	resp := SessionsResponse{
		Sessions: sessions,
	}

	json.NewEncoder(w).Encode(resp)
}

// getAllSessions retrieves all sessions with metadata
func (h *Handlers) getAllSessions() []SessionInfo {
	if h.sessionManager == nil {
		return []SessionInfo{}
	}

	summaries := h.sessionManager.GetAllSessions()
	infos := make([]SessionInfo, len(summaries))
	for i, summary := range summaries {
		infos[i] = SessionInfo{
			Key:          summary.Key,
			UpdatedAt:    summary.Updated,
			MessageCount: summary.MessageCount,
			Preview:      summary.Preview,
		}
	}
	return infos
}

// HistoryHandler returns message history for a session with pagination
func (h *Handlers) HistoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	session := r.URL.Query().Get("session")
	if session == "" {
		http.Error(w, "Session parameter required", http.StatusBadRequest)
		return
	}

	// Parse pagination parameters
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	if h.sessionManager == nil {
		resp := HistoryResponse{
			Messages:   []providers.Message{},
			TotalCount: 0,
			HasMore:    false,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Get full history and filter for UI display
	fullHistory := h.sessionManager.GetHistory(session)
	filteredHistory := filterMessagesForUI(fullHistory)
	totalCount := len(filteredHistory)

	// Calculate slice bounds for pagination
	// Offset starts from 0 (oldest messages)
	start := offset
	end := offset + limit

	if start >= totalCount {
		resp := HistoryResponse{
			Messages:   []providers.Message{},
			TotalCount: totalCount,
			HasMore:    false,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	if end > totalCount {
		end = totalCount
	}

	messages := filteredHistory[start:end]
	hasMore := end < totalCount

	resp := HistoryResponse{
		Messages:   messages,
		TotalCount: totalCount,
		HasMore:    hasMore,
	}

	json.NewEncoder(w).Encode(resp)
}

// ReadyHandler returns health status
func (h *Handlers) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	status := map[string]interface{}{
		"status": "ready",
		"time":   time.Now().Format(time.RFC3339),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// extractPeer extracts peer ID from message content for routing
func extractPeer(content string) string {
	// Simple extraction - in real usage would come from channel metadata
	return strings.Split(content, "\n")[0]
}

// filterMessagesForUI filters messages for display in the WebUI.
// Only user messages and final assistant responses (without tool_calls) are shown.
// Tool and system messages are hidden.
func filterMessagesForUI(messages []providers.Message) []providers.Message {
	filtered := make([]providers.Message, 0, len(messages))
	
	for _, msg := range messages {
		// Skip tool and system messages
		if msg.Role == "tool" || msg.Role == "system" {
			continue
		}
		
		// Skip assistant messages with tool calls (intermediate reasoning steps)
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			continue
		}
		
		// Allow user messages and assistant messages without tool calls
		filtered = append(filtered, msg)
	}
	
	return filtered
}
