const STORAGE_KEYS = {
  theme: "hc_theme",
  lang: "hc_lang",
  token: "hc_auth_token",
  localSessions: "hc_local_sessions",
};

const state = {
  theme: localStorage.getItem(STORAGE_KEYS.theme) || "dark",
  lang: localStorage.getItem(STORAGE_KEYS.lang) || "en",
  token: "",
  authenticated: true,
  authUsername: "",
  config: null,
  meta: null,
  models: [],
  providers: [],
  channels: [],
  skills: [],
  sessions: [],
  localSessions: [],
  selectedModel: "",
  catalogModels: [],
  currentPage: "chat",
  currentSession: "",
};

const i18n = {
  en: {
    new_session: "New session",
    sessions_label: "Sessions",
    settings: "Settings",
    chat_welcome: "What can I help with?",
    chat_hint: "Send a message to start a conversation",
    chat_placeholder: "Message...",
    tab_models: "AI Models",
    tab_providers: "AI Providers",
    tab_messengers: "Messengers",
    tab_skills: "Skills",
    tab_other: "Other",
    models_title: "AI Models",
    live_model: "Live Model",
    change_model: "Change Model",
    search_models: "Search models...",
    add_provider: "Add Provider",
    apply_model: "Apply Model",
    providers_title: "Providers & API Keys",
    messengers_title: "Messengers",
    skills_title: "Skills and Integrations",
    search_skills: "Search by skills...",
    add_custom_skill: "Add custom skill",
    other_title: "Other",
    openclaw_folder: "OpenClaw Folder",
    openclaw_folder_value: "OpenClaw folder",
    openclaw_folder_desc: "Contains your local OpenClaw state and app data.",
    open_folder: "Open folder",
    workspace: "Workspace",
    workspace_value: "Agent workspace",
    workspace_desc: "Contains editable .md files (AGENTS, SOUL, USER, IDENTITY, TOOLS, HEARTBEAT, BOOTSTRAP) that shape the agent.",
    terminal_title: "Terminal",
    show_in_sidebar: "Show in sidebar",
    open_terminal: "Open Terminal",
    terminal_desc: "Built-in terminal with openclaw and bundled tools in PATH.",
    app_title: "App",
    version_label: "Version",
    auto_start: "Auto start",
    license: "License",
    about_title: "About",
    online: "Online",
    offline: "Offline",
    save_ok: "Saved successfully",
    save_failed: "Save failed",
    load_failed: "Failed to load data",
    no_models: "No models found",
    no_skills: "No skills found",
    coming_soon: "Coming Soon",
    connect: "Connect",
    edit: "Edit",
  },
  zh: {
    new_session: "新会话",
    sessions_label: "会话",
    settings: "设置",
    chat_welcome: "我可以帮你做什么？",
    chat_hint: "发送一条消息开始对话",
    chat_placeholder: "输入消息...",
    tab_models: "AI Models",
    tab_providers: "AI Providers",
    tab_messengers: "Messengers",
    tab_skills: "Skills",
    tab_other: "Other",
    models_title: "AI 模型",
    live_model: "当前模型",
    change_model: "切换模型",
    search_models: "搜索模型...",
    add_provider: "添加 Provider",
    apply_model: "应用模型",
    providers_title: "提供商与 API Keys",
    messengers_title: "消息渠道",
    skills_title: "技能与集成",
    search_skills: "按技能搜索...",
    add_custom_skill: "添加自定义技能",
    other_title: "其他",
    openclaw_folder: "OpenClaw 目录",
    openclaw_folder_value: "OpenClaw folder",
    openclaw_folder_desc: "包含你的本地 OpenClaw 状态和应用数据。",
    open_folder: "打开目录",
    workspace: "工作空间",
    workspace_value: "Agent workspace",
    workspace_desc: "包含可编辑 .md 文件（AGENTS, SOUL, USER, IDENTITY, TOOLS, HEARTBEAT, BOOTSTRAP）用于塑造代理。",
    terminal_title: "终端",
    show_in_sidebar: "在侧栏显示",
    open_terminal: "打开终端",
    terminal_desc: "内置终端，PATH 中包含 openclaw 及工具。",
    app_title: "应用",
    version_label: "版本",
    auto_start: "开机启动",
    license: "许可证",
    about_title: "关于",
    online: "在线",
    offline: "离线",
    save_ok: "保存成功",
    save_failed: "保存失败",
    load_failed: "加载失败",
    no_models: "没有可用模型",
    no_skills: "没有技能",
    coming_soon: "即将支持",
    connect: "连接",
    edit: "编辑",
  },
};

