// PicoClaw WebUI Client

class PicoClawWebUI {
    constructor() {
        this.session = null;
        this.isStreaming = false;
        this.messagesContainer = document.getElementById('messages');
        this.messageInput = document.getElementById('messageInput');
        this.sendBtn = document.getElementById('sendBtn');
        this.statusEl = document.getElementById('status');
        this.sessionInfoEl = document.getElementById('sessionInfo');

        this.init();
    }

    init() {
        this.bindEvents();
        this.checkStatus();
        this.autoResizeTextarea();
    }

    bindEvents() {
        this.sendBtn.addEventListener('click', () => this.sendMessage());
        this.messageInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.sendMessage();
            }
        });
        this.messageInput.addEventListener('input', () => this.autoResizeTextarea());
    }

    autoResizeTextarea() {
        this.messageInput.style.height = 'auto';
        this.messageInput.style.height = Math.min(this.messageInput.scrollHeight, 150) + 'px';
    }

    async checkStatus() {
        try {
            const response = await fetch('/api/ready');
            if (response.ok) {
                this.setStatus('connected', 'Connected');
            } else {
                this.setStatus('error', 'Disconnected');
            }
        } catch (error) {
            this.setStatus('error', 'Connection failed');
        }
    }

    setStatus(status, text) {
        this.statusEl.className = 'status ' + status;
        this.statusEl.querySelector('.status-text').textContent = text;
    }

    async sendMessage() {
        const message = this.messageInput.value.trim();
        if (!message || this.isStreaming) return;

        // Add user message to chat
        this.addMessage(message, 'user');
        this.messageInput.value = '';
        this.autoResizeTextarea();
        this.isStreaming = true;
        this.sendBtn.disabled = true;

        // Show typing indicator
        const typingEl = this.showTypingIndicator();

        try {
            const response = await fetch('/api/chat', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    message: message,
                    session: this.session || '',
                    stream: true,
                }),
            });

            if (!response.ok) {
                throw new Error('Request failed');
            }

            // Remove typing indicator
            typingEl.remove();

            // Handle SSE stream
            const reader = response.body.getReader();
            const decoder = new TextDecoder();
            let assistantMessageEl = null;
            let content = '';

            while (true) {
                const { done, value } = await reader.read();
                if (done) break;

                const chunk = decoder.decode(value);
                const lines = chunk.split('\n');

                for (const line of lines) {
                    if (line.startsWith('data: ')) {
                        try {
                            const data = JSON.parse(line.slice(6));

                            // Store session from first message
                            if (data.session && !this.session) {
                                this.session = data.session;
                                this.sessionInfoEl.textContent = `Session: ${this.session.slice(-8)}`;
                            }

                            // Handle error
                            if (data.error) {
                                this.addMessage('Error: ' + data.error, 'assistant', true);
                                break;
                            }

                            // Handle content
                            if (data.content) {
                                if (!assistantMessageEl) {
                                    assistantMessageEl = this.addMessage('', 'assistant');
                                }
                                content += data.content;
                                assistantMessageEl.querySelector('.message-content').textContent = content;
                                this.scrollToBottom();
                            }

                            // Check if done
                            if (data.done) {
                                this.isStreaming = false;
                                this.sendBtn.disabled = false;
                            }
                        } catch (e) {
                            console.error('Parse error:', e);
                        }
                    }
                }
            }

            if (!this.isStreaming) {
                this.isStreaming = false;
                this.sendBtn.disabled = false;
            }

        } catch (error) {
            typingEl.remove();
            this.addMessage('Error: ' + error.message, 'assistant', true);
            this.isStreaming = false;
            this.sendBtn.disabled = false;
        }
    }

    addMessage(content, type, isError = false) {
        const messageEl = document.createElement('div');
        messageEl.className = `message ${type}${isError ? ' error' : ''}`;
        messageEl.innerHTML = `
            <div class="message-content">${this.escapeHtml(content)}</div>
        `;
        this.messagesContainer.appendChild(messageEl);
        this.scrollToBottom();
        return messageEl;
    }

    showTypingIndicator() {
        const typingEl = document.createElement('div');
        typingEl.className = 'message assistant typing-message';
        typingEl.innerHTML = `
            <div class="message-content">
                <div class="typing">
                    <div class="typing-dot"></div>
                    <div class="typing-dot"></div>
                    <div class="typing-dot"></div>
                </div>
            </div>
        `;
        this.messagesContainer.appendChild(typingEl);
        this.scrollToBottom();
        return typingEl;
    }

    scrollToBottom() {
        this.messagesContainer.scrollTop = this.messagesContainer.scrollHeight;
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.picoClaw = new PicoClawWebUI();
});
