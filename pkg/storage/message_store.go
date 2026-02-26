// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers/protocoltypes"
)

// MessageStore provides persistent storage for chat messages with vector search
type MessageStore struct {
	qdrantClient      *QdrantClient
	embeddingClient   EmbeddingClient
	config            config.QdrantConfig
	enabled           bool
	mu                sync.RWMutex
	pointCounter      int64
}

// StoredMessage represents a message ready for storage
type StoredMessage struct {
	SessionKey string
	Message    protocoltypes.Message
	Timestamp  time.Time
	Index      int
}

// NewMessageStore creates a new message store with the given configuration
func NewMessageStore(cfg config.StorageConfig) (*MessageStore, error) {
	store := &MessageStore{
		config:  cfg.Qdrant,
		enabled: cfg.Qdrant.Enabled,
	}

	if !store.enabled {
		return store, nil
	}

	// Initialize Qdrant client
	store.qdrantClient = NewQdrantClient(cfg.Qdrant)

	// Initialize embedding client (Mistral)
	// Use embedding config from storage.embedding
	embedCfg := cfg.Embedding
	if embedCfg.APIKey == "" {
		// Fallback: try to find mistral-embed in model_list via environment
		// The key should be available via PICOCLAW_EMBEDDING_API_KEY env var
		embedCfg.APIBase = "https://api.mistral.ai/v1"
		embedCfg.Model = "mistral-embed"
	}

	store.embeddingClient = NewMistralEmbeddingClient(
		embedCfg.APIKey,
		embedCfg.APIBase,
		embedCfg.Model,
	)

	// Ensure collection exists
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := store.qdrantClient.CreateCollection(ctx); err != nil {
		return nil, fmt.Errorf("failed to create Qdrant collection: %w", err)
	}

	return store, nil
}

// NewMessageStoreWithClients creates a message store with pre-configured clients
func NewMessageStoreWithClients(cfg config.QdrantConfig, embeddingClient EmbeddingClient) (*MessageStore, error) {
	store := &MessageStore{
		config:          cfg,
		enabled:         cfg.Enabled,
		embeddingClient: embeddingClient,
	}

	if !store.enabled {
		return store, nil
	}

	store.qdrantClient = NewQdrantClient(cfg)

	// Ensure collection exists
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := store.qdrantClient.CreateCollection(ctx); err != nil {
		return nil, fmt.Errorf("failed to create Qdrant collection: %w", err)
	}

	return store, nil
}

// IsEnabled returns whether the message store is enabled
func (s *MessageStore) IsEnabled() bool {
	return s.enabled
}

// StoreMessage stores a message in the vector database
func (s *MessageStore) StoreMessage(sessionKey string, msg protocoltypes.Message, index int) error {
	if !s.enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate embedding for message content
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vector, err := s.embeddingClient.GenerateEmbedding(ctx, msg.Content)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Create payload
	payload := MessagePayload{
		SessionKey:   sessionKey,
		Role:         msg.Role,
		Content:      msg.Content,
		Timestamp:    time.Now(),
		MessageIndex: index,
	}

	payloadMap, err := structToMap(payload)
	if err != nil {
		return fmt.Errorf("failed to convert payload to map: %w", err)
	}

	// Create point
	s.pointCounter++
	point := Point{
		ID:      s.pointCounter,
		Vector:  vector,
		Payload: payloadMap,
	}

	// Upsert to Qdrant
	if err := s.qdrantClient.UpsertPoints(ctx, []Point{point}); err != nil {
		return fmt.Errorf("failed to upsert point to Qdrant: %w", err)
	}

	return nil
}

// StoreMessages stores multiple messages in batch
func (s *MessageStore) StoreMessages(messages []StoredMessage) error {
	if !s.enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate embeddings for all messages
	texts := make([]string, len(messages))
	for i, msg := range messages {
		texts[i] = msg.Message.Content
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	vectors, err := s.embeddingClient.GenerateEmbeddingsBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Create points
	points := make([]Point, len(messages))
	for i, msg := range messages {
		s.pointCounter++

		payload := MessagePayload{
			SessionKey:   msg.SessionKey,
			Role:         msg.Message.Role,
			Content:      msg.Message.Content,
			Timestamp:    msg.Timestamp,
			MessageIndex: msg.Index,
		}

		payloadMap, err := structToMap(payload)
		if err != nil {
			return fmt.Errorf("failed to convert payload to map: %w", err)
		}

		points[i] = Point{
			ID:      s.pointCounter,
			Vector:  vectors[i],
			Payload: payloadMap,
		}
	}

	// Upsert to Qdrant
	if err := s.qdrantClient.UpsertPoints(ctx, points); err != nil {
		return fmt.Errorf("failed to upsert points to Qdrant: %w", err)
	}

	return nil
}

// SearchSimilarMessages finds messages similar to the query text
func (s *MessageStore) SearchSimilarMessages(sessionKey, query string, limit int) ([]protocoltypes.Message, error) {
	if !s.enabled {
		return []protocoltypes.Message{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Generate embedding for query
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vector, err := s.embeddingClient.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search in Qdrant
	results, err := s.qdrantClient.Search(ctx, vector, sessionKey, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search Qdrant: %w", err)
	}

	// Convert results to messages
	messages := make([]protocoltypes.Message, 0, len(results))
	for _, result := range results {
		msg, err := payloadToMessage(result.Payload)
		if err != nil {
			// Log error but continue with other results
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// SearchSimilarMessagesWithPayload finds messages similar to the query text and returns full payload
// This is used by tools that need access to all message metadata
func (s *MessageStore) SearchSimilarMessagesWithPayload(sessionKey, query string, limit int) ([]MessagePayload, error) {
	if !s.enabled {
		return []MessagePayload{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Generate embedding for query
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vector, err := s.embeddingClient.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search in Qdrant
	results, err := s.qdrantClient.Search(ctx, vector, sessionKey, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search Qdrant: %w", err)
	}

	// Convert results to payloads
	messages := make([]MessagePayload, 0, len(results))
	for _, result := range results {
		payload, err := payloadToMessagePayload(result.Payload)
		if err != nil {
			// Log error but continue with other results
			continue
		}
		messages = append(messages, payload)
	}

	return messages, nil
}

// DeleteSessionMessages deletes all messages for a session
func (s *MessageStore) DeleteSessionMessages(sessionKey string) error {
	if !s.enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.qdrantClient.DeleteBySessionKey(ctx, sessionKey); err != nil {
		return fmt.Errorf("failed to delete session messages: %w", err)
	}

	return nil
}

// structToMap converts a struct to a map for Qdrant payload
func structToMap(payload MessagePayload) (map[string]any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// payloadToMessage converts a Qdrant payload back to a Message
func payloadToMessage(payload map[string]any) (protocoltypes.Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return protocoltypes.Message{}, err
	}

	var msgPayload MessagePayload
	if err := json.Unmarshal(data, &msgPayload); err != nil {
		return protocoltypes.Message{}, err
	}

	return protocoltypes.Message{
		Role:    msgPayload.Role,
		Content: msgPayload.Content,
	}, nil
}

// payloadToMessagePayload converts a Qdrant payload to MessagePayload
func payloadToMessagePayload(payload map[string]any) (MessagePayload, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return MessagePayload{}, err
	}

	var msgPayload MessagePayload
	if err := json.Unmarshal(data, &msgPayload); err != nil {
		return MessagePayload{}, err
	}

	return msgPayload, nil
}