const PROVIDER_CARDS = [
  { key: "anthropic", name: "Anthropic (Claude)", icon: "/static/assets/ai-providers/anthropic.svg", desc: "Best for complex reasoning, long-form writing and precise instructions" },
  { key: "openrouter", name: "OpenRouter", icon: "/static/assets/ai-providers/openrouter.svg", desc: "One gateway to 200+ AI models. Ideal for flexibility and experimentation" },
  { key: "google", name: "Google (Gemini)", icon: "/static/assets/ai-providers/gemini.svg", desc: "Strong with images, documents and large amounts of context" },
  { key: "openai", name: "OpenAI (GPT)", icon: "/static/assets/ai-providers/opeanai.svg", desc: "An all-rounder for chat, coding, and everyday tasks" },
  { key: "zai", name: "Z.ai (GLM)", icon: "/static/assets/ai-providers/zai.svg", desc: "Cost-effective models for everyday tasks and high-volume usage" },
  { key: "minimax", name: "MiniMax", icon: "/static/assets/ai-providers/minimax.svg", desc: "Good for creative writing and expressive conversations" },
];

const MESSENGER_CARDS = [
  { key: "telegram", name: "Telegram", icon: "/static/assets/messengers/Telegram.svg", desc: "Connect a Telegram bot to receive and send messages", available: true },
  { key: "slack", name: "Slack", icon: "/static/assets/set-up-skills/Slack.svg", desc: "Connect a Slack workspace via Socket Mode", available: true },
  { key: "discord", name: "Discord", icon: "/static/assets/messengers/Discord.svg", desc: "Connect a Discord bot to interact with your server", available: true },
  { key: "whatsapp", name: "WhatsApp", icon: "/static/assets/messengers/WhatsApp.svg", desc: "Connect WhatsApp Web via QR code pairing", available: false },
  { key: "signal", name: "Signal", icon: "/static/assets/messengers/Signal.svg", desc: "Connect Signal via signal-cli for private messaging", available: false },
  { key: "imessage", name: "iMessage", icon: "/static/assets/messengers/iMessage.svg", desc: "Connect iMessage on macOS for native messaging", available: false },
  { key: "matrix", name: "Matrix", icon: "/static/assets/messengers/Matrix.svg", desc: "Connect to a Matrix homeserver for decentralized messaging", available: false },
  { key: "teams", name: "Microsoft Teams", icon: "/static/assets/messengers/Microsoft-Teams.svg", desc: "Connect Microsoft Teams for enterprise messaging", available: false },
];

const SKILL_CARDS = [
  { key: "gworkspace", name: "Google Workspace", icon: "/static/assets/set-up-skills/Google.svg", desc: "Clears your inbox, sends emails and manages your calendar", status: "connect" },
  { key: "apple-notes", name: "Apple Notes", icon: "/static/assets/set-up-skills/Notes.svg", desc: "Create, search and organize your notes", status: "connect" },
  { key: "apple-reminders", name: "Apple Reminders", icon: "/static/assets/set-up-skills/Reminders.svg", desc: "Add, list and complete your reminders", status: "connect" },
  { key: "notion", name: "Notion", icon: "/static/assets/set-up-skills/Notion.svg", desc: "Create, search, update and organize your Notion pages", status: "connect" },
  { key: "github", name: "GitHub", icon: "/static/assets/set-up-skills/GitHub.svg", desc: "Review pull requests, manage issues and workflows", status: "connect" },
  { key: "trello", name: "Trello", icon: "/static/assets/set-up-skills/Trello.svg", desc: "Track tasks, update boards and manage your projects", status: "connect" },
  { key: "slack", name: "Slack", icon: "/static/assets/set-up-skills/Slack.svg", desc: "Send messages, search info and manage pins in your workspace", status: "connect" },
  { key: "obsidian", name: "Obsidian", icon: "/static/assets/set-up-skills/Obsidian.svg", desc: "Search and manage your Obsidian vaults", status: "connect" },
  { key: "media-analysis", name: "Media Analysis", icon: "/static/assets/set-up-skills/Media.svg", desc: "Analyze images, audio and video from external sources", status: "connect" },
  { key: "web-search", name: "Advanced Web Search", icon: "/static/assets/set-up-skills/Web-Search.svg", desc: "Lets the bot fetch fresh web data using external providers", status: "connect" },
  { key: "eleven-labs", name: "Eleven Labs", icon: "/static/assets/set-up-skills/Sag.svg", desc: "Create lifelike speech with AI voice generator", status: "coming" },
  { key: "nano-banana", name: "Nano Banana (Images)", icon: "/static/assets/set-up-skills/Nano-Banana.svg", desc: "Generate AI images from text prompts", status: "coming" },
];

