// HighClaw Web UI - Vue 3 Application
const { createApp } = Vue;

createApp({
    data() {
        return {
            currentView: 'chat',
            currentSession: 'main',
            loading: false,
            status: null,
            models: [],
            providers: [],
            channels: [],
            config: null,
            messages: [],
            chatInput: '',
            sending: false,
            ws: null,
            sessions: [],
            skills: [],
            skillsSummary: null,
            nodes: [],
            cronJobs: [],
            logs: [],
            runtimeStats: null,
        };
    },

    computed: {
        isOnline() {
            return this.status && this.status.gateway && this.status.gateway.status === 'running';
        },

        modelCount() {
            return this.models.length;
        },

        providerCount() {
            return this.providers.length;
        },

        channelCount() {
            return this.channels.filter(c => c.status === 'configured' || c.status === 'connected').length;
        },
    },

    methods: {
        async loadSessionMessages(sessionKey) {
            try {
                const response = await axios.get(`/api/sessions/${sessionKey}`);
                if (response.data && response.data.messages) {
                    this.messages = response.data.messages.map(msg => ({
                        role: msg.role,
                        content: msg.content,
                        timestamp: msg.timestamp || new Date().toISOString(),
                    }));
                }
            } catch (error) {
                // Session may not exist yet, that's ok
                console.debug('No existing session messages:', error.message);
            }
        },

        async fetchStatus() {
            try {
                const response = await axios.get('/api/status');
                this.status = response.data;
            } catch (error) {
                console.error('Failed to fetch status:', error);
            }
        },

        async fetchModels() {
            try {
                const response = await axios.get('/api/models');
                this.models = response.data.models || [];
            } catch (error) {
                console.error('Failed to fetch models:', error);
            }
        },

        async fetchProviders() {
            try {
                const response = await axios.get('/api/providers');
                this.providers = response.data.providers || [];
            } catch (error) {
                console.error('Failed to fetch providers:', error);
            }
        },

        async fetchChannels() {
            try {
                const response = await axios.get('/api/channels/status');
                this.channels = response.data.channels || [];
            } catch (error) {
                console.error('Failed to fetch channels:', error);
            }
        },

        async fetchConfig() {
            try {
                const response = await axios.get('/api/config');
                this.config = response.data;
            } catch (error) {
                console.error('Failed to fetch config:', error);
            }
        },

        async fetchSessions() {
            try {
                const response = await axios.get('/api/sessions');
                this.sessions = response.data.sessions || [];
            } catch (error) {
                console.error('Failed to fetch sessions:', error);
            }
        },

        async fetchSkills() {
            try {
                const response = await axios.get('/api/skills');
                this.skills = response.data.skills || [];
                this.skillsSummary = response.data.summary || {};
            } catch (error) {
                console.error('Failed to fetch skills:', error);
            }
        },

        async fetchRuntimeStats() {
            try {
                const response = await axios.get('/api/runtime/stats');
                this.runtimeStats = response.data;
            } catch (error) {
                console.error('Failed to fetch runtime stats:', error);
            }
        },

        async fetchLogs() {
            try {
                const response = await axios.get('/api/logs');
                this.logs = response.data.logs || [];
            } catch (error) {
                console.error('Failed to fetch logs:', error);
            }
        },

        async deleteSession(key) {
            try {
                await axios.delete(`/api/sessions/${key}`);
                this.fetchSessions();
            } catch (error) {
                console.error('Failed to delete session:', error);
            }
        },

        async sendMessage() {
            if (!this.chatInput.trim() || this.sending) return;

            const userMessage = {
                role: 'user',
                content: this.chatInput,
                timestamp: new Date().toISOString(),
            };

            this.messages.push(userMessage);
            this.sending = true;
            const input = this.chatInput;
            this.chatInput = '';

            try {
                const response = await axios.post('/api/chat', {
                    message: input,
                    session: this.currentSession,
                });

                const assistantMessage = {
                    role: 'assistant',
                    content: response.data.response || 'No response',
                    timestamp: new Date().toISOString(),
                };

                this.messages.push(assistantMessage);
            } catch (error) {
                console.error('Failed to send message:', error);
                this.messages.push({
                    role: 'assistant',
                    content: 'Error: Failed to get response. ' + (error.response?.data?.error || error.message),
                    timestamp: new Date().toISOString(),
                });
            } finally {
                this.sending = false;
                this.scrollToBottom();
            }
        },

        handleChatKeydown(event) {
            if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault();
                this.sendMessage();
            }
        },

        scrollToBottom() {
            this.$nextTick(() => {
                const container = this.$refs.chatMessages;
                if (container) {
                    container.scrollTop = container.scrollHeight;
                }
            });
        },

        renderMarkdown(text) {
            if (!text) return '';
            if (typeof marked !== 'undefined' && marked.parse) {
                return marked.parse(text);
            }
            // Fallback: basic formatting
            return text
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/`([^`]+)`/g, '<code>$1</code>')
                .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
                .replace(/\n/g, '<br>');
        },

        skillStatusBadge(status) {
            switch (status) {
                case 'eligible': return 'badge-success';
                case 'missing_deps': return 'badge-warning';
                case 'missing_api_key': return 'badge-warning';
                case 'blocked_allowlist': return 'badge-error';
                case 'disabled': return 'badge-error';
                default: return 'badge-warning';
            }
        },

        skillStatusText(status) {
            switch (status) {
                case 'eligible': return 'Ready';
                case 'missing_deps': return 'Missing Deps';
                case 'missing_api_key': return 'Missing API Key';
                case 'blocked_allowlist': return 'Blocked';
                case 'disabled': return 'Disabled';
                default: return status;
            }
        },

        logLevelBadge(level) {
            switch (level) {
                case 'DEBUG': return 'badge-info';
                case 'INFO': return 'badge-success';
                case 'WARN': return 'badge-warning';
                case 'ERROR': return 'badge-error';
                default: return 'badge-success';
            }
        },

        connectWebSocket() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws`;

            this.ws = new WebSocket(wsUrl);

            this.ws.onopen = () => {
                console.log('WebSocket connected');
            };

            this.ws.onmessage = (event) => {
                const data = JSON.parse(event.data);
                console.log('WebSocket message:', data);

                if (data.type === 'event' && data.method === 'chat.typing') {
                    // Could show typing indicator
                } else if (data.type === 'status_update') {
                    this.fetchStatus();
                } else if (data.type === 'chat_message') {
                    this.messages.push(data.message);
                }
            };

            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
            };

            this.ws.onclose = () => {
                console.log('WebSocket disconnected');
                setTimeout(() => this.connectWebSocket(), 5000);
            };
        },

        formatTimestamp(timestamp) {
            if (!timestamp) return '';
            return new Date(timestamp).toLocaleString();
        },

        formatMemory(mb) {
            if (!mb && mb !== 0) return 'N/A';
            if (mb < 1) return (mb * 1024).toFixed(0) + ' KB';
            return mb.toFixed(1) + ' MB';
        },
    },

    mounted() {
        // Parse URL parameters
        const urlParams = new URLSearchParams(window.location.search);
        const session = urlParams.get('session');
        const view = urlParams.get('view');

        if (session) {
            this.currentSession = session;
        }

        if (view) {
            this.currentView = view;
        } else if (window.location.pathname.includes('/chat')) {
            this.currentView = 'chat';
        }

        // Load session messages
        if (this.currentSession) {
            this.loadSessionMessages(this.currentSession);
        }

        // Initial data fetch
        this.fetchStatus();
        this.fetchModels();
        this.fetchProviders();
        this.fetchChannels();
        this.fetchConfig();
        this.fetchSessions();
        this.fetchSkills();
        this.fetchRuntimeStats();
        this.fetchLogs();

        // Connect WebSocket
        this.connectWebSocket();

        // Refresh intervals
        setInterval(() => this.fetchStatus(), 10000);
        setInterval(() => this.fetchRuntimeStats(), 15000);
        setInterval(() => this.fetchLogs(), 10000);
        setInterval(() => this.fetchSessions(), 15000);

        // Update URL when view changes
        this.$watch('currentView', (newView) => {
            const url = new URL(window.location);
            url.searchParams.set('view', newView);
            if (this.currentSession && newView === 'chat') {
                url.searchParams.set('session', this.currentSession);
            }
            window.history.pushState({}, '', url);
        });
    },
}).mount('#app');
