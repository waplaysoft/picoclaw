// PicoClaw WebUI Client

// Simple Markdown parser
function parseMarkdown(text) {
    if (!text) return '';
    
    let html = text;
    
    // Escape HTML first
    html = html
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
    
    // Code blocks (``` ... ```)
    html = html.replace(/```(\w*)\n([\s\S]*?)```/g, function(match, lang, code) {
        return '<pre><code class="language-' + lang + '">' + code.trim() + '</code></pre>';
    });
    
    // Inline code (` ... `)
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
    
    // Bold (** ... ** or __ ... __)
    html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    html = html.replace(/__([^_]+)__/g, '<strong>$1</strong>');
    
    // Italic (* ... * or _ ... _)
    html = html.replace(/\*([^*]+)\*/g, '<em>$1</em>');
    html = html.replace(/_([^_]+)_/g, '<em>$1</em>');
    
    // Links [text](url)
    html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
    
    // Headers (# ... ######)
    html = html.replace(/^######\s+(.+)$/gm, '<h6>$1</h6>');
    html = html.replace(/^#####\s+(.+)$/gm, '<h5>$1</h5>');
    html = html.replace(/^####\s+(.+)$/gm, '<h4>$1</h4>');
    html = html.replace(/^###\s+(.+)$/gm, '<h3>$1</h3>');
    html = html.replace(/^##\s+(.+)$/gm, '<h2>$1</h2>');
    html = html.replace(/^#\s+(.+)$/gm, '<h1>$1</h1>');
    
    // Blockquotes (> ...)
    html = html.replace(/^&gt;\s*(.+)$/gm, '<blockquote>$1</blockquote>');
    
    // Horizontal rule (--- or ***)
    html = html.replace(/^([-*_]){3,}$/gm, '<hr>');
    
    // Unordered lists (- or * at start of line)
    html = html.replace(/^[\-\*]\s+(.+)$/gm, '<li>$1</li>');
    // Wrap consecutive li elements in ul
    html = html.replace(/(<li>.*<\/li>\n?)+/g, function(match) {
        return '<ul>' + match + '</ul>';
    });
    
    // Ordered lists (1. 2. 3. at start of line)
    html = html.replace(/^\d+\.\s+(.+)$/gm, '<oli>$1</oli>');
    // Wrap consecutive oli elements in ol
    html = html.replace(/(<oli>.*<\/oli>\n?)+/g, function(match) {
        return '<ol>' + match.replace(/<\/?oli>/g, function(tag) {
            return tag.replace('oli', 'li');
        }) + '</ol>';
    });
    
    // Line breaks (convert newlines to <br> for non-block elements)
    // Split by double newline for paragraphs
    const blocks = html.split(/\n\n+/);
    html = blocks.map(block => {
        block = block.trim();
        if (!block) return '';
        
        // Don't wrap if it's already a block element
        if (/^<(h[1-6]|ul|ol|li|pre|blockquote|hr|div|p)/.test(block)) {
            return block;
        }
        
        // Convert single newlines to <br> within the block
        block = block.replace(/\n/g, '<br>');
        return '<p>' + block + '</p>';
    }).join('\n');
    
    return html;
}

class PicoClawWebUI {

    constructor() {
        this.session = localStorage.getItem('picoclaw_session') || null;
        this.isStreaming = false;
        this.messagesContainer = document.getElementById('messages');
        this.messageInput = document.getElementById('messageInput');
        this.sendBtn = document.getElementById('sendBtn');
        this.attachBtn = document.getElementById('attachBtn');
        this.statusEl = document.getElementById('status');
        this.sessionInfoEl = document.getElementById('sessionInfo');
        this.sessionSelectEl = document.getElementById('sessionSelect');
        this.loadSessionBtn = document.getElementById('loadSessionBtn');

        // Pagination state
        this.pagination = {
            offset: 0,
            limit: 50,
            totalCount: 0,
            hasMore: false,
            isLoading: false
        };

        // Messages cache
        this.messages = [];

        this.init();
    }

    async init() {
        this.bindEvents();
        this.checkStatus();
        this.autoResizeTextarea();
        this.addCopyButtons();

        // Load sessions list and restore session
        await this.loadSessionsList();
        await this.restoreSession();
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
            const codeBtn = e.target.closest('.copy-btn');
            if (codeBtn) {
                this.copyCode(codeBtn);
                return;
            }
            // Copy message button event delegation
            const messageBtn = e.target.closest('.message-copy-btn');
            if (messageBtn) {
                this.copyMessage(messageBtn);
            }
        });

        // Session selector change
        if (this.sessionSelectEl) {
            this.sessionSelectEl.addEventListener('change', (e) => {
                this.switchSession(e.target.value);
            });
        }

        // Load session button
        if (this.loadSessionBtn) {
            this.loadSessionBtn.addEventListener('click', () => {
                this.restoreSession();
            });
        }

        // Scroll handler for lazy pagination
        this.messagesContainer.addEventListener('scroll', () => {
            this.handleScroll();
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

    async copyMessage(btn) {
        const messageEl = btn.closest('.message');
        const markdown = messageEl.dataset.markdown;

        if (!markdown) {
            console.error('No markdown content found');
            btn.querySelector('.copy-text').textContent = 'Error';
            return;
        }

        try {
            await navigator.clipboard.writeText(markdown);
            btn.classList.add('copied');
            btn.querySelector('.copy-text').textContent = 'Copied!';
            setTimeout(() => {
                btn.classList.remove('copied');
                btn.querySelector('.copy-text').textContent = 'Copy';
            }, 2000);
        } catch (err) {
            console.error('Failed to copy:', err);
            btn.querySelector('.copy-text').textContent = 'Error';
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

    // Session management
    async loadSessionsList() {
        if (!this.sessionSelectEl) return;

        try {
            const response = await fetch('/api/sessions');
            if (!response.ok) return;

            const data = await response.json();
            this.sessionSelectEl.innerHTML = '<option value="">New Session</option>';

            data.sessions.forEach(session => {
                const option = document.createElement('option');
                option.value = session.key;
                const preview = session.preview ? ` - ${session.preview}` : '';
                option.textContent = `${session.key.slice(-12)} (${session.message_count})${preview}`;
                if (session.key === this.session) {
                    option.selected = true;
                }
                this.sessionSelectEl.appendChild(option);
            });
        } catch (error) {
            console.error('Failed to load sessions:', error);
        }
    }

    async restoreSession() {
        if (!this.session) {
            this.updateSessionInfo();
            return;
        }

        // Reset pagination
        this.pagination = {
            offset: 0,
            limit: 50,
            totalCount: 0,
            hasMore: false,
            isLoading: false
        };
        this.messages = [];
        this.messagesContainer.innerHTML = '';

        try {
            const response = await fetch(`/api/history?session=${encodeURIComponent(this.session)}&limit=${this.pagination.limit}&offset=${this.pagination.offset}`);
            if (!response.ok) {
                console.error('Failed to load history');
                return;
            }

            const data = await response.json();
            this.pagination.totalCount = data.total_count;
            this.pagination.hasMore = data.has_more;
            this.messages = data.messages;

            // Render messages
            data.messages.forEach(msg => {
                this.addMessageToContainer(msg.content, msg.role, false);
            });

            this.updateSessionInfo();

            // Scroll to bottom only if there are messages and we're not paginating
            if (data.messages.length > 0) {
                // Use requestAnimationFrame to ensure DOM is updated
                requestAnimationFrame(() => {
                    this.messagesContainer.scrollTop = this.messagesContainer.scrollHeight;
                });
            }
        } catch (error) {
            console.error('Failed to restore session:', error);
        }
    }

    switchSession(sessionKey) {
        this.session = sessionKey || null;
        if (sessionKey) {
            localStorage.setItem('picoclaw_session', sessionKey);
        } else {
            localStorage.removeItem('picoclaw_session');
        }
        this.restoreSession();
    }

    updateSessionInfo() {
        if (this.session) {
            this.sessionInfoEl.textContent = `Session: ${this.session.slice(-8)}`;
        } else {
            this.sessionInfoEl.textContent = 'New Session';
        }
    }

    handleScroll() {
        // Check if scrolled to top
        if (this.messagesContainer.scrollTop === 0 &&
            this.pagination.hasMore &&
            !this.pagination.isLoading) {
            this.loadOlderMessages();
        }
    }

    async loadOlderMessages() {
        this.pagination.isLoading = true;

        // Remember current scroll position
        const previousScrollTop = this.messagesContainer.scrollTop;
        const previousScrollHeight = this.messagesContainer.scrollHeight;

        // Show loading indicator
        const loadingEl = document.createElement('div');
        loadingEl.className = 'loading-indicator';
        loadingEl.textContent = 'Loading older messages...';
        this.messagesContainer.insertBefore(loadingEl, this.messagesContainer.firstChild);

        try {
            const offset = this.pagination.offset + this.pagination.limit;
            const response = await fetch(`/api/history?session=${encodeURIComponent(this.session)}&limit=${this.pagination.limit}&offset=${offset}`);
            if (!response.ok) {
                throw new Error('Failed to load older messages');
            }

            const data = await response.json();

            // Remove loading indicator
            loadingEl.remove();

            if (data.messages.length > 0) {
                // Prepend messages in reverse order (oldest first)
                const fragment = document.createDocumentFragment();
                data.messages.reverse().forEach(msg => {
                    const el = this.createMessageElement(msg.content, msg.role);
                    fragment.appendChild(el);
                });

                // Insert at beginning
                this.messagesContainer.insertBefore(fragment, this.messagesContainer.firstChild);

                // Maintain scroll position by adjusting for new content height
                const newScrollHeight = this.messagesContainer.scrollHeight;
                const heightDifference = newScrollHeight - previousScrollHeight;
                this.messagesContainer.scrollTop = previousScrollTop + heightDifference;
            }

            this.pagination.offset = offset;
            this.pagination.hasMore = data.has_more;
        } catch (error) {
            console.error('Error loading older messages:', error);
            loadingEl.remove();
        } finally {
            this.pagination.isLoading = false;
        }
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
                                localStorage.setItem('picoclaw_session', this.session);
                                this.updateSessionInfo();
                                await this.loadSessionsList();
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
                                assistantMessageEl.querySelector('.message-content').innerHTML = parseMarkdown(content);
                                assistantMessageEl.dataset.markdown = content;
                                this.addCopyButtons();
                                this.scrollToBottom();
                            }

                            // Check if done
                            if (data.done) {
                                this.isStreaming = false;
                                this.sendBtn.disabled = false;
                                // Reload sessions list to update preview
                                await this.loadSessionsList();
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
        const messageEl = this.createMessageElement(content, type, isError);
        this.messagesContainer.appendChild(messageEl);
        this.scrollToBottom();
        return messageEl;
    }

    createMessageElement(content, type, isError = false) {
        const messageEl = document.createElement('div');
        messageEl.className = `message ${type}${isError ? ' error' : ''}`;

        messageEl.innerHTML = `
            <div class="message-wrapper">
                <div class="message-content"></div>
            </div>
        `;

        const contentEl = messageEl.querySelector('.message-content');
        
        // Parse markdown for assistant messages, plain text for user
        if (type === 'assistant' && !isError) {
            contentEl.innerHTML = parseMarkdown(content);
            messageEl.dataset.markdown = content;
            this.createMessageCopyButton(messageEl);
        } else {
            // Escape HTML and convert newlines to <br> for user messages
            contentEl.textContent = content;
        }

        return messageEl;
    }

    addMessageToContainer(content, type, isError = false) {
        const messageEl = this.createMessageElement(content, type, isError);
        this.messagesContainer.appendChild(messageEl);
    }

    createMessageCopyButton(messageEl) {
        const wrapper = messageEl.querySelector('.message-wrapper');
        const btn = document.createElement('button');
        btn.className = 'message-copy-btn';
        btn.innerHTML = `
            <svg class="copy-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            <span class="copy-text">Copy</span>
        `;
        wrapper.appendChild(btn);
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