const PRESET_MODELS = [
  { provider: "openrouter", id: "z-ai/glm-4.5", name: "Z.ai: GLM 4.5", maxTokens: "131K", reasoning: true, tag: "" },
  { provider: "openrouter", id: "arcee-ai/trinity-large-preview", name: "Arcee AI: Trinity Large Preview (free)", maxTokens: "131K", reasoning: false, tag: "ultra" },
  { provider: "openrouter", id: "moonshotai/kimi-k2.5", name: "MoonshotAI: Kimi K2.5", maxTokens: "262K", reasoning: true, tag: "pro" },
  { provider: "openrouter", id: "google/gemini-3-flash-preview", name: "Google: Gemini 3 Flash Preview", maxTokens: "1.0M", reasoning: true, tag: "fast" },
  { provider: "openrouter", id: "ai21/jamba-large-1.7", name: "AI21: Jamba Large 1.7", maxTokens: "256K", reasoning: false, tag: "" },
  { provider: "openrouter", id: "allenai/olmo-3.1-32b", name: "AllenAI: Olmo 3.1 32B Instruct", maxTokens: "66K", reasoning: false, tag: "" },
  { provider: "openrouter", id: "amazon/nova-2-lite", name: "Amazon: Nova 2 Lite", maxTokens: "1.0M", reasoning: true, tag: "" },
  { provider: "openrouter", id: "amazon/nova-lite-1.0", name: "Amazon: Nova Lite 1.0", maxTokens: "300K", reasoning: false, tag: "" },
  { provider: "openrouter", id: "amazon/nova-micro-1.0", name: "Amazon: Nova Micro 1.0", maxTokens: "128K", reasoning: false, tag: "" },
  { provider: "openrouter", id: "amazon/nova-premier-1.0", name: "Amazon: Nova Premier 1.0", maxTokens: "1.0M", reasoning: false, tag: "" },
  { provider: "openrouter", id: "amazon/nova-pro-1.0", name: "Amazon: Nova Pro 1.0", maxTokens: "300K", reasoning: false, tag: "" },
];

const MODEL_CATALOG_URL = "/static/models-catalog.txt?v=20260216";
const HOT_KEYWORDS = [
  "gpt-5", "claude", "gemini 2.5", "gemini 3", "kimi", "deepseek r1", "deepseek v3.2", "grok 4", "glm 4.6", "glm 4.7", "o3", "o4", "qwen3",
];
const KNOWN_MODEL_ID_MAP = {
  "z.ai: glm 4.5": "openrouter/z-ai/glm-4.5",
  "arcee ai: trinity large preview (free)": "openrouter/arcee-ai/trinity-large-preview",
  "moonshotai: kimi k2.5": "openrouter/moonshotai/kimi-k2.5",
  "google: gemini 3 flash preview": "openrouter/google/gemini-3-flash-preview",
  "ai21: jamba large 1.7": "openrouter/ai21/jamba-large-1.7",
  "allenai: olmo 3.1 32b instruct": "openrouter/allenai/olmo-3.1-32b",
  "amazon: nova 2 lite": "openrouter/amazon/nova-2-lite",
  "amazon: nova lite 1.0": "openrouter/amazon/nova-lite-1.0",
  "amazon: nova micro 1.0": "openrouter/amazon/nova-micro-1.0",
  "amazon: nova premier 1.0": "openrouter/amazon/nova-premier-1.0",
  "amazon: nova pro 1.0": "openrouter/amazon/nova-pro-1.0",
};

function t(key) {
  return i18n[state.lang]?.[key] || i18n.en[key] || key;
}

function getEl(id) {
  return document.getElementById(id);
}

function escapeHtml(text) {
  if (text === null || text === undefined) return "";
  return String(text).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/\"/g, "&quot;").replace(/'/g, "&#039;");
}

function showToast(msg, isError = false) {
  const el = getEl("toast");
  if (!el) return;
  el.textContent = msg;
  el.style.borderColor = isError ? "#ef4444" : "#2f3340";
  el.classList.add("show");
  window.clearTimeout(showToast._timer);
  showToast._timer = window.setTimeout(() => el.classList.remove("show"), 1800);
}

function loadLocalSessions() {
  try {
    const raw = localStorage.getItem(STORAGE_KEYS.localSessions);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((s) => s && s.key && s.key !== "main");
  } catch {
    return [];
  }
}

function persistLocalSessions() {
  try {
    localStorage.setItem(STORAGE_KEYS.localSessions, JSON.stringify(state.localSessions || []));
  } catch {
    // ignore
  }
}

