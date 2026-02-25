// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

// QdrantClient provides connection to Qdrant vector database
type QdrantClient struct {
	config     config.QdrantConfig
	httpClient *http.Client
	baseURL    string
}

// Point represents a Qdrant point with vector and payload
type Point struct {
	ID      int64             `json:"id"`
	Vector  []float32         `json:"vector"`
	Payload map[string]any    `json:"payload"`
}

// MessagePayload represents the payload structure for stored messages
type MessagePayload struct {
	SessionKey   string    `json:"session_key"`
	Role         string    `json:"role"`
	Content      string    `json:"content"`
	Timestamp    time.Time `json:"timestamp"`
	MessageIndex int       `json:"message_index"`
}

// SearchRequest represents a Qdrant search request
type SearchRequest struct {
	Vector      []float32         `json:"vector"`
	Limit       int               `json:"limit"`
	WithPayload bool              `json:"with_payload"`
	Filter      *FilterCondition  `json:"filter,omitempty"`
}

// FilterCondition represents Qdrant filter conditions
type FilterCondition struct {
	Must []FilterClause `json:"must,omitempty"`
}

// FilterClause represents a single filter clause
type FilterClause struct {
	Key   string      `json:"key"`
	Match MatchCondition `json:"match"`
}

// MatchCondition represents a match condition
type MatchCondition struct {
	Value string `json:"value"`
}

// SearchResponse represents a Qdrant search response
type SearchResponse struct {
	Result []ScoredPoint `json:"result"`
}

// ScoredPoint represents a point with similarity score
type ScoredPoint struct {
	ID      int64             `json:"id"`
	Version int64             `json:"version"`
	Score   float32           `json:"score"`
	Payload map[string]any    `json:"payload"`
	Vector  []float32         `json:"vector,omitempty"`
}

// NewQdrantClient creates a new Qdrant client from config
func NewQdrantClient(cfg config.QdrantConfig) *QdrantClient {
	protocol := "http"
	if cfg.Secure {
		protocol = "https"
	}
	baseURL := fmt.Sprintf("%s://%s:%d", protocol, cfg.Host, cfg.Port)

	return &QdrantClient{
		config:  cfg,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateCollection creates the collection if it doesn't exist
func (c *QdrantClient) CreateCollection(ctx context.Context) error {
	collectionName := c.config.Collection
	vectorSize := c.config.VectorSize
	if vectorSize <= 0 {
		vectorSize = 1024 // default for mistral-embed
	}

	// Check if collection exists
	exists, err := c.CollectionExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Create collection
	createReq := map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}

	body, err := json.Marshal(createReq)
	if err != nil {
		return fmt.Errorf("failed to marshal create collection request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s", c.baseURL, collectionName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create collection: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// CollectionExists checks if the collection exists
func (c *QdrantClient) CollectionExists(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/collections/%s", c.baseURL, c.config.Collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	if c.config.APIKey != "" {
		req.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check collection existence: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("unexpected status checking collection: status=%d, body=%s", resp.StatusCode, string(body))
}

// UpsertPoints inserts or updates points in the collection
func (c *QdrantClient) UpsertPoints(ctx context.Context, points []Point) error {
	if len(points) == 0 {
		return nil
	}

	upsertReq := map[string]any{
		"points": points,
	}

	body, err := json.Marshal(upsertReq)
	if err != nil {
		return fmt.Errorf("failed to marshal upsert request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points", c.baseURL, c.config.Collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upsert points: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// Search performs a vector search in the collection
func (c *QdrantClient) Search(ctx context.Context, vector []float32, sessionKey string, limit int) ([]ScoredPoint, error) {
	searchReq := SearchRequest{
		Vector:      vector,
		Limit:       limit,
		WithPayload: true,
	}

	// Filter by session key if provided
	if sessionKey != "" {
		searchReq.Filter = &FilterCondition{
			Must: []FilterClause{
				{
					Key: "session_key",
					Match: MatchCondition{
						Value: sessionKey,
					},
				},
			},
		}
	}

	body, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", c.baseURL, c.config.Collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return searchResp.Result, nil
}

// DeleteBySessionKey deletes all points for a given session key
func (c *QdrantClient) DeleteBySessionKey(ctx context.Context, sessionKey string) error {
	deleteReq := map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "session_key",
					"match": map[string]any{
						"value": sessionKey,
					},
				},
			},
		},
	}

	body, err := json.Marshal(deleteReq)
	if err != nil {
		return fmt.Errorf("failed to marshal delete request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete", c.baseURL, c.config.Collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete points: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete points: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}
