package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// pendingFileLinks stores file links waiting to be added to WebUI responses
var pendingFileLinks = make(map[string]string) // session -> markdown link
var pendingFileLinksMu sync.RWMutex

// GetPendingFileLink returns and clears pending file link for a session
func GetPendingFileLink(session string) string {
	pendingFileLinksMu.Lock()
	defer pendingFileLinksMu.Unlock()
	
	link := pendingFileLinks[session]
	delete(pendingFileLinks, session)
	return link
}

// WebUISendFileTool allows agents to send files to WebUI users
type WebUISendFileTool struct {
	workspace        string
	restrict         bool
	msgBus           *bus.MessageBus
	currentSession   string
	currentChatID    string
}

// NewWebUISendFileTool creates a new WebUI file sending tool
func NewWebUISendFileTool(workspace string, restrict bool) *WebUISendFileTool {
	return &WebUISendFileTool{
		workspace: workspace,
		restrict:  restrict,
	}
}

// SetContext sets the current session/chatID for sending files
func (t *WebUISendFileTool) SetContext(channel, chatID, threadID string) {
	if channel == "webui" {
		t.currentSession = chatID
		t.currentChatID = chatID
	}
}

// SetMessageBus sets the message bus for sending outbound messages
func (t *WebUISendFileTool) SetMessageBus(msgBus *bus.MessageBus) {
	t.msgBus = msgBus
}

func (t *WebUISendFileTool) Name() string {
	return "webui_send_file"
}

func (t *WebUISendFileTool) Description() string {
	return "Send a file to the user in WebUI chat. Use this when you need to share a file that the user can download."
}

func (t *WebUISendFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to the file to send (absolute or relative to workspace)",
			},
			"caption": map[string]any{
				"type":        "string",
				"description": "Optional caption or description for the file",
			},
		},
		"required": []string{"file_path"},
	}
}

func (t *WebUISendFileTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return &ToolResult{ForLLM: "file_path is required", IsError: true}
	}

	caption, _ := args["caption"].(string)

	// Resolve file path
	resolvedPath := filePath
	if !filepath.IsAbs(filePath) {
		resolvedPath = filepath.Join(t.workspace, filePath)
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
				ForLLM:  fmt.Sprintf("Access denied: file path %q is outside workspace", filePath),
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

	// Check file size (limit to 50MB for WebUI)
	maxSize := int64(50 * 1024 * 1024) // 50MB
	if fileInfo.Size() > maxSize {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("File too large: %d bytes (max: %d bytes)", fileInfo.Size(), maxSize),
			IsError: true,
		}
	}

	logger.InfoCF("tool", "File sent via WebUI",
		map[string]any{
			"path":     resolvedPath,
			"caption":  caption,
			"size":     fileInfo.Size(),
		})

	// Extract session from path: /workspace/webui/uploads/{session}/filename
	session := t.currentSession
	if session == "" {
		session = "current"
		pathParts := strings.Split(resolvedPath, string(filepath.Separator))
		for i, part := range pathParts {
			if part == "webui" && i+1 < len(pathParts) && pathParts[i+1] == "uploads" && i+2 < len(pathParts) {
				session = pathParts[i+2]
				break
			}
		}
	}
	
	fileName := filepath.Base(resolvedPath)
	downloadLink := fmt.Sprintf("[📎 Download: %s](/api/files/download/%s/%s)", fileName, session, fileName)
	
	// Store link for handler to add to response
	pendingFileLinksMu.Lock()
	pendingFileLinks[session] = downloadLink
	pendingFileLinksMu.Unlock()
	
	// Return simple confirmation
	return &ToolResult{
		ForLLM: fmt.Sprintf("File prepared: %s", fileName),
		Silent: true,
	}
}

// formatFileSize formats file size in human-readable format
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}