function normalizeModelName(name) {
  return String(name || "").trim().toLowerCase().replace(/\s+/g, " ");
}

function categoryForModel(modelName) {
  const n = normalizeModelName(modelName);
  if (n.includes("(free)") || n === "free models router") return "free";
  if (HOT_KEYWORDS.some((k) => n.includes(k))) return "hot";
  return "normal";
}

function parseCatalogModels(rawText) {
  const blocks = String(rawText || "")
    .split(/\n\s*\n/g)
    .map((b) => b.trim())
    .filter(Boolean);
  return blocks.map((block) => {
    const lines = block.split("\n").map((l) => l.trim()).filter(Boolean);
    const name = lines[0] || "";
    let speedTag = "";
    let ctxLine = lines[1] || "";
    if (ctxLine && !ctxLine.toLowerCase().startsWith("ctx ")) {
      speedTag = ctxLine.toLowerCase();
      ctxLine = lines[2] || "";
    }
    const reasoning = ctxLine.toLowerCase().includes("reasoning");
    const maxTokens = ctxLine.toLowerCase().startsWith("ctx ") ? ctxLine.slice(4).split("·")[0].trim() : "-";
    return { name, speedTag, ctxLine, maxTokens, reasoning, category: categoryForModel(name) };
  });
}

function normalizeBackendModels(rawModels) {
  return (rawModels || []).map((m) => ({
    provider: m.provider || m.Provider || "",
    id: m.id || m.ID || "",
    name: m.name || m.Name || "",
    maxTokens: m.maxTokens || m.MaxTokens || "",
    capabilities: m.capabilities || m.Capabilities || [],
  }));
}

function modelRefForCatalogItem(item) {
  const nameKey = normalizeModelName(item.name);
  if (KNOWN_MODEL_ID_MAP[nameKey]) return KNOWN_MODEL_ID_MAP[nameKey];
  const matched = (state.models || []).find((m) => normalizeModelName(m.name) === nameKey);
  if (matched?.provider && matched?.id) return `${matched.provider}/${matched.id}`;
  return "";
}

async function loadModelCatalog() {
  try {
    const res = await fetch(MODEL_CATALOG_URL, { credentials: "same-origin" });
    if (!res.ok) return;
    const text = await res.text();
    state.catalogModels = parseCatalogModels(text);
  } catch {
    state.catalogModels = [];
  }
}

async function apiRequest(path, options = {}) {
  const headers = { "Content-Type": "application/json", ...(options.headers || {}) };

  const res = await fetch(`/api${path}`, { ...options, headers, credentials: "same-origin" });
  const ct = res.headers.get("content-type") || "";
  const body = ct.includes("application/json") ? await res.json() : await res.text();
  if (!res.ok) {
    const errMsg = typeof body === "string" ? body : body?.error || `HTTP ${res.status}`;
    const err = new Error(errMsg);
    err.status = res.status;
    if (res.status === 401 && !options.skipAuthRedirect) {
      state.authenticated = false;
      showLogin();
    }
    throw err;
  }
  return body;
}

function setTheme(theme) {
  state.theme = theme;
  localStorage.setItem(STORAGE_KEYS.theme, theme);
  document.documentElement.dataset.theme = theme;
}

function applyI18n() {
  document.documentElement.lang = state.lang;
  document.querySelectorAll("[data-i18n]").forEach((el) => (el.textContent = t(el.dataset.i18n)));
  document.querySelectorAll("[data-i18n-placeholder]").forEach((el) => el.setAttribute("placeholder", t(el.dataset.i18nPlaceholder)));
}

function showLogin(errorMsg = "") {
  // Basic Auth mode: browser handles auth prompt directly.
  if (errorMsg) showToast(errorMsg, true);
}

function hideLogin() {
  // no-op in Basic Auth mode
}

async function ensureAuthenticated() {
  state.authenticated = true;
  return true;
}

function switchPage(page) {
  state.currentPage = page;
  document.querySelectorAll(".page-view").forEach((el) => el.classList.remove("active"));
  const pageEl = getEl(`page-${page}`);
  if (pageEl) pageEl.classList.add("active");
  getEl("nav-settings")?.classList.toggle("active", page === "settings");
}

function switchSettingsTab(tab) {
  document.querySelectorAll(".settings-tab").forEach((b) => b.classList.toggle("active", b.dataset.settingsTab === tab));
  document.querySelectorAll(".settings-view").forEach((v) => v.classList.remove("active"));
  getEl(`settings-view-${tab}`)?.classList.add("active");
}

