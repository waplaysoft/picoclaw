package channels

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/utils"
	"github.com/sipeed/picoclaw/pkg/voice"
)

type TelegramChannel struct {
	*BaseChannel
	bot          *telego.Bot
	commands     TelegramCommander
	config       *config.Config
	chatIDs      map[string]int64
	transcriber  *voice.GroqTranscriber
	placeholders sync.Map // chatID -> messageID
	stopThinking sync.Map // chatID -> thinkingCancel
}

type thinkingCancel struct {
	fn context.CancelFunc
}

func (c *thinkingCancel) Cancel() {
	if c != nil && c.fn != nil {
		c.fn()
	}
}

func NewTelegramChannel(cfg *config.Config, bus *bus.MessageBus) (*TelegramChannel, error) {
	var opts []telego.BotOption
	telegramCfg := cfg.Channels.Telegram

	if telegramCfg.Proxy != "" {
		proxyURL, parseErr := url.Parse(telegramCfg.Proxy)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", telegramCfg.Proxy, parseErr)
		}
		opts = append(opts, telego.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}))
	} else if os.Getenv("HTTP_PROXY") != "" || os.Getenv("HTTPS_PROXY") != "" {
		// Use environment proxy if configured
		opts = append(opts, telego.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		}))
	}

	bot, err := telego.NewBot(telegramCfg.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	base := NewBaseChannel("telegram", telegramCfg, bus, telegramCfg.AllowFrom)

	return &TelegramChannel{
		BaseChannel:  base,
		commands:     NewTelegramCommands(bot, cfg),
		bot:          bot,
		config:       cfg,
		chatIDs:      make(map[string]int64),
		transcriber:  nil,
		placeholders: sync.Map{},
		stopThinking: sync.Map{},
	}, nil
}

func (c *TelegramChannel) SetTranscriber(transcriber *voice.GroqTranscriber) {
	c.transcriber = transcriber
}

func (c *TelegramChannel) Start(ctx context.Context) error {
	logger.InfoC("telegram", "Starting Telegram bot (polling mode)...")

	updates, err := c.bot.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{
		Timeout: 30,
	})
	if err != nil {
		return fmt.Errorf("failed to start long polling: %w", err)
	}

	bh, err := telegohandler.NewBotHandler(c.bot, updates)
	if err != nil {
		return fmt.Errorf("failed to create bot handler: %w", err)
	}

	bh.HandleMessage(func(ctx *th.Context, message telego.Message) error {
		c.commands.Help(ctx, message)
		return nil
	}, th.CommandEqual("help"))
	bh.HandleMessage(func(ctx *th.Context, message telego.Message) error {
		return c.commands.Start(ctx, message)
	}, th.CommandEqual("start"))

	bh.HandleMessage(func(ctx *th.Context, message telego.Message) error {
		return c.commands.Show(ctx, message)
	}, th.CommandEqual("show"))

	bh.HandleMessage(func(ctx *th.Context, message telego.Message) error {
		return c.commands.List(ctx, message)
	}, th.CommandEqual("list"))

	bh.HandleMessage(func(ctx *th.Context, message telego.Message) error {
		return c.handleMessage(ctx, &message)
	}, th.AnyMessage())

	c.setRunning(true)
	logger.InfoCF("telegram", "Telegram bot connected", map[string]any{
		"username": c.bot.Username(),
	})

	go bh.Start()

	go func() {
		<-ctx.Done()
		bh.Stop()
	}()

	return nil
}

func (c *TelegramChannel) Stop(ctx context.Context) error {
	logger.InfoC("telegram", "Stopping Telegram bot...")
	c.setRunning(false)
	return nil
}

// splitLongMessage splits a long message into multiple parts
// Tries to split at reasonable break points (double newline or period + space)
const MAX_TELEGRAM_MESSAGE_LENGTH = 4096

