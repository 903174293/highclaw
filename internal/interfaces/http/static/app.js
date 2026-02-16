// HighClaw Control Dashboard

let currentTab = 'chat';
let currentSession = 'main';
let agentModel = '';
let uptimeInterval;

const api = {
    async get(endpoint) {
        try {
            const res = await fetch(`/api${endpoint}`);
            if (!res.ok) throw new Error(`API Error: ${res.status}`);
            return await res.json();
        } catch (e) {
            console.error(e);
            return null;
        }
    },
    async post(endpoint, data) {
        try {
            const res = await fetch(`/api${endpoint}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
            if (!res.ok) throw new Error(`API Error: ${res.status}`);
            return await res.json();
        } catch (e) {
            console.error(e);
            return null;
        }
    }
};

document.addEventListener('DOMContentLoaded', async () => {
    initTabs();
    initChat();
    initStatus();
    loadConfig();
    
    // Auto refresh status
    setInterval(updateStatus, 5000);
    updateStatus();
});

function initTabs() {
    const tabs = document.querySelectorAll('.nav-item');
    tabs.forEach(tab => {
        tab.addEventListener('click', (e) => {
            e.preventDefault();
            const target = tab.dataset.tab;
            switchTab(target);
        });
    });
}

function switchTab(tabId) {
    // Nav active state
    document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
    document.querySelector(`.nav-item[data-tab="${tabId}"]`).classList.add('active');

    // View visibility
    document.querySelectorAll('.view').forEach(el => el.classList.remove('active'));
    
    // Special handling per tab loading
    if (tabId === 'sessions') loadSessions();
    if (tabId === 'channels') loadChannels();
    if (tabId === 'skills') loadSkills();
    if (tabId === 'settings') loadSettings();

    // Show new view
    const view = document.getElementById(`view-${tabId}`);
    if (view) view.classList.add('active');
    currentTab = tabId;
}

// --- Chat Logic ---

function initChat() {
    const input = document.getElementById('chat-input');
    const sendBtn = document.getElementById('send-btn');

    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendMessage();
        }
    });

    sendBtn.addEventListener('click', sendMessage);
    
    // Load history (stub)
    loadChatHistory();
}

async function loadChatHistory() {
    const data = await api.get(`/sessions/${currentSession}`);
    if (data && data.messages) {
        const container = document.getElementById('chat-messages');
        container.innerHTML = ''; // Request clear
        // Add welcome message back
        if (data.messages.length === 0) {
            addMessage('system', 'Welcome to HighClaw Control. Start a conversation with your agent.');
        }
        data.messages.forEach(msg => {
            addMessage(msg.role, msg.content);
        });
        scrollToBottom();
    }
}

async function sendMessage() {
    const input = document.getElementById('chat-input');
    const text = input.value.trim();
    if (!text) return;

    // Optimistic UI
    addMessage('user', text);
    input.value = '';
    scrollToBottom();

    // Send to API
    const res = await api.post('/chat', {
        message: text,
        session: currentSession
    });

    if (res && res.response) {
        addMessage('assistant', res.response);
        
        // Update tokens if provided
        if (res.usage) {
            document.getElementById('token-usage').innerText = `${res.usage} tokens used`;
        }
    } else {
        addMessage('system', 'Error: Failed to get response from agent.');
    }
    scrollToBottom();
}

function addMessage(role, content) {
    const container = document.getElementById('chat-messages');
    
    const div = document.createElement('div');
    div.className = `message ${role}`;
    
    let avatarIcon = 'fas fa-user';
    if (role === 'assistant') avatarIcon = 'fas fa-robot';
    if (role === 'system') avatarIcon = 'fas fa-info-circle';

    div.innerHTML = `
        <div class="avatar"><i class="${avatarIcon}"></i></div>
        <div class="content"><p>${escapeHtml(content)}</p></div>
    `;
    
    container.appendChild(div);
}

function scrollToBottom() {
    const container = document.getElementById('chat-messages');
    container.scrollTop = container.scrollHeight;
}

// --- Status & Config ---

async function updateStatus() {
    const status = await api.get('/status');
    if (status) {
        if (uptimeInterval) clearInterval(uptimeInterval);
        document.getElementById('uptime-display').innerHTML = `Uptime: ${status.uptime}`;
        document.querySelector('.status-indicator').classList.add('online');
        document.querySelector('.status-text').innerText = 'Gateway Active';
    } else {
        document.querySelector('.status-indicator').classList.remove('online');
        document.querySelector('.status-text').innerText = 'Gateway Offline';
    }
}

async function loadConfig() {
    const cfg = await api.get('/config');
    if (cfg && cfg.agent) {
        agentModel = cfg.agent.model;
        document.getElementById('current-model').innerText = agentModel;
    }
}

// --- Views Loaders ---

async function loadSessions() {
    const data = await api.get('/sessions');
    const container = document.getElementById('sessions-list');
    container.innerHTML = '';
    
    if (data && data.sessions) {
        data.sessions.forEach(s => {
            const card = document.createElement('div');
            card.className = 'card';
            card.innerHTML = `
                <h3>Session: ${s.key}</h3>
                <p>Channel: ${s.channel}</p>
                <p>Messages: ${s.messageCount}</p>
                <p>Last Active: ${timeAgo(s.lastActivity)}</p>
                <button class="btn btn-primary" onclick="switchSession('${s.key}')">Open Chat</button>
            `;
            container.appendChild(card);
        });
    }
}

window.switchSession = (key) => {
    currentSession = key;
    document.getElementById('session-key').innerText = key;
    switchTab('chat');
    loadChatHistory();
};

async function loadChannels() {
    const data = await api.get('/channels/status');
    const container = document.getElementById('channels-list');
    container.innerHTML = '';
    
    if (data && data.channels) {
        data.channels.forEach(ch => {
            const card = document.createElement('div');
            card.className = 'card';
            card.innerHTML = `
                <h3><i class="${ch.icon}"></i> ${capitalize(ch.name)}</h3>
                <p>Status: <span class="badge" style="background:${ch.status === 'configured' ? '#238636' : '#8b949e'}">${ch.status}</span></p>
            `;
            container.appendChild(card);
        });
    }
}

async function loadSkills() {
    const data = await api.get('/skills');
    const container = document.getElementById('skills-list');
    container.innerHTML = '';

    if (data && data.skills) {
        data.skills.forEach(skill => {
            const card = document.createElement('div');
            card.className = 'card';
            card.innerHTML = `
                <h3>${skill.icon} ${skill.name}</h3>
                <p>${skill.description}</p>
            `;
            container.appendChild(card);
        });
    }
}

async function loadSettings() {
    const cfg = await api.get('/config');
    const models = await api.get('/models');
    
    if (cfg) {
        document.getElementById('setting-workspace').value = cfg.agent.workspace;
        
        // Populate models
        const select = document.getElementById('setting-model');
        select.innerHTML = '';
        if (models && models.models) {
            models.models.forEach(m => {
                const opt = document.createElement('option');
                opt.value = `${m.provider}/${m.id}`;
                opt.innerText = `${m.provider} - ${m.name}`;
                if (opt.value === cfg.agent.model) opt.selected = true;
                select.appendChild(opt);
            });
        }
    }
}

async function saveSettings() {
    const model = document.getElementById('setting-model').value;
    await api.post('/config', {
        agent: { model: model }
    });
    alert('Settings saved!');
    loadConfig();
}

function initStatus() {
    // Initial status check
    updateStatus();
    loadConfig();
}

// Helpers
function escapeHtml(text) {
    if (!text) return "";
    return text.toString()
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

function capitalize(s) {
    return s.charAt(0).toUpperCase() + s.slice(1);
}

function timeAgo(dateString) {
    const date = new Date(dateString);
    const seconds = Math.floor((new Date() - date) / 1000);
    if (seconds < 60) return "just now";
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ago`;
}