function renderChat(messages = []) {
  const list = getEl("chat-list");
  const hero = document.querySelector(".chat-hero");
  if (!list) return;
  list.innerHTML = "";
  if (!messages.length) {
    hero?.classList.remove("hidden");
    list.classList.remove("active");
    return;
  }
  hero?.classList.add("hidden");
  list.classList.add("active");
  messages.forEach((m) => {
    const role = escapeHtml(m.role || "assistant");
    const content = escapeHtml(m.content || "");
    const bubble = document.createElement("div");
    bubble.className = `chat-msg ${role === "user" ? "user" : "assistant"}`;
    bubble.innerHTML = `<div class="chat-role">${role}</div><p>${content}</p>`;
    list.appendChild(bubble);
  });
  list.scrollTop = list.scrollHeight;
}

function renderSidebarSessions() {
  const nav = getEl("nav-sessions");
  const title = document.querySelector(".session-title");
  if (!nav) return;
  const remote = (state.sessions || []).filter((s) => s.key && s.key !== "main");
  const remoteKeys = new Set(remote.map((s) => s.key));
  const localOnly = (state.localSessions || []).filter((s) => !remoteKeys.has(s.key));
  const items = [...remote, ...localOnly].sort((a, b) => Number(b.lastActivityAt || b.updatedAt || 0) - Number(a.lastActivityAt || a.updatedAt || 0));

  if (!items.length) {
    nav.innerHTML = "";
    if (title) title.style.display = "none";
    return;
  }
  if (title) title.style.display = "";

  const displayLabel = (s) => {
    const raw = String(s.label || s.title || s.name || s.key || "").trim();
    return raw.length > 24 ? `${raw.slice(0, 24)}…` : raw;
  };

  nav.innerHTML = items.map((s) => `<button class="session-item ${state.currentSession === s.key ? "active" : ""}" data-session="${escapeHtml(s.key)}">${escapeHtml(displayLabel(s))}</button>`).join("");

  nav.querySelectorAll(".session-item").forEach((btn) => {
    btn.addEventListener("click", () => {
      state.currentSession = btn.dataset.session || "";
      switchPage("chat");
      refreshSessionMessages();
      renderSidebarSessions();
    });
  });
}

function renderModels() {
  const list = getEl("models-list");
  if (!list) return;
  const q = (getEl("model-search")?.value || "").trim().toLowerCase();
  const source = state.catalogModels?.length ? state.catalogModels : PRESET_MODELS.map((m) => ({ name: m.name, speedTag: m.tag || "", ctxLine: `ctx ${m.maxTokens}${m.reasoning ? " · reasoning" : ""}`, maxTokens: m.maxTokens, reasoning: !!m.reasoning, category: categoryForModel(m.name) }));
  let models = source.filter((m) => String(m.name || "").toLowerCase().includes(q) || String(m.ctxLine || "").toLowerCase().includes(q));

  const liveModel = state.config?.agent?.model || "openrouter/z-ai/glm-4.5";
  const backendActive = (state.models || []).find((m) => `${m.provider}/${m.id}` === liveModel);
  const activeByCatalog = source.find((m) => modelRefForCatalogItem(m) === liveModel);
  const liveName = activeByCatalog?.name || backendActive?.name || liveModel;
  const liveMeta = activeByCatalog?.ctxLine ? `OpenRouter · ${activeByCatalog.ctxLine}` : backendActive ? `${backendActive.provider} · ctx ${backendActive.maxTokens || "-"}${Array.isArray(backendActive.capabilities) && backendActive.capabilities.includes("reasoning") ? " · reasoning" : ""}` : "-";
  const liveCard = getEl("live-model-card");
  if (liveCard) liveCard.innerHTML = `<strong>${escapeHtml(liveName)}</strong><p>${escapeHtml(liveMeta)}</p>`;

  models = models.filter((m) => modelRefForCatalogItem(m) !== liveModel);
  list.innerHTML = "";
  if (!models.length) {
    list.innerHTML = `<div class="model-empty">${t("no_models")}</div>`;
    return;
  }

  const grouped = new Map();
  grouped.set("high_heat", models.filter((m) => m.category === "hot"));
  grouped.set("free", models.filter((m) => m.category === "free"));
  grouped.set("normal", models.filter((m) => m.category === "normal"));

  let idx = 0;
  grouped.forEach((providerModels, provider) => {
    if (!providerModels.length) return;
    const head = document.createElement("div");
    head.className = "model-provider-head";
    head.textContent = provider === "high_heat" ? "HIGH HEAT" : provider === "free" ? "FREE" : "NORMAL";
    list.appendChild(head);

    providerModels.forEach((m) => {
      const full = modelRefForCatalogItem(m);
      const checked = full && (state.selectedModel === full || state.config?.agent?.model === full);
      let tag = "";
      if (m.speedTag === "ultra") tag = '<span class="model-tag ultra">Ultra</span>';
      else if (m.speedTag === "pro") tag = '<span class="model-tag pro">Pro</span>';
      else if (m.speedTag === "fast") tag = '<span class="model-tag fast">Fast</span>';
      else if (idx % 11 === 0) tag = '<span class="model-tag ultra">Ultra</span>';
      else if (idx % 7 === 0) tag = '<span class="model-tag pro">Pro</span>';
      else if (idx % 5 === 0) tag = '<span class="model-tag fast">Fast</span>';
      idx += 1;

      const categoryTag = m.category === "free" ? '<span class="model-tag free">Free</span>' : m.category === "hot" ? '<span class="model-tag hot">Hot</span>' : '<span class="model-tag normal">Normal</span>';
      const disabled = !full ? "disabled" : "";
      const row = document.createElement("label");
      row.className = "model-item";
      row.innerHTML = `<input type="radio" name="model-choice" value="${escapeHtml(full || "")}" ${checked ? "checked" : ""} ${disabled}><div><strong>${escapeHtml(m.name || "")} ${tag} ${categoryTag}</strong><p>${escapeHtml(m.ctxLine || `ctx ${m.maxTokens || "-"}`)}</p></div>`;
      row.querySelector("input")?.addEventListener("change", (e) => {
        if (!e.target.value) {
          showToast("This model is catalog-only. Configure an exact model id first.", true);
          e.target.checked = false;
          return;
        }
        state.selectedModel = e.target.value;
        applyModel(true);
      });
      list.appendChild(row);
    });
  });
}

