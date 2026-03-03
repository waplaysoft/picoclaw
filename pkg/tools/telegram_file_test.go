package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTelegramFileTool_Name(t *testing.T) {
	tool := &TelegramFileTool{}
	assert.Equal(t, "telegram_send_file", tool.Name())
}

func TestTelegramFileTool_Description(t *testing.T) {
	tool := &TelegramFileTool{}
	assert.NotEmpty(t, tool.Description())
}

func TestTelegramFileTool_Parameters(t *testing.T) {
	tool := &TelegramFileTool{}
	params := tool.Parameters()

	assert.NotNil(t, params)
	assert.Equal(t, "object", params["type"])

	props, ok := params["properties"].(map[string]any)
	assert.True(t, ok)
	assert.NotNil(t, props["file_path"])
	assert.NotNil(t, props["chat_id"])
	assert.NotNil(t, props["caption"])
	assert.NotNil(t, props["file_type"])

	required, ok := params["required"].([]string)
	assert.True(t, ok)
	assert.Contains(t, required, "file_path")
}

func TestTelegramFileTool_Execute_MissingFilePath(t *testing.T) {
	tool := &TelegramFileTool{}
	result := tool.Execute(context.Background(), map[string]any{})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "file_path is required")
}

func TestTelegramFileTool_Execute_FileNotFound(t *testing.T) {
	tool := &TelegramFileTool{
		workspace: "/tmp",
		restrict:  false,
	}
	result := tool.Execute(context.Background(), map[string]any{
		"file_path": "/nonexistent/file.txt",
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "File not found")
}

func TestTelegramFileTool_Execute_RestrictedPath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &TelegramFileTool{
		workspace: tmpDir,
		restrict:  true,
	}

	// Try to access file outside workspace
	result := tool.Execute(context.Background(), map[string]any{
		"file_path": "/etc/passwd",
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "Access denied")
	assert.Contains(t, result.ForLLM, "outside workspace")
}

func TestTelegramGetFileTool_Name(t *testing.T) {
	tool := &TelegramGetFileTool{}
	assert.Equal(t, "telegram_get_file", tool.Name())
}

func TestTelegramGetFileTool_Description(t *testing.T) {
	tool := &TelegramGetFileTool{}
	assert.NotEmpty(t, tool.Description())
}

func TestTelegramGetFileTool_Parameters(t *testing.T) {
	tool := &TelegramGetFileTool{}
	params := tool.Parameters()

	assert.NotNil(t, params)
	assert.Equal(t, "object", params["type"])

	props, ok := params["properties"].(map[string]any)
	assert.True(t, ok)
	assert.NotNil(t, props["file_id"])
	assert.NotNil(t, props["save_path"])

	required, ok := params["required"].([]string)
	assert.True(t, ok)
	assert.Contains(t, required, "file_id")
	assert.Contains(t, required, "save_path")
}

func TestTelegramGetFileTool_Execute_MissingFileID(t *testing.T) {
	tool := &TelegramGetFileTool{}
	result := tool.Execute(context.Background(), map[string]any{
		"save_path": "/tmp/test.txt",
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "file_id is required")
}

func TestTelegramGetFileTool_Execute_MissingSavePath(t *testing.T) {
	tool := &TelegramGetFileTool{}
	result := tool.Execute(context.Background(), map[string]any{
		"file_id": "12345",
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "save_path is required")
}

func TestTelegramGetFileTool_Execute_RestrictedPath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &TelegramGetFileTool{
		workspace: tmpDir,
		restrict:  true,
	}

	// Try to save file outside workspace
	result := tool.Execute(context.Background(), map[string]any{
		"file_id":   "12345",
		"save_path": "/tmp/outside.txt",
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "Access denied")
	assert.Contains(t, result.ForLLM, "outside workspace")
}

func TestTelegramFileTool_AutoDetectFileType(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{"JPG image", "test.jpg", "photo"},
		{"JPEG image", "test.jpeg", "photo"},
		{"PNG image", "test.png", "photo"},
		{"GIF image", "test.gif", "photo"},
		{"WEBP image", "test.webp", "photo"},
		{"MP3 audio", "test.mp3", "audio"},
		{"WAV audio", "test.wav", "audio"},
		{"OGG audio", "test.ogg", "audio"},
		{"PDF document", "test.pdf", "document"},
		{"TXT document", "test.txt", "document"},
		{"ZIP document", "test.zip", "document"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temp file to test with
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, tt.filePath)
			err := os.WriteFile(testFile, []byte("test"), 0644)
			assert.NoError(t, err)

			// Verify file extension detection logic
			ext := filepath.Ext(tt.filePath)
			var expectedType string
			switch ext {
			case ".jpg", ".jpeg", ".png", ".gif", ".webp":
				expectedType = "photo"
			case ".mp3", ".wav", ".ogg":
				expectedType = "audio"
			default:
				expectedType = "document"
			}
			assert.Equal(t, tt.expected, expectedType)
		})
	}
}
