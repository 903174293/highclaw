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
            nodes: [],
            cronJobs: [],
            logs: [],
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
            return this.channels.filter(c => c.status === 'connected').length;
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
                console.error('Failed to load session messages:', error);
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
                    content: 'Error: Failed to get response',
                    timestamp: new Date().toISOString(),
                });
            } finally {
                this.sending = false;
                this.$nextTick(() => {
                    const container = this.$refs.chatMessages;
                    if (container) {
                        container.scrollTop = container.scrollHeight;
                    }
                });
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
                
                // Handle different message types
                if (data.type === 'status_update') {
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
                // Reconnect after 5 seconds
                setTimeout(() => this.connectWebSocket(), 5000);
            };
        },
        
        formatTimestamp(timestamp) {
            return new Date(timestamp).toLocaleTimeString();
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

        // Connect WebSocket
        this.connectWebSocket();

        // Refresh status every 10 seconds
        setInterval(() => {
            this.fetchStatus();
        }, 10000);

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