function renderCard(gridId, cards, actionType) {
  const grid = getEl(gridId);
  if (!grid) return;
  grid.innerHTML = cards.map((c) => `
    <div class="setting-card ${c.active ? "active-provider" : ""}">
      <div class="card-top">
        <span class="card-icon-wrap">
          <img src="${c.icon}" alt="${escapeHtml(c.name)}">
          ${c.active ? '<span class="active-check" title="Active">✓</span>' : ""}
        </span>
        <button class="status-pill ${c.status === "coming" ? "coming" : c.status === "edit" ? "edit" : ""}" data-action="${actionType}" data-key="${escapeHtml(c.key || "")}" ${c.status === "coming" ? "disabled" : ""}>${c.status === "connect" ? t("connect") : c.status === "edit" ? t("edit") : t("coming_soon")}</button>
      </div>
      <h4>${escapeHtml(c.name)}</h4>
      <p class="muted">${escapeHtml(c.desc)}</p>
    </div>
  `).join("");
}

function providerConfigured(providerKey) {
  return Boolean(state.meta?.providers?.configured?.[providerKey]);
}

function normalizeProviderKeyFromModel(modelRef) {
  const rawProvider = String(modelRef || "").trim().toLowerCase().split("/")[0];
  if (!rawProvider) return "";
  if (rawProvider === "z-ai") return "zai";
  if (rawProvider === "gemini") return "google";
  return rawProvider;
}

function channelConfigured(channelKey) {
  return Boolean(state.meta?.channels?.[channelKey]);
}

function renderProvidersSettings() {
  const activeProviderKey = normalizeProviderKeyFromModel(state.selectedModel || state.config?.agent?.model || "");
  const cards = PROVIDER_CARDS.map((p) => ({
    ...p,
    status: providerConfigured(p.key) ? "edit" : "connect",
    active: providerConfigured(p.key) || p.key === activeProviderKey,
  }));
  renderCard("providers-grid", cards, "provider");
}

function renderMessengersSettings() {
  const cards = MESSENGER_CARDS.map((c) => {
    if (!c.available) return { ...c, status: "coming" };
    return { ...c, status: channelConfigured(c.key) ? "edit" : "connect" };
  });
  renderCard("messengers-grid", cards, "messenger");
}

function renderSkillsSettings() {
  const q = (getEl("skills-search")?.value || "").trim().toLowerCase();
  const statusByName = new Map((state.skills || []).map((s) => [String(s.name || "").toLowerCase(), String(s.status || "").toLowerCase()]));
  const statusByID = new Map((state.skills || []).map((s) => [String(s.id || "").toLowerCase(), String(s.status || "").toLowerCase()]));
  const cards = SKILL_CARDS.map((s) => {
    const st = statusByID.get(String(s.key || "").toLowerCase()) || statusByName.get(String(s.name || "").toLowerCase());
    if (!st) return s;
    return { ...s, status: st === "eligible" ? "connect" : s.status === "coming" ? "coming" : "edit" };
  });
  const filtered = cards.filter((s) => String(s.name || "").toLowerCase().includes(q) || String(s.desc || "").toLowerCase().includes(q));
  if (!filtered.length) {
    const grid = getEl("skills-grid");
    if (grid) grid.innerHTML = `<div class="setting-card">${t("no_skills")}</div>`;
    return;
  }
  renderCard("skills-grid", filtered, "skill");
}

