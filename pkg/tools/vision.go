package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// ReadImageTool allows agents to analyze images from the filesystem using vision models
type ReadImageTool struct {
	workspace string
	restrict  bool
	provider  providers.LLMProvider
	model     string
}

// NewReadImageTool creates a new image analysis tool
func NewReadImageTool(workspace string, restrict bool, provider providers.LLMProvider, model string) *ReadImageTool {
	return &ReadImageTool{
		workspace: workspace,
		restrict:  restrict,
		provider:  provider,
		model:     model,
	}
}

func (t *ReadImageTool) Name() string {
	return "read_image"
}

func (t *ReadImageTool) Description() string {
	return "Analyze an image file from the filesystem using AI vision. Use this when you need to understand what's in an image, read text from it, or describe its contents."
}

func (t *ReadImageTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the image file (absolute or relative to workspace)",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "Optional: specific question or instruction about the image (e.g., 'What text is in this image?', 'Describe the scene')",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadImageTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	path, ok := args["path"].(string)
	if !ok {
		return &ToolResult{ForLLM: "path is required", IsError: true}
	}

	prompt, _ := args["prompt"].(string)

	// Resolve file path
	resolvedPath := path
	if !filepath.IsAbs(path) {
		resolvedPath = filepath.Join(t.workspace, path)
	}

	// Security check: if restrict is enabled, ensure file is within workspace
	if t.restrict {
		cleanPath, err := filepath.Abs(resolvedPath)
		if err != nil {
			return &ToolResult{ForLLM: fmt.Sprintf("Invalid file path: %v", err), IsError: true}
		}
		cleanWorkspace, err := filepath.Abs(t.workspace)
		if err != nil {
			return &ToolResult{ForLLM: fmt.Sprintf("Invalid workspace path: %v", err), IsError: true}
		}
		if !filepath.HasPrefix(cleanPath, cleanWorkspace) {
			return &ToolResult{
				ForLLM:  fmt.Sprintf("Access denied: file path %q is outside workspace", path),
				IsError: true,
			}
		}
	}

	// Check if file exists
	fileInfo, err := os.Stat(resolvedPath)
	if os.IsNotExist(err) {
		return &ToolResult{ForLLM: fmt.Sprintf("File not found: %s", resolvedPath), IsError: true}
	}
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Error accessing file: %v", err), IsError: true}
	}

	// Check file size (limit to 10MB)
	maxSize := int64(10 * 1024 * 1024) // 10MB
	if fileInfo.Size() > maxSize {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("File too large: %d bytes (max: %d bytes)", fileInfo.Size(), maxSize),
			IsError: true,
		}
	}

	// Read file
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Failed to read file: %v", err), IsError: true}
	}

	// Detect MIME type
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("File is not an image (detected type: %s)", mimeType),
			IsError: true,
		}
	}

	// Validate image extension
	ext := strings.ToLower(filepath.Ext(resolvedPath))
	validExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp"}
	validExt := false
	for _, valid := range validExts {
		if ext == valid {
			validExt = true
			break
		}
	}
	if !validExt {
		// Try to infer from MIME type
		extensions, err := mime.ExtensionsByType(mimeType)
		if err != nil || len(extensions) == 0 {
			return &ToolResult{
				ForLLM:  fmt.Sprintf("Unsupported image format: %s", mimeType),
				IsError: true,
			}
		}
	}

	// Encode as base64
	base64Data := base64.StdEncoding.EncodeToString(data)

	logger.InfoCF("tool", "Analyzing image with vision model",
		map[string]any{
			"path":      resolvedPath,
			"size":      len(data),
			"mime_type": mimeType,
		})

	// Build message for vision model
	userMessage := "Please analyze this image and describe what you see."
	if prompt != "" {
		userMessage = prompt
	}

	messages := []providers.Message{
		{
			Role: "user",
			MultiContent: []providers.ImageContent{
				{
					Type:       "text",
					Base64Data: userMessage,
				},
				{
					Type:       "image",
					Base64Data: base64Data,
					MIMEType:   mimeType,
				},
			},
		},
	}

	// Call LLM with vision capabilities
	resp, err := t.provider.Chat(ctx, messages, nil, t.model, map[string]any{
		"max_tokens": 1024,
	})
	if err != nil {
		logger.ErrorCF("tool", "Vision model analysis failed",
			map[string]any{
				"error": err.Error(),
				"path":  resolvedPath,
			})
		return &ToolResult{
			ForLLM:  fmt.Sprintf("Failed to analyze image: %v", err),
			IsError: true,
			Err:     err,
		}
	}

	logger.InfoCF("tool", "Image analysis completed",
		map[string]any{
			"path":      resolvedPath,
			"response_len": len(resp.Content),
		})

	return &ToolResult{
		ForLLM: resp.Content,
		Silent: false,
	}
}
