package tools

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/sipeed/picoclaw/pkg/channels"
)

// TelegramFileTool allows agents to send files via Telegram
type TelegramFileTool struct {
	channelManager *channels.Manager
	workspace      string
	restrict       bool
}

// NewTelegramFileTool creates a new Telegram file sending tool
func NewTelegramFileTool(channelManager *channels.Manager, workspace string, restrict bool) *TelegramFileTool {
	return &TelegramFileTool{
		channelManager: channelManager,
		workspace:      workspace,
		restrict:       restrict,
	}
}

func (t *TelegramFileTool) Name() string {
	return "telegram_send_file"
}

func (t *TelegramFileTool) Description() string {
	return "Send a file (image, document, audio) to a Telegram chat. Use this when you need to share files with users."
}

func (t *TelegramFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to the file to send (absolute path or relative to workspace)",
			},
			"chat_id": map[string]any{
				"type":        "string",
				"description": "Target Telegram chat ID (optional, uses current chat if not specified)",
			},
			"caption": map[string]any{
				"type":        "string",
				"description": "Optional caption for the file",
			},
			"file_type": map[string]any{
				"type":        "string",
				"description": "Type of file: 'photo', 'document', or 'audio' (auto-detected if not specified)",
				"enum":        []string{"photo", "document", "audio"},
			},
		},
		"required": []string{"file_path"},
	}
}

func (t *TelegramFileTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return &ToolResult{ForLLM: "file_path is required", IsError: true}
	}

	caption, _ := args["caption"].(string)
	fileType, _ := args["file_type"].(string)
	chatID, _ := args["chat_id"].(string)

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
	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return &ToolResult{ForLLM: fmt.Sprintf("File not found: %s", resolvedPath), IsError: true}
	}

	// Get Telegram channel
	ch, ok := t.channelManager.GetChannel("telegram")
	if !ok {
		return &ToolResult{ForLLM: "Telegram channel not available", IsError: true}
	}

	telegramChannel, ok := ch.(*channels.TelegramChannel)
	if !ok {
		return &ToolResult{ForLLM: "Failed to get Telegram channel instance", IsError: true}
	}

	// Get bot instance
	bot := telegramChannel.GetBot()
	if bot == nil {
		return &ToolResult{ForLLM: "Telegram bot not available", IsError: true}
	}

	// Determine target chat ID
	if chatID == "" {
		// Use current chat from channel state
		chatID = telegramChannel.GetCurrentChatID()
		if chatID == "" {
			return &ToolResult{ForLLM: "No chat_id specified and no current chat available", IsError: true}
		}
	}

	// Parse chat ID
	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Invalid chat ID: %v", err), IsError: true}
	}

	// Auto-detect file type if not specified
	if fileType == "" {
		ext := filepath.Ext(filePath)
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
			fileType = "photo"
		case ".mp3", ".wav", ".ogg":
			fileType = "audio"
		default:
			fileType = "document"
		}
	}

	// Send file based on type
	var sendErr error
	switch fileType {
	case "photo":
		sendErr = t.sendPhoto(ctx, bot, chatIDInt, resolvedPath, caption)
	case "audio":
		sendErr = t.sendAudio(ctx, bot, chatIDInt, resolvedPath, caption)
	default:
		sendErr = t.sendDocument(ctx, bot, chatIDInt, resolvedPath, caption)
	}

	if sendErr != nil {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("Failed to send file: %v", sendErr),
			IsError: true,
			Err:     sendErr,
		}
	}

	return &ToolResult{
		ForLLM: fmt.Sprintf("File sent successfully: %s", filepath.Base(resolvedPath)),
		Silent: true,
	}
}

func (t *TelegramFileTool) sendPhoto(ctx context.Context, bot *telego.Bot, chatID int64, filePath, caption string) error {
	// Open file for upload
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	params := &telego.SendPhotoParams{
		ChatID: tu.ID(chatID),
		Photo:  tu.File(file),
	}
	if caption != "" {
		params.Caption = caption
	}

	_, err = bot.SendPhoto(ctx, params)
	return err
}

func (t *TelegramFileTool) sendAudio(ctx context.Context, bot *telego.Bot, chatID int64, filePath, caption string) error {
	// Open file for upload
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	params := &telego.SendAudioParams{
		ChatID: tu.ID(chatID),
		Audio:  tu.File(file),
	}
	if caption != "" {
		params.Caption = caption
	}

	_, err = bot.SendAudio(ctx, params)
	return err
}

