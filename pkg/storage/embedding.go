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
)

// EmbeddingClient provides interface for generating embeddings
type EmbeddingClient interface {
	// GenerateEmbedding generates embedding vector for the given text
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	// GenerateEmbeddingsBatch generates embeddings for multiple texts in a single request
	GenerateEmbeddingsBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// MistralEmbeddingClient implements EmbeddingClient using Mistral AI API
type MistralEmbeddingClient struct {
	apiKey     string
	apiBase    string
	model      string
	httpClient *http.Client
}

// MistralEmbeddingRequest represents the request body for Mistral embeddings API
type MistralEmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
}

// MistralEmbeddingResponse represents the response from Mistral embeddings API
type MistralEmbeddingResponse struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Model  string `json:"model"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Data []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// NewMistralEmbeddingClient creates a new Mistral embedding client
func NewMistralEmbeddingClient(apiKey, apiBase, model string) *MistralEmbeddingClient {
	if apiBase == "" {
		apiBase = "https://api.mistral.ai/v1"
	}
	if model == "" {
		model = "mistral-embed"
	}

	return &MistralEmbeddingClient{
		apiKey:  apiKey,
		apiBase: apiBase,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GenerateEmbedding generates embedding vector for the given text using Mistral API
func (c *MistralEmbeddingClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Mistral API key is not configured")
	}

	reqBody := MistralEmbeddingRequest{
		Model:          c.model,
		Input:          []string{text},
		EncodingFormat: "float",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	url := c.apiBase + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to generate embedding: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var respBody MistralEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	if len(respBody.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned from Mistral API")
	}

	return respBody.Data[0].Embedding, nil
}

// GenerateEmbeddingsBatch generates embeddings for multiple texts in a single request
func (c *MistralEmbeddingClient) GenerateEmbeddingsBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Mistral API key is not configured")
	}

	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	reqBody := MistralEmbeddingRequest{
		Model:          c.model,
		Input:          texts,
		EncodingFormat: "float",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	url := c.apiBase + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to generate embeddings: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var respBody MistralEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	embeddings := make([][]float32, len(respBody.Data))
	for i, item := range respBody.Data {
		embeddings[i] = item.Embedding
	}

	return embeddings, nil
}
