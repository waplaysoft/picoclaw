package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/storage"
)

type Session struct {
	Key      string              `json:"key"`
	Messages []providers.Message `json:"messages"`
	Summary  string              `json:"summary,omitempty"`
	Created  time.Time           `json:"created"`
	Updated  time.Time           `json:"updated"`
}

type SessionManager struct {
	sessions     map[string]*Session
	mu           sync.RWMutex
	storage      string
	messageStore *storage.MessageStore
}

func NewSessionManager(storagePath string) *SessionManager {
	return NewSessionManagerWithConfig(storagePath, config.StorageConfig{})
}

// NewSessionManagerWithConfig creates a new SessionManager with the given storage configuration
func NewSessionManagerWithConfig(storagePath string, storageCfg config.StorageConfig) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		storage:  storagePath,
	}

	if storagePath != "" {
		os.MkdirAll(storagePath, 0o755)
		sm.loadSessions()
	}

	// Initialize message store if Qdrant is configured
	if storageCfg.Qdrant.Enabled {
		var err error
		sm.messageStore, err = storage.NewMessageStore(storageCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Qdrant] Failed to create message store in SessionManager: %v\n", err)
		} else if sm.messageStore.IsEnabled() {
			fmt.Fprintf(os.Stderr, "[Qdrant] SessionManager initialized (collection: %s)\n", storageCfg.Qdrant.Collection)
		}
	}

	return sm
}

func (sm *SessionManager) GetOrCreate(key string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[key]
	if ok {
		return session
	}

	session = &Session{
		Key:      key,
		Messages: []providers.Message{},
		Created:  time.Now(),
		Updated:  time.Now(),
	}
	sm.sessions[key] = session

	return session
}

func (sm *SessionManager) AddMessage(sessionKey, role, content string) {
	sm.AddFullMessage(sessionKey, providers.Message{
		Role:    role,
		Content: content,
	})
}

// AddFullMessage adds a complete message with tool calls and tool call ID to the session.
// This is used to save the full conversation flow including tool calls and tool results.
func (sm *SessionManager) AddFullMessage(sessionKey string, msg providers.Message) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionKey]
	if !ok {
		session = &Session{
			Key:      sessionKey,
			Messages: []providers.Message{},
			Created:  time.Now(),
		}
		sm.sessions[sessionKey] = session
	}

	session.Messages = append(session.Messages, msg)
	session.Updated = time.Now()

	// Also store in Qdrant if enabled
	if sm.messageStore != nil && sm.messageStore.IsEnabled() {
		// Skip heartbeat messages
		if sessionKey == "heartbeat" {
			return
		}

		// Skip tool and system messages (internal agent messages)
		if msg.Role == "tool" || msg.Role == "system" {
			return
		}

		// Skip messages with empty content
		if msg.Content == "" {
			return
		}

		// Unlock before calling external store to avoid holding lock during I/O
		sm.mu.Unlock()
		defer sm.mu.Lock()

		index := len(session.Messages) - 1
		if err := sm.messageStore.StoreMessage(sessionKey, msg, index); err != nil {
			fmt.Fprintf(os.Stderr, "[Qdrant] Failed to store message: %v\n", err)
		}
	}
}

func (sm *SessionManager) GetHistory(key string) []providers.Message {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[key]
	if !ok {
		return []providers.Message{}
	}

	history := make([]providers.Message, len(session.Messages))
	copy(history, session.Messages)
	return history
}

func (sm *SessionManager) GetSummary(key string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[key]
	if !ok {
		return ""
	}
	return session.Summary
}

func (sm *SessionManager) SetSummary(key string, summary string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[key]
	if ok {
		session.Summary = summary
		session.Updated = time.Now()
	}
}

func (sm *SessionManager) TruncateHistory(key string, keepLast int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[key]
	if !ok {
		return
	}

	if keepLast <= 0 {
		session.Messages = []providers.Message{}
		session.Updated = time.Now()
		return
	}

	if len(session.Messages) <= keepLast {
		return
	}

	session.Messages = session.Messages[len(session.Messages)-keepLast:]
	session.Updated = time.Now()
}

// sanitizeFilename converts a session key into a cross-platform safe filename.
// Session keys use "channel:chatID" (e.g. "telegram:123456") but ':' is the
// volume separator on Windows, so filepath.Base would misinterpret the key.
// We replace it with '_'. The original key is preserved inside the JSON file,
// so loadSessions still maps back to the right in-memory key.
func sanitizeFilename(key string) string {
	return strings.ReplaceAll(key, ":", "_")
}

func (sm *SessionManager) Save(key string) error {
	if sm.storage == "" {
		return nil
	}

	filename := sanitizeFilename(key)

	// filepath.IsLocal rejects empty names, "..", absolute paths, and
	// OS-reserved device names (NUL, COM1 … on Windows).
	// The extra checks reject "." and any directory separators so that
	// the session file is always written directly inside sm.storage.
	if filename == "." || !filepath.IsLocal(filename) || strings.ContainsAny(filename, `/\`) {
		return os.ErrInvalid
	}

	// Snapshot under read lock, then perform slow file I/O after unlock.
	sm.mu.RLock()
	stored, ok := sm.sessions[key]
	if !ok {
		sm.mu.RUnlock()
		return nil
	}

	snapshot := Session{
		Key:     stored.Key,
		Summary: stored.Summary,
		Created: stored.Created,
		Updated: stored.Updated,
	}
	if len(stored.Messages) > 0 {
		snapshot.Messages = make([]providers.Message, len(stored.Messages))
		copy(snapshot.Messages, stored.Messages)
	} else {
		snapshot.Messages = []providers.Message{}
	}
	sm.mu.RUnlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	sessionPath := filepath.Join(sm.storage, filename+".json")
	tmpFile, err := os.CreateTemp(sm.storage, "session-*.tmp")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(0o644); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, sessionPath); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func (sm *SessionManager) loadSessions() error {
	files, err := os.ReadDir(sm.storage)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		sessionPath := filepath.Join(sm.storage, file.Name())
		data, err := os.ReadFile(sessionPath)
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		sm.sessions[session.Key] = &session
	}

	return nil
}

// SetHistory updates the messages of a session.
func (sm *SessionManager) SetHistory(key string, history []providers.Message) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[key]
	if ok {
		// Create a deep copy to strictly isolate internal state
		// from the caller's slice.
		msgs := make([]providers.Message, len(history))
		copy(msgs, history)
		session.Messages = msgs
		session.Updated = time.Now()
	}
}

// SearchSimilarMessages searches for messages similar to the query using vector search.
// Returns messages from Qdrant if enabled, otherwise returns empty slice.
func (sm *SessionManager) SearchSimilarMessages(sessionKey, query string, limit int) ([]providers.Message, error) {
	if sm.messageStore == nil || !sm.messageStore.IsEnabled() {
		return []providers.Message{}, nil
	}

	return sm.messageStore.SearchSimilarMessages(sessionKey, query, limit)
}
