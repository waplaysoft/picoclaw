// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package storage

import (
	"context"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers/protocoltypes"
)

func TestMessageStore_NotEnabled(t *testing.T) {
	// Test that MessageStore works when disabled
	cfg := config.StorageConfig{
		Qdrant: config.QdrantConfig{
			Enabled: false,
		},
	}

	store, err := NewMessageStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create message store: %v", err)
	}

	if store.IsEnabled() {
		t.Error("MessageStore should be disabled")
	}

	// Test StoreMessage does nothing when disabled
	msg := protocoltypes.Message{
		Role:    "user",
		Content: "test message",
	}
	
	err = store.StoreMessage("test-session", msg, 0)
	if err != nil {
		t.Errorf("StoreMessage should not return error when disabled: %v", err)
	}

	// Test SearchSimilarMessages returns empty when disabled
	messages, err := store.SearchSimilarMessages("test-session", "query", 5)
	if err != nil {
		t.Errorf("SearchSimilarMessages should not return error when disabled: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("SearchSimilarMessages should return empty slice when disabled, got %d messages", len(messages))
	}
}

func TestMessagePayloadConversion(t *testing.T) {
	payload := MessagePayload{
		SessionKey:   "test-session",
		Role:         "user",
		Content:      "Hello, World!",
		Timestamp:    time.Now(),
		MessageIndex: 0,
	}

	// Convert to map
	payloadMap, err := structToMap(payload)
	if err != nil {
		t.Fatalf("Failed to convert payload to map: %v", err)
	}

	// Convert back to message
	msg, err := payloadToMessage(payloadMap)
	if err != nil {
		t.Fatalf("Failed to convert map to message: %v", err)
	}

	// Verify fields
	if msg.Role != payload.Role {
		t.Errorf("Role mismatch: expected %s, got %s", payload.Role, msg.Role)
	}

	if msg.Content != payload.Content {
		t.Errorf("Content mismatch: expected %s, got %s", payload.Content, msg.Content)
	}
}

func TestMistralClientCreation(t *testing.T) {
	// Test client creation with default values
	client := NewMistralEmbeddingClient("", "", "")
	if client == nil {
		t.Fatal("Failed to create Mistral client")
	}

	// Test client creation with custom values
	client = NewMistralEmbeddingClient(
		"test-key",
		"https://custom.api.com/v1",
		"custom-model",
	)
	if client == nil {
		t.Fatal("Failed to create Mistral client with custom config")
	}
}

func TestQdrantClientCreation(t *testing.T) {
	cfg := config.QdrantConfig{
		Host:       "localhost",
		Port:       6333,
		GRPCPort:   6334,
		Collection: "test-collection",
		VectorSize: 1024,
		Secure:     false,
	}

	client := NewQdrantClient(cfg)
	if client == nil {
		t.Fatal("Failed to create Qdrant client")
	}

	// Verify baseURL is correct
	expectedURL := "http://localhost:6333"
	if client.baseURL != expectedURL {
		t.Errorf("Expected baseURL %s, got %s", expectedURL, client.baseURL)
	}
}

func TestQdrantClientCreation_Secure(t *testing.T) {
	cfg := config.QdrantConfig{
		Host:       "cloud.qdrant.io",
		Port:       443,
		Collection: "test-collection",
		VectorSize: 1024,
		Secure:     true,
	}

	client := NewQdrantClient(cfg)
	if client == nil {
		t.Fatal("Failed to create Qdrant client")
	}

	// Verify baseURL uses HTTPS
	expectedURL := "https://cloud.qdrant.io:443"
	if client.baseURL != expectedURL {
		t.Errorf("Expected baseURL %s, got %s", expectedURL, client.baseURL)
	}
}

func TestMessageStore_WithMockEmbeddingClient(t *testing.T) {
	// Create a mock embedding client
	mockClient := &mockEmbeddingClient{
		embeddings: map[string][]float32{
			"test message": {0.1, 0.2, 0.3},
			"query":        {0.15, 0.25, 0.35},
		},
	}

	cfg := config.QdrantConfig{
		Enabled:    true,
		Host:       "localhost",
		Port:       6333,
		Collection: "test-collection",
		VectorSize: 3, // Use small vector size for testing
	}

	// This would test the full integration, but requires running Qdrant
	// For now, just verify the client can be created
	store, err := NewMessageStoreWithClients(cfg, mockClient)
	if err != nil {
		// Expected to fail if Qdrant is not running
		t.Logf("Note: MessageStore creation failed (Qdrant may not be running): %v", err)
	} else {
		if !store.IsEnabled() {
			t.Error("MessageStore should be enabled")
		}
	}
}

// mockEmbeddingClient is a test double for EmbeddingClient
type mockEmbeddingClient struct {
	embeddings map[string][]float32
}

func (m *mockEmbeddingClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if emb, ok := m.embeddings[text]; ok {
		return emb, nil
	}
	// Return default embedding for unknown texts
	return []float32{0.0, 0.0, 0.0}, nil
}

func (m *mockEmbeddingClient) GenerateEmbeddingsBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		if emb, ok := m.embeddings[text]; ok {
			result[i] = emb
		} else {
			result[i] = []float32{0.0, 0.0, 0.0}
		}
	}
	return result, nil
}
