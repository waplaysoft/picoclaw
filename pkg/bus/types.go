package bus

type InboundMessage struct {
	Channel    string            `json:"channel"`
	SenderID   string            `json:"sender_id"`
	ChatID     string            `json:"chat_id"`
	ThreadID   string            `json:"thread_id,omitempty"`
	Content    string            `json:"content"`
	Media      []string          `json:"media,omitempty"`      // Image paths for vision
	Files      []string          `json:"files,omitempty"`      // File paths for read_file tool
	SessionKey string            `json:"session_key"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type OutboundMessage struct {
	Channel  string `json:"channel"`
	ChatID   string `json:"chat_id"`
	ThreadID string `json:"thread_id,omitempty"`
	Content  string `json:"content"`
	Files    []string `json:"files,omitempty"`    // File paths for download
}

type MessageHandler func(InboundMessage) error