func splitLongMessage(content string) []string {
	if len(content) <= MAX_TELEGRAM_MESSAGE_LENGTH {
		return []string{content}
	}

	var parts []string
	remaining := content

	for len(remaining) > 0 {
		var part string
		if len(remaining) > MAX_TELEGRAM_MESSAGE_LENGTH {
			// Try to find a good break point in the last part of the message
			lookahead := remaining[:MAX_TELEGRAM_MESSAGE_LENGTH]

			// Priority 1: Double newline (paragraph break) - look backwards from the end
			lastDoubleNewline := strings.LastIndex(lookahead, "\n\n")
			if lastDoubleNewline > 0 {
				part = remaining[:lastDoubleNewline]
				remaining = remaining[lastDoubleNewline:]
			} else {
				// Priority 2: Last sentence ending (period + space)
				lastSentenceEnd := strings.LastIndex(lookahead, ". ")
				if lastSentenceEnd > 0 {
					part = remaining[:lastSentenceEnd+1]
					remaining = remaining[lastSentenceEnd+1:]
				} else {
					// Priority 3: Last sentence ending (period)
					lastPeriod := strings.LastIndex(lookahead, ".")
					if lastPeriod > 0 {
						part = remaining[:lastPeriod]
						remaining = remaining[lastPeriod:]
					} else {
						// Fallback: Hard split at limit
						part = remaining[:MAX_TELEGRAM_MESSAGE_LENGTH]
						remaining = remaining[MAX_TELEGRAM_MESSAGE_LENGTH:]
					}
				}
			}
		} else {
			// Remaining content fits in one message
			part = remaining
			remaining = ""
		}

		// Trim whitespace from part and add if non-empty
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}

	return parts
}