func (t *TelegramFileTool) sendDocument(ctx context.Context, bot *telego.Bot, chatID int64, filePath, caption string) error {
	// Open file for upload
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	params := &telego.SendDocumentParams{
		ChatID:   tu.ID(chatID),
		Document: tu.File(file),
	}
	if caption != "" {
		params.Caption = caption
	}

	_, err = bot.SendDocument(ctx, params)
	return err
}

// TelegramGetFileTool allows agents to retrieve files from Telegram messages
type TelegramGetFileTool struct {
	channelManager *channels.Manager
	workspace      string
	restrict       bool
}

// NewTelegramGetFileTool creates a new Telegram file retrieval tool
func NewTelegramGetFileTool(channelManager *channels.Manager, workspace string, restrict bool) *TelegramGetFileTool {
	return &TelegramGetFileTool{
		channelManager: channelManager,
		workspace:      workspace,
		restrict:       restrict,
	}
}

func (t *TelegramGetFileTool) Name() string {
	return "telegram_get_file"
}

func (t *TelegramGetFileTool) Description() string {
	return "Download a file from a Telegram message by file_id. Use this to retrieve files sent by users."
}

func (t *TelegramGetFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_id": map[string]any{
				"type":        "string",
				"description": "Telegram file ID from a message",
			},
			"save_path": map[string]any{
				"type":        "string",
				"description": "Path to save the file (relative to workspace or absolute)",
			},
		},
		"required": []string{"file_id", "save_path"},
	}
}

func (t *TelegramGetFileTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	fileID, ok := args["file_id"].(string)
	if !ok {
		return &ToolResult{ForLLM: "file_id is required", IsError: true}
	}

	savePath, ok := args["save_path"].(string)
	if !ok {
		return &ToolResult{ForLLM: "save_path is required", IsError: true}
	}

	// Resolve save path
	resolvedPath := savePath
	if !filepath.IsAbs(savePath) {
		resolvedPath = filepath.Join(t.workspace, savePath)
	}

	// Security check: if restrict is enabled, ensure file is within workspace
	if t.restrict {
		cleanPath, err := filepath.Abs(resolvedPath)
		if err != nil {
			return &ToolResult{ForLLM: fmt.Sprintf("Invalid save path: %v", err), IsError: true}
		}
		cleanWorkspace, err := filepath.Abs(t.workspace)
		if err != nil {
			return &ToolResult{ForLLM: fmt.Sprintf("Invalid workspace path: %v", err), IsError: true}
		}
		if !filepath.HasPrefix(cleanPath, cleanWorkspace) {
			return &ToolResult{
				ForLLM:  fmt.Sprintf("Access denied: save path %q is outside workspace", savePath),
				IsError: true,
			}
		}
	}

	// Get Telegram channel
	ch, ok := t.channelManager.GetChannel("telegram")
	if !ok {
		return &ToolResult{ForLLM: "Telegram channel not available", IsError: true}
	}

	telegramChannel, ok := ch.(*channels.TelegramChannel)
	if !ok {
		return &ToolResult{ForLLM: "Failed to get Telegram channel instance", IsError: true}
	}

	// Get bot instance
	bot := telegramChannel.GetBot()
	if bot == nil {
		return &ToolResult{ForLLM: "Telegram bot not available", IsError: true}
	}

	// Get file info from Telegram
	file, err := bot.GetFile(ctx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("Failed to get file info: %v", err),
			IsError: true,
			Err:     err,
		}
	}

	// Download file
	downloadURL := bot.FileDownloadURL(file.FilePath)
	if downloadURL == "" {
		return &ToolResult{ForLLM: "Failed to get download URL", IsError: true}
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("Failed to create directory: %v", err),
			IsError: true,
			Err:     err,
		}
	}

	// Download the file
	downloadedPath, err := downloadFileFromURL(ctx, downloadURL, resolvedPath)
	if err != nil {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("Failed to download file: %v", err),
			IsError: true,
			Err:     err,
		}
	}

	return &ToolResult{
		ForLLM: fmt.Sprintf("File downloaded successfully to: %s", downloadedPath),
		Silent: true,
	}
}

// downloadFileFromURL downloads a file from URL to the specified path
func downloadFileFromURL(ctx context.Context, url, savePath string) (string, error) {
	resp, err := httpGet(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	file, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = file.ReadFrom(resp.Body)
	if err != nil {
		os.Remove(savePath) // Clean up on error
		return "", err
	}

	return savePath, nil
}

// httpGet performs an HTTP GET request with context
func httpGet(ctx context.Context, url string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}
