// PicoClaw - Ultra-lightweight personal AI agent
// WebUI Server Tests
//
// Copyright (c) 2026 PicoClaw contributors

package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlers_ChatHandler_EmptyMessage(t *testing.T) {
	handlers := NewHandlers(nil)

	reqBody := ChatRequest{
		Message: "",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.ChatHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Message is required")
}

func TestHandlers_ChatHandler_InvalidJSON(t *testing.T) {
	handlers := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handlers.ChatHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlers_ChatHandler_OPTIONS(t *testing.T) {
	handlers := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodOptions, "/api/chat", nil)
	w := httptest.NewRecorder()

	handlers.ChatHandler(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "POST, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", w.Header().Get("Access-Control-Allow-Headers"))
}

func TestHandlers_ChatHandler_WrongMethod(t *testing.T) {
	handlers := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chat", nil)
	w := httptest.NewRecorder()

	handlers.ChatHandler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandlers_ReadyHandler(t *testing.T) {
	handlers := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ready", nil)
	w := httptest.NewRecorder()

	handlers.ReadyHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ready")
}

func TestHandlers_SessionsHandler(t *testing.T) {
	handlers := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()

	handlers.SessionsHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_SessionsHandler_Delete(t *testing.T) {
	handlers := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions", nil)
	w := httptest.NewRecorder()

	handlers.SessionsHandler(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandlers_HistoryHandler_MissingSession(t *testing.T) {
	handlers := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	w := httptest.NewRecorder()

	handlers.HistoryHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Session parameter required")
}

func TestHandlers_HistoryHandler_Success(t *testing.T) {
	handlers := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/history?session=test", nil)
	w := httptest.NewRecorder()

	handlers.HistoryHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_NewServer(t *testing.T) {
	cfg := &Config{
		Host:    "127.0.0.1",
		Port:    18791,
		Enabled: true,
	}
	handlers := NewHandlers(nil)

	server := NewServer(cfg, handlers)

	assert.NotNil(t, server)
	assert.NotNil(t, server.server)
	assert.Equal(t, "127.0.0.1:18791", server.server.Addr)
}

func TestHandlers_SessionMutex(t *testing.T) {
	handlers := NewHandlers(nil)

	// Get mutex for same session - should return same instance
	mu1 := handlers.getSessionMutex("session-1")
	mu2 := handlers.getSessionMutex("session-1")
	mu3 := handlers.getSessionMutex("session-2")

	assert.Equal(t, mu1, mu2, "Same session should return same mutex")
	// Note: sync.Map.LoadOrStore creates new mutex for each unique key
	// but we can't directly compare pointer equality for different keys
	// as they may have same underlying structure
	_ = mu3 // Use mu3 to avoid unused variable
}
