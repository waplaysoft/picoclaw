// PicoClaw WebUI Client
class PicoClawWebUI {
    constructor() {
        this.session = null;
        this.isStreaming = false;
        this.messagesContainer = document.getElementById('messages');
        this.messageInput = document.getElementById('messageInput');
        this.sendBtn = document.getElementById('sendBtn');
        this.attachBtn = document.getElementById('attachBtn');
        this.statusEl = document.getElementById('status');
        this.sessionInfoEl = document.getElementById('sessionInfo');
        this.init();
    }

    init() {
        this.bindEvents();
        this.checkStatus();
        this.autoResizeTextarea();
        this.initMarkdown();
        this.addCopyButtons();
    }

    initMarkdown() {
        // Configure marked for better rendering
        marked.setOptions({
            breaks: true,
            gfm: true,
        });
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

        // Placeholder for attach button
        this.attachBtn.addEventListener('click', () => {
            console.log('Attach functionality coming soon');
        });

        // Copy code button event delegation
        this.messagesContainer.addEventListener('click', (e) => {
            if (e.target.classList.contains('copy-btn') || e.target.closest('.copy-btn')) {
                const btn = e.target.classList.contains('copy-btn') ? e.target : e.target.closest('.copy-btn');
                this.copyCode(btn);
            }
        });
    }

    addCopyButtons() {
        // Add copy buttons to existing code blocks
        this.messagesContainer.querySelectorAll('pre').forEach((pre) => {
            if (!pre.querySelector('.copy-btn')) {
                this.createCopyButton(pre);
            }
        });
    }

    createCopyButton(pre) {
        const btn = document.createElement('button');
        btn.className = 'copy-btn';
        btn.innerHTML = `
            <svg class="copy-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            <span class="copy-text">Copy</span>
        `;
        pre.appendChild(btn);
    }

    async copyCode(btn) {
        const pre = btn.closest('pre');
        const code = pre.querySelector('code');
        const text = code ? code.textContent : pre.textContent;

        try {
            await navigator.clipboard.writeText(text);
            btn.classList.add('copied');
            btn.querySelector('.copy-text').textContent = 'Copied!';
            setTimeout(() => {
                btn.classList.remove('copied');
                btn.querySelector('.copy-text').textContent = 'Copy';
            }, 2000);
        } catch (err) {
            console.error('Failed to copy:', err);
        }
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
                                assistantMessageEl.querySelector('.message-content').innerHTML = marked.parse(content);
                                this.addCopyButtons();
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

        } catch (error) {
            if (typingEl && typingEl.parentNode) {
                typingEl.remove();
            }
            this.addMessage('Error: ' + error.message, 'assistant', true);
        } finally {
            this.isStreaming = false;
            this.sendBtn.disabled = false;
        }
    }

    addMessage(content, type, isError = false) {
        const messageEl = document.createElement('div');
        messageEl.className = `message ${type}${isError ? ' error' : ''}`;

        // Escape HTML for user messages, render markdown for assistant
        const contentHtml = type === 'assistant' && !isError
            ? marked.parse(content)
            : this.escapeHtml(content);

        messageEl.innerHTML = `
            <div class="message-wrapper">
                <div class="message-content">${contentHtml}</div>
            </div>
        `;

        this.messagesContainer.appendChild(messageEl);
        this.scrollToBottom();
        return messageEl;
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    showTypingIndicator() {
        const typingEl = document.createElement('div');
        typingEl.className = 'message assistant typing-message';
        
        typingEl.innerHTML = `
            <div class="message-wrapper">
                <div class="message-content">
                    <div class="typing">
                        <div class="typing-dot"></div>
                        <div class="typing-dot"></div>
                        <div class="typing-dot"></div>
                    </div>
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
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.picoClaw = new PicoClawWebUI();
});