func (c *TelegramChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("telegram bot not running")
	}

	chatID, err := parseChatID(msg.ChatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	// Stop thinking animation
	if stop, ok := c.stopThinking.Load(msg.ChatID); ok {
		if cf, ok := stop.(*thinkingCancel); ok && cf != nil {
			cf.Cancel()
		}
		c.stopThinking.Delete(msg.ChatID)
	}

	htmlContent := markdownToTelegramHTML(msg.Content)

	// Split message if exceeds Telegram limit (4096 characters)
	var messageParts []string
	if len(htmlContent) > MAX_TELEGRAM_MESSAGE_LENGTH {
		messageParts = splitLongMessage(htmlContent)
		logger.WarnCF("telegram", "Long message split into parts",
			map[string]any{
				"original_len": len(htmlContent),
				"parts_count":  len(messageParts),
				"chat_id":      msg.ChatID,
			})
	} else {
		messageParts = []string{htmlContent}
	}

	// If thread_id is specified, skip placeholder editing and send directly to thread
	// Placeholder was created in main chat, but response should go to thread
	if msg.ThreadID == "" && len(messageParts) == 1 {
		// Try to edit placeholder (only for messages without thread_id and single part)
		if pID, ok := c.placeholders.Load(msg.ChatID); ok {
			c.placeholders.Delete(msg.ChatID)
			editMsg := tu.EditMessageText(tu.ID(chatID), pID.(int), messageParts[0])
			editMsg.ParseMode = telego.ModeHTML

			if _, err = c.bot.EditMessageText(ctx, editMsg); err == nil {
				return nil
			}
			// Fallback to new message if edit fails
		}
	} else {
		// Clear placeholder if we're sending to thread or multiple messages
		c.placeholders.Delete(msg.ChatID)
	}

	// Send message(s)
	var threadIDInt int
	if msg.ThreadID != "" {
		fmt.Sscanf(msg.ThreadID, "%d", &threadIDInt)
	}

	for i, part := range messageParts {
		tgMsg := tu.Message(tu.ID(chatID), part)
		tgMsg.ParseMode = telego.ModeHTML

		// Add thread ID if specified
		if threadIDInt != 0 {
			tgMsg.MessageThreadID = threadIDInt
		}

		if _, err = c.bot.SendMessage(ctx, tgMsg); err != nil {
			logger.ErrorCF("telegram", "Failed to send message part",
				map[string]any{
					"part":       i + 1,
					"total_parts": len(messageParts),
					"error":      err.Error(),
				})
		}

		// Delay between parts (except last)
		if i < len(messageParts)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	return nil
}

func (c *TelegramChannel) handleMessage(ctx context.Context, message *telego.Message) error {
	if message == nil {
		return fmt.Errorf("message is nil")
	}

	user := message.From
	if user == nil {
		return fmt.Errorf("message sender (user) is nil")
	}

	senderID := fmt.Sprintf("%d", user.ID)
	if user.Username != "" {
		senderID = fmt.Sprintf("%d|%s", user.ID, user.Username)
	}

	// check allowlist to avoid downloading attachments for rejected users
	if !c.IsAllowed(senderID) {
		logger.DebugCF("telegram", "Message rejected by allowlist", map[string]any{
			"user_id": senderID,
		})
		return nil
	}

	chatID := message.Chat.ID
	c.chatIDs[senderID] = chatID

	content := ""
	mediaPaths := []string{}
	localFiles := []string{} // track local files that need cleanup
	workspaceMediaPaths := []string{} // media files copied to workspace (persistent)

	// ensure temp files are cleaned up when function returns
	defer func() {
		for _, file := range localFiles {
			if err := os.Remove(file); err != nil {
				logger.DebugCF("telegram", "Failed to cleanup temp file", map[string]any{
					"file":  file,
					"error": err.Error(),
				})
			}
		}
	}()

	if message.Text != "" {
		content += message.Text
	}

	if message.Caption != "" {
		if content != "" {
			content += "\n"
		}
		content += message.Caption
	}

	if len(message.Photo) > 0 {
		photo := message.Photo[len(message.Photo)-1]
		photoPath := c.downloadPhoto(ctx, photo.FileID)
		if photoPath != "" {
			localFiles = append(localFiles, photoPath)
			
			// Copy to workspace for persistent access by agent
			workspacePhotoPath := c.copyMediaToWorkspace(photoPath, "photo", ".jpg")
			if workspacePhotoPath != "" {
				workspaceMediaPaths = append(workspaceMediaPaths, workspacePhotoPath)
			}
			
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("[image: photo] [file_id: %s]", photo.FileID)
		}
	}

	if message.Voice != nil {
		voicePath := c.downloadFile(ctx, message.Voice.FileID, ".ogg")
		if voicePath != "" {
			localFiles = append(localFiles, voicePath)
			mediaPaths = append(mediaPaths, voicePath)

			var transcribedText string
			if c.transcriber != nil && c.transcriber.IsAvailable() {
				transcriberCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()

				result, err := c.transcriber.Transcribe(transcriberCtx, voicePath)
				if err != nil {
					logger.ErrorCF("telegram", "Voice transcription failed", map[string]any{
						"error": err.Error(),
						"path":  voicePath,
					})
					transcribedText = fmt.Sprintf("[voice (transcription failed)] [file_id: %s]", message.Voice.FileID)
				} else {
					transcribedText = fmt.Sprintf("[voice transcription: %s] [file_id: %s]", result.Text, message.Voice.FileID)
					logger.InfoCF("telegram", "Voice transcribed successfully", map[string]any{
						"text": result.Text,
					})
				}
			} else {
				transcribedText = fmt.Sprintf("[voice] [file_id: %s]", message.Voice.FileID)
			}

			if content != "" {
				content += "\n"
			}
			content += transcribedText
		}
	}

	if message.Audio != nil {
		audioPath := c.downloadFile(ctx, message.Audio.FileID, ".mp3")
		if audioPath != "" {
			localFiles = append(localFiles, audioPath)
			
			// Copy to workspace for persistent access
			workspaceAudioPath := c.copyMediaToWorkspace(audioPath, "audio", ".mp3")
			if workspaceAudioPath != "" {
				workspaceMediaPaths = append(workspaceMediaPaths, workspaceAudioPath)
			}
			
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("[audio] [file_id: %s]", message.Audio.FileID)
		}
	}

	if message.Document != nil {
		docPath := c.downloadFile(ctx, message.Document.FileID, "")
		if docPath != "" {
			localFiles = append(localFiles, docPath)
			
			// Copy to workspace for persistent access
			ext := filepath.Ext(docPath)
			if ext == "" {
				ext = ".bin"
			}
			workspaceDocPath := c.copyMediaToWorkspace(docPath, "document", ext)
			if workspaceDocPath != "" {
				workspaceMediaPaths = append(workspaceMediaPaths, workspaceDocPath)
			}
			
			if content != "" {
				content += "\n"
			}
			// Add file path hint for agent to use read_file tool
			content += fmt.Sprintf("[file: %s] [file_id: %s]", filepath.Base(workspaceDocPath), message.Document.FileID)
		}
	}

	if content == "" {
		content = "[empty message]"
	}

	logger.DebugCF("telegram", "Received message", map[string]any{
		"sender_id": senderID,
		"chat_id":   fmt.Sprintf("%d", chatID),
		"preview":   utils.Truncate(content, 50),
	})

	// Extract thread ID early
	threadID := ""
	threadIDInt := 0
	if message.MessageThreadID != 0 {
		threadID = fmt.Sprintf("%d", message.MessageThreadID)
		threadIDInt = message.MessageThreadID
	}

	// Send typing indicator
	// For threads (thread_id > 1): use SendChatActionParams with MessageThreadID
	// For main chat of forum groups (thread_id=1): use SendChatActionParams with MessageThreadID=1
	// For DM: use simple SendChatAction
	if threadIDInt != 0 {
		// For threads, use SendChatActionParams with MessageThreadID
		params := &telego.SendChatActionParams{
			ChatID:          tu.ID(chatID),
			Action:          telego.ChatActionTyping,
			MessageThreadID: threadIDInt,
		}
		if err := c.bot.SendChatAction(ctx, params); err != nil {
			logger.ErrorCF("telegram", "Failed to send chat action (thread mode)", map[string]any{
				"error": err.Error(),
			})
		}
	} else if message.Chat.Type != "private" {
		// For main chat of forum groups, try with MessageThreadID=1
		params := &telego.SendChatActionParams{
			ChatID:          tu.ID(chatID),
			Action:          telego.ChatActionTyping,
			MessageThreadID: 1, // Special ID for main chat in forum groups
		}
		if err := c.bot.SendChatAction(ctx, params); err != nil {
			logger.ErrorCF("telegram", "Failed to send chat action (forum main chat)", map[string]any{
				"error": err.Error(),
			})
		}
	} else {
		// For DM, use simple SendChatAction
		err := c.bot.SendChatAction(ctx, tu.ChatAction(tu.ID(chatID), telego.ChatActionTyping))
		if err != nil {
			logger.ErrorCF("telegram", "Failed to send chat action (DM)", map[string]any{
				"error": err.Error(),
			})
		}
	}

	// Stop any previous thinking animation
	chatIDStr := fmt.Sprintf("%d", chatID)
	if prevStop, ok := c.stopThinking.Load(chatIDStr); ok {
		if cf, ok := prevStop.(*thinkingCancel); ok && cf != nil {
			cf.Cancel()
		}
	}

	// Create cancel function for thinking state
	_, thinkCancel := context.WithTimeout(ctx, 5*time.Minute)
	c.stopThinking.Store(chatIDStr, &thinkingCancel{fn: thinkCancel})

	// Note: We don't send "Thinking..." placeholder anymore - native typing indicator is sufficient

	peerKind := "direct"
	peerID := fmt.Sprintf("%d", user.ID)
	if message.Chat.Type != "private" {
		peerKind = "group"
		peerID = fmt.Sprintf("%d", chatID)
	}

	metadata := map[string]string{
		"message_id": fmt.Sprintf("%d", message.MessageID),
		"user_id":    fmt.Sprintf("%d", user.ID),
		"username":   user.Username,
		"first_name": user.FirstName,
		"is_group":   fmt.Sprintf("%t", message.Chat.Type != "private"),
		"peer_kind":  peerKind,
		"peer_id":    peerID,
	}

	// Add thread_id to metadata if present
	if threadID != "" {
		metadata["thread_id"] = threadID
	}

	c.HandleMessage(fmt.Sprintf("%d", user.ID), fmt.Sprintf("%d", chatID), content, workspaceMediaPaths, metadata, threadID)
	return nil
}

func (c *TelegramChannel) downloadPhoto(ctx context.Context, fileID string) string {
	file, err := c.bot.GetFile(ctx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		logger.ErrorCF("telegram", "Failed to get photo file", map[string]any{
			"error": err.Error(),
		})
		return ""
	}

	return c.downloadFileWithInfo(file, ".jpg")
}

func (c *TelegramChannel) downloadFileWithInfo(file *telego.File, ext string) string {
	if file.FilePath == "" {
		return ""
	}

	url := c.bot.FileDownloadURL(file.FilePath)
	logger.DebugCF("telegram", "File URL", map[string]any{"url": url})

	// Use FilePath as filename for better identification
	filename := file.FilePath + ext
	return utils.DownloadFile(url, filename, utils.DownloadOptions{
		LoggerPrefix: "telegram",
	})
}

func (c *TelegramChannel) downloadFile(ctx context.Context, fileID, ext string) string {
	file, err := c.bot.GetFile(ctx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		logger.ErrorCF("telegram", "Failed to get file", map[string]any{
			"error": err.Error(),
		})
		return ""
	}

	return c.downloadFileWithInfo(file, ext)
}

// copyMediaToWorkspace copies a media file from temp location to workspace for persistent access.
// Returns the workspace path or empty string if copy failed.
func (c *TelegramChannel) copyMediaToWorkspace(sourcePath, prefix, ext string) string {
	if sourcePath == "" {
		return ""
	}

	// Get workspace path from config
	workspacePath := c.config.WorkspacePath()
	if workspacePath == "" {
		// Fallback to default workspace
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.ErrorCF("telegram", "Failed to get home directory", map[string]any{
				"error": err.Error(),
			})
			return ""
		}
		workspacePath = filepath.Join(homeDir, ".picoclaw", "workspace")
	}

	// Create media directory in workspace
	mediaDir := filepath.Join(workspacePath, "media", "received")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		logger.ErrorCF("telegram", "Failed to create media directory", map[string]any{
			"error": err.Error(),
		})
		return ""
	}

	// Generate unique filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s%s", prefix, timestamp, generateRandomString(6), ext)
	destPath := filepath.Join(mediaDir, filename)

	// Copy file
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		logger.ErrorCF("telegram", "Failed to read source file", map[string]any{
			"error": err.Error(),
			"path":  sourcePath,
		})
		return ""
	}

	if err := os.WriteFile(destPath, sourceData, 0644); err != nil {
		logger.ErrorCF("telegram", "Failed to write destination file", map[string]any{
			"error": err.Error(),
			"path":  destPath,
		})
		return ""
	}

	logger.InfoCF("telegram", "Media copied to workspace", map[string]any{
		"source":      sourcePath,
		"destination": destPath,
	})

	return destPath
}