function renderOtherSettings() {
  const prefTerminal = getEl("pref-show-terminal");
  const prefAutoStart = getEl("pref-auto-start");
  if (prefTerminal) prefTerminal.checked = Boolean(state.meta?.web?.showTerminalInSidebar);
  if (prefAutoStart) prefAutoStart.checked = Boolean(state.meta?.web?.autoStart);
}

function renderSettingsAll() {
  renderModels();
  renderProvidersSettings();
  renderMessengersSettings();
  renderSkillsSettings();
  renderOtherSettings();
}

async function refreshSessionMessages() {
  try {
    const key = state.currentSession;
    if (!key) {
      renderChat([]);
      return;
    }
    const session = await apiRequest(`/sessions/${encodeURIComponent(key)}`);
    renderChat(session?.messages || []);
  } catch {
    renderChat([]);
  }
}

async function refreshData() {
  const settled = await Promise.allSettled([
    apiRequest("/status"),
    apiRequest("/config"),
    apiRequest("/meta"),
    apiRequest("/models"),
    apiRequest("/providers"),
    apiRequest("/channels/status"),
    apiRequest("/skills"),
    apiRequest("/sessions"),
  ]);

  const valueAt = (idx) => (settled[idx].status === "fulfilled" ? settled[idx].value : null);
  state.config = valueAt(1) || null;
  state.meta = valueAt(2) || null;
  state.models = normalizeBackendModels(valueAt(3)?.models || []);
  state.providers = valueAt(4)?.providers || [];
  state.channels = valueAt(5)?.channels || [];
  state.skills = valueAt(6)?.skills || [];
  state.sessions = valueAt(7)?.sessions || [];
  state.selectedModel = state.config?.agent?.model || state.selectedModel || "openrouter/z-ai/glm-4.5";

  renderSidebarSessions();
  renderSettingsAll();
  await refreshSessionMessages();
}

async function applyModel(silent = false) {
  const model = state.selectedModel || state.config?.agent?.model;
  if (!model) return;
  try {
    await apiRequest("/config", { method: "PATCH", body: JSON.stringify({ agent: { model } }) });
    if (!silent) showToast(t("save_ok"));
    await refreshData();
  } catch (err) {
    showToast(`${t("save_failed")}: ${err.message}`, true);
  }
}

async function sendChat() {
  const input = getEl("chat-input");
  if (!input) return;
  const message = input.value.trim();
  if (!message) return;
  if (!state.currentSession) state.currentSession = `session-${Date.now().toString(36)}`;

  input.value = "";
  const key = state.currentSession;
  const title = message.length > 24 ? `${message.slice(0, 24)}…` : message;
  const now = Date.now();
  const idx = state.localSessions.findIndex((s) => s.key === key);
  if (idx >= 0) state.localSessions[idx] = { ...state.localSessions[idx], title: state.localSessions[idx].title || title, updatedAt: now };
  else state.localSessions.unshift({ key, title, updatedAt: now });
  persistLocalSessions();
  renderSidebarSessions();

  try {
    await apiRequest("/chat", { method: "POST", body: JSON.stringify({ session: key, message }) });
    await refreshSessionMessages();
  } catch (err) {
    showToast(`${t("save_failed")}: ${err.message}`, true);
  }
}

async function updateProvider(providerKey) {
  const apiKey = window.prompt(`Enter API key for ${providerKey}`);
  if (apiKey === null) return;
  const defaultBaseURL = providerKey === "openrouter" ? "https://openrouter.ai/api/v1" : "";
  const baseUrl = window.prompt(`Enter base URL for ${providerKey} (optional)`, defaultBaseURL) || "";
  try {
    await apiRequest("/config", {
      method: "PATCH",
      body: JSON.stringify({ agent: { providers: { [providerKey]: { apiKey: apiKey.trim(), baseUrl: baseUrl.trim() } } } }),
    });
    showToast(t("save_ok"));
    await refreshData();
  } catch (err) {
    showToast(`${t("save_failed")}: ${err.message}`, true);
  }
}

