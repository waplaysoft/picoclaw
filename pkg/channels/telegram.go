package channels

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
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
	bot         *telego.Bot
	commands    TelegramCommander
	config      *config.Config
	chatIDs     map[string]int64
	transcriber *voice.GroqTranscriber
	typingCtx   sync.Map // chatID -> context.CancelFunc
}

// StartTyping starts a continuous typing indicator loop.
// Returns a stop function that can be called to cancel the typing indicator.
// The stop function is idempotent - safe to call multiple times.
func (c *TelegramChannel) StartTyping(ctx context.Context, chatID string, threadID int) (func(), error) {
	cid, err := parseChatID(chatID)
	if err != nil {
		return func() {}, err
	}

	// Stop any existing typing loop for this chat
	c.StopTyping(chatID)

	// Create context for this typing loop
	typingCtx, cancel := context.WithCancel(ctx)

	// Store cancel function
	c.typingCtx.Store(chatID, cancel)

	// Send first typing action immediately
	c.sendTypingAction(ctx, cid, threadID)

	// Start goroutine to repeat typing indicator every 4 seconds
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				c.sendTypingAction(typingCtx, cid, threadID)
			}
		}
	}()

	// Return stop function
	return func() {
		c.StopTyping(chatID)
	}, nil
}

// StopTyping stops the typing indicator for a specific chat.
// Safe to call multiple times (idempotent).
func (c *TelegramChannel) StopTyping(chatID string) {
	if cancel, ok := c.typingCtx.Load(chatID); ok {
		if cf, ok := cancel.(context.CancelFunc); ok && cf != nil {
			cf()
		}
		c.typingCtx.Delete(chatID)
	}
}

// sendTypingAction sends a single typing indicator
func (c *TelegramChannel) sendTypingAction(ctx context.Context, chatID int64, threadID int) {
	if threadID != 0 {
		params := &telego.SendChatActionParams{
			ChatID:          tu.ID(chatID),
			Action:          telego.ChatActionTyping,
			MessageThreadID: threadID,
		}
		if err := c.bot.SendChatAction(ctx, params); err != nil {
			logger.DebugCF("telegram", "Failed to send typing indicator (thread mode)", map[string]any{
				"error": err.Error(),
			})
		}
	} else {
		err := c.bot.SendChatAction(ctx, tu.ChatAction(tu.ID(chatID), telego.ChatActionTyping))
		if err != nil {
			logger.DebugCF("telegram", "Failed to send typing indicator (DM)", map[string]any{
				"error": err.Error(),
			})
		}
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
		BaseChannel: base,
		commands:    NewTelegramCommands(bot, cfg),
		bot:         bot,
		config:      cfg,
		chatIDs:     make(map[string]int64),
		transcriber: nil,
		typingCtx:   sync.Map{},
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

	// Stop typing indicator
	c.StopTyping(msg.ChatID)

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
					"part":        i + 1,
					"total_parts": len(messageParts),
					"error":       err.Error(),
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
			mediaPaths = append(mediaPaths, photoPath)
			if content != "" {
				content += "\n"
			}
			content += "[image: photo]"
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
					transcribedText = "[voice (transcription failed)]"
				} else {
					transcribedText = fmt.Sprintf("[voice transcription: %s]", result.Text)
					logger.InfoCF("telegram", "Voice transcribed successfully", map[string]any{
						"text": result.Text,
					})
				}
			} else {
				transcribedText = "[voice]"
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
			mediaPaths = append(mediaPaths, audioPath)
			if content != "" {
				content += "\n"
			}
			content += "[audio]"
		}
	}

	if message.Document != nil {
		docPath := c.downloadFile(ctx, message.Document.FileID, "")
		if docPath != "" {
			localFiles = append(localFiles, docPath)
			mediaPaths = append(mediaPaths, docPath)
			if content != "" {
				content += "\n"
			}
			content += "[file]"
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

	// Start typing indicator (stops automatically when response is sent)
	chatIDStr := fmt.Sprintf("%d", chatID)
	_, _ = c.StartTyping(ctx, chatIDStr, threadIDInt)

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

	c.HandleMessage(fmt.Sprintf("%d", user.ID), fmt.Sprintf("%d", chatID), content, mediaPaths, metadata, threadID)
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

	reItalic := regexp.MustCompile(`_([^_]+)_`)
	text = reItalic.ReplaceAllStringFunc(text, func(s string) string {
		match := reItalic.FindStringSubmatch(s)
		if len(match) < 2 {
			return s
		}
		return "<i>" + match[1] + "</i>"
	})

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