// generateRandomString generates a random alphanumeric string of given length.
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	if _, err := rand.Read(result); err != nil {
		// Fallback to simple approach if crypto/rand fails
		for i := range result {
			result[i] = charset[i%len(charset)]
		}
		return string(result)
	}
	for i := range result {
		result[i] = charset[int(result[i])%len(charset)]
	}
	return string(result)
}

func parseChatID(chatIDStr string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(chatIDStr, "%d", &id)
	return id, err
}

func markdownToTelegramHTML(text string) string {
	if text == "" {
		return ""
	}

	codeBlocks := extractCodeBlocks(text)
	text = codeBlocks.text

	inlineCodes := extractInlineCodes(text)
	text = inlineCodes.text

	text = regexp.MustCompile(`^#{1,6}\s+(.+)$`).ReplaceAllString(text, "$1")

	text = regexp.MustCompile(`^>\s*(.*)$`).ReplaceAllString(text, "$1")

	text = escapeHTML(text)

	text = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(text, `<a href="$2">$1</a>`)

	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "<b>$1</b>")

	text = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(text, "<b>$1</b>")

	// Italic: manually process to avoid matching identifiers like file_id
	// Only match _word_ where word is alphabetic and surrounded by whitespace/punctuation
	text = processItalics(text)

	text = regexp.MustCompile(`~~(.+?)~~`).ReplaceAllString(text, "<s>$1</s>")

	text = regexp.MustCompile(`^[-*]\s+`).ReplaceAllString(text, "• ")

	for i, code := range inlineCodes.codes {
		escaped := escapeHTML(code)
		text = strings.ReplaceAll(text, fmt.Sprintf("\x00IC%d\x00", i), fmt.Sprintf("<code>%s</code>", escaped))
	}

	for i, code := range codeBlocks.codes {
		escaped := escapeHTML(code)
		text = strings.ReplaceAll(
			text,
			fmt.Sprintf("\x00CB%d\x00", i),
			fmt.Sprintf("<pre><code>%s</code></pre>", escaped),
		)
	}

	return text
}