async function updateMessenger(channelKey) {
  try {
    if (channelKey === "telegram") {
      const botToken = window.prompt("Telegram Bot Token");
      if (botToken === null) return;
      await apiRequest("/config", { method: "PATCH", body: JSON.stringify({ channels: { telegram: { botToken: botToken.trim() } } }) });
    } else if (channelKey === "discord") {
      const token = window.prompt("Discord Bot Token");
      if (token === null) return;
      await apiRequest("/config", { method: "PATCH", body: JSON.stringify({ channels: { discord: { token: token.trim() } } }) });
    } else if (channelKey === "slack") {
      const botToken = window.prompt("Slack Bot Token");
      if (botToken === null) return;
      const appToken = window.prompt("Slack App Token", "") || "";
      await apiRequest("/config", { method: "PATCH", body: JSON.stringify({ channels: { slack: { botToken: botToken.trim(), appToken: appToken.trim() } } }) });
    }
    showToast(t("save_ok"));
    await refreshData();
  } catch (err) {
    showToast(`${t("save_failed")}: ${err.message}`, true);
  }
}

async function connectSkill(skillID) {
  const allow = Array.isArray(state.config?.agent?.sandbox?.allow) ? state.config.agent.sandbox.allow.slice() : [];
  if (!allow.includes(skillID)) allow.push(skillID);
  try {
    await apiRequest("/config", { method: "PATCH", body: JSON.stringify({ agent: { sandbox: { ...(state.config?.agent?.sandbox || {}), allow } } }) });
    showToast(t("save_ok"));
    await refreshData();
  } catch (err) {
    showToast(`${t("save_failed")}: ${err.message}`, true);
  }
}

async function saveWebPreferences() {
  const showTerminalInSidebar = Boolean(getEl("pref-show-terminal")?.checked);
  const autoStart = Boolean(getEl("pref-auto-start")?.checked);
  try {
    await apiRequest("/config", { method: "PATCH", body: JSON.stringify({ web: { preferences: { showTerminalInSidebar, autoStart } } }) });
    await refreshData();
  } catch (err) {
    showToast(`${t("save_failed")}: ${err.message}`, true);
  }
}

function copyPathFromMeta(pathKey) {
  const path = state.meta?.paths?.[pathKey];
  if (!path) return;
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(path).then(() => showToast(`Copied: ${path}`)).catch(() => showToast(path));
  } else {
    showToast(path);
  }
}

async function handleCardAction(action, key) {
  if (!action || !key) return;
  if (action === "provider") {
    await updateProvider(key);
    return;
  }
  if (action === "messenger") {
    await updateMessenger(key);
    return;
  }
  if (action === "skill") {
    await connectSkill(key);
  }
}

function bindEvents() {
  getEl("nav-settings")?.addEventListener("click", () => switchPage("settings"));
  getEl("new-session")?.addEventListener("click", () => {
    state.currentSession = "";
    switchPage("chat");
    renderChat([]);
    renderSidebarSessions();
  });

  document.querySelectorAll(".settings-tab").forEach((btn) => {
    btn.addEventListener("click", () => switchSettingsTab(btn.dataset.settingsTab));
  });

  getEl("model-search")?.addEventListener("input", renderModels);
  getEl("skills-search")?.addEventListener("input", renderSkillsSettings);
  getEl("apply-model")?.addEventListener("click", applyModel);
  getEl("send-chat")?.addEventListener("click", sendChat);
  getEl("chat-input")?.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendChat();
    }
  });

  getEl("pref-show-terminal")?.addEventListener("change", saveWebPreferences);
  getEl("pref-auto-start")?.addEventListener("change", saveWebPreferences);
  getEl("open-config-dir")?.addEventListener("click", () => copyPathFromMeta("configDir"));
  getEl("open-workspace-dir")?.addEventListener("click", () => copyPathFromMeta("workspace"));

  document.body.addEventListener("click", (e) => {
    const target = e.target;
    if (!(target instanceof HTMLElement)) return;
    const btn = target.closest("button.status-pill");
    if (!btn) return;
    const action = btn.dataset.action;
    const key = btn.dataset.key;
    handleCardAction(action, key);
  });
}

async function init() {
  // Web auth has switched to browser Basic Auth, legacy bearer token is no longer used.
  localStorage.removeItem(STORAGE_KEYS.token);
  state.token = "";

  setTheme(state.theme);
  applyI18n();
  bindEvents();
  await loadModelCatalog();
  state.localSessions = loadLocalSessions();
  switchPage("chat");
  switchSettingsTab("models");

  await ensureAuthenticated();
  await refreshData();
  window.setInterval(() => {
    if (state.authenticated) refreshData();
  }, 22000);
}

document.addEventListener("DOMContentLoaded", init);