// processItalics converts _word_ to <i>word</i> but avoids identifiers like file_id
func processItalics(text string) string {
	var result strings.Builder
	i := 0
	for i < len(text) {
		// Look for underscore
		if text[i] == '_' {
			// Check if preceded by whitespace, start, or certain punctuation
			canStart := i == 0 || isItalicBoundary(text[i-1])
			if canStart {
				// Find closing underscore
				j := i + 1
				for j < len(text) && text[j] != '_' && text[j] != ' ' && text[j] != '\n' {
					j++
				}
				// Check if we found a closing underscore and it's followed by boundary
				if j < len(text) && text[j] == '_' {
					content := text[i+1 : j]
					// Only process if content is alphabetic (no underscores/numbers)
					if len(content) > 0 && isAllAlpha(content) {
						// Check if followed by boundary
						nextIdx := j + 1
						if nextIdx >= len(text) || isItalicBoundary(text[nextIdx]) {
							result.WriteString("<i>")
							result.WriteString(content)
							result.WriteString("</i>")
							i = nextIdx
							continue
						}
					}
				}
			}
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

// isItalicBoundary checks if a character is a valid boundary for italic markers
func isItalicBoundary(c byte) bool {
	return c == ' ' || c == '\n' || c == '\t' || c == '<' || c == '>' || c == '.' || c == ',' || c == '!' || c == '?' || c == ';' || c == ':'
}

// isAllAlpha checks if a string contains only alphabetic characters
func isAllAlpha(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

type codeBlockMatch struct {
	text  string
	codes []string
}

func extractCodeBlocks(text string) codeBlockMatch {
	re := regexp.MustCompile("```[\\w]*\\n?([\\s\\S]*?)```")
	matches := re.FindAllStringSubmatch(text, -1)

	codes := make([]string, 0, len(matches))
	for _, match := range matches {
		codes = append(codes, match[1])
	}

	i := 0
	text = re.ReplaceAllStringFunc(text, func(m string) string {
		placeholder := fmt.Sprintf("\x00CB%d\x00", i)
		i++
		return placeholder
	})

	return codeBlockMatch{text: text, codes: codes}
}

type inlineCodeMatch struct {
	text  string
	codes []string
}

func extractInlineCodes(text string) inlineCodeMatch {
	re := regexp.MustCompile("`([^`]+)`")
	matches := re.FindAllStringSubmatch(text, -1)

	codes := make([]string, 0, len(matches))
	for _, match := range matches {
		codes = append(codes, match[1])
	}

	i := 0
	text = re.ReplaceAllStringFunc(text, func(m string) string {
		placeholder := fmt.Sprintf("\x00IC%d\x00", i)
		i++
		return placeholder
	})

	return inlineCodeMatch{text: text, codes: codes}
}

func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// GetBot returns the Telegram bot instance (for tools to use)
func (c *TelegramChannel) GetBot() *telego.Bot {
	return c.bot
}

// GetCurrentChatID returns the most recent chat ID for the current user
// This is used by tools to send messages/files when no explicit chat_id is provided
func (c *TelegramChannel) GetCurrentChatID() string {
	// Get the last known chat ID from the map
	// In a multi-user scenario, this would need more sophisticated tracking
	for _, chatID := range c.chatIDs {
		return fmt.Sprintf("%d", chatID)
	}
	return ""
}

// GetChatIDForUser returns the chat ID for a specific user ID
func (c *TelegramChannel) GetChatIDForUser(userID string) string {
	if chatID, ok := c.chatIDs[userID]; ok {
		return fmt.Sprintf("%d", chatID)
	}
	return ""
}
