import { html, nothing } from "lit";
import type { ConfigUiHints } from "../types.ts";

export type ConfigProps = {
  raw: string;
  originalRaw: string;
  valid: boolean | null;
  issues: unknown[];
  loading: boolean;
  saving: boolean;
  applying: boolean;
  updating: boolean;
  connected: boolean;
  schema: unknown;
  schemaLoading: boolean;
  uiHints: ConfigUiHints;
  formMode: "form" | "raw";
  formValue: Record<string, unknown> | null;
  originalValue: Record<string, unknown> | null;
  searchQuery: string;
  activeSection: string | null;
  activeSubsection: string | null;
  onRawChange: (next: string) => void;
  onFormModeChange: (mode: "form" | "raw") => void;
  onFormPatch: (path: Array<string | number>, value: unknown) => void;
  onSearchChange: (query: string) => void;
  onSectionChange: (section: string | null) => void;
  onSubsectionChange: (section: string | null) => void;
  onReload: () => void;
  onSave: () => void;
  onApply: () => void;
  onUpdate: () => void;
};

type SettingsTabId = "ai-models" | "ai-providers" | "messengers" | "skills" | "other";

const SETTINGS_TABS: Array<{ id: SettingsTabId; label: string }> = [
  { id: "ai-models", label: "AI Models" },
  { id: "ai-providers", label: "AI Providers" },
  { id: "messengers", label: "Messengers" },
  { id: "skills", label: "Skills" },
  { id: "other", label: "Other" },
];

const PROVIDERS = [
  {
    name: "Anthropic (Claude)",
    desc: "Best for complex reasoning, long-form writing and precise instructions",
    icon: "/atomicbot/ai-providers/anthropic.svg",
    action: "Connect",
  },
  {
    name: "OpenRouter",
    desc: "One gateway to 200+ AI models. Ideal for flexibility and experimentation",
    icon: "/atomicbot/ai-providers/openrouter.svg",
    action: "Edit",
  },
  {
    name: "Google (Gemini)",
    desc: "Strong with images, documents and large amounts of context",
    icon: "/atomicbot/ai-providers/gemini.svg",
    action: "Connect",
  },
  {
    name: "OpenAI (GPT)",
    desc: "An all-rounder for chat, coding, and everyday tasks",
    icon: "/atomicbot/ai-providers/opeanai.svg",
    action: "Connect",
  },
  {
    name: "Z.ai (GLM)",
    desc: "Cost-effective models for everyday tasks and high-volume usage",
    icon: "/atomicbot/ai-providers/zai.svg",
    action: "Connect",
  },
  {
    name: "MiniMax",
    desc: "Good for creative writing and expressive conversations",
    icon: "/atomicbot/ai-providers/minimax.svg",
    action: "Connect",
  },
];

const MESSENGERS = [
  {
    name: "Telegram",
    desc: "Connect a Telegram bot to receive and send messages",
    icon: "/atomicbot/messangers/Telegram.svg",
    action: "Connect",
  },
  {
    name: "Slack",
    desc: "Connect a Slack workspace via Socket Mode",
    icon: "/atomicbot/messangers/Slack.svg",
    action: "Connect",
  },
  {
    name: "Discord",
    desc: "Connect a Discord bot to interact with your server",
    icon: "/atomicbot/messangers/Discord.svg",
    action: "Coming Soon",
  },
  {
    name: "WhatsApp",
    desc: "Connect WhatsApp Web via QR code pairing",
    icon: "/atomicbot/messangers/WhatsApp.svg",
    action: "Coming Soon",
  },
  {
    name: "Signal",
    desc: "Connect Signal via signal-cli for private messaging",
    icon: "/atomicbot/messangers/Signal.svg",
    action: "Coming Soon",
  },
  {
    name: "iMessage",
    desc: "Connect iMessage on macOS for native messaging",
    icon: "/atomicbot/messangers/iMessage.svg",
    action: "Coming Soon",
  },
  {
    name: "Matrix",
    desc: "Connect to a Matrix homeserver for decentralized messaging",
    icon: "/atomicbot/messangers/Matrix.svg",
    action: "Coming Soon",
  },
  {
    name: "Microsoft Teams",
    desc: "Connect Microsoft Teams for enterprise messaging",
    icon: "/atomicbot/messangers/Microsoft-Teams.svg",
    action: "Coming Soon",
  },
];

const SKILLS = [
  {
    name: "Google Workspace",
    desc: "Clears your inbox, sends emails and manages your calendar",
    icon: "/atomicbot/set-up-skills/Google.svg",
    action: "Connect",
  },
  {
    name: "Apple Notes",
    desc: "Create, search and organize your notes",
    icon: "/atomicbot/set-up-skills/Notes.svg",
    action: "Connect",
  },
  {
    name: "Apple Reminders",
    desc: "Add, list and complete your reminders",
    icon: "/atomicbot/set-up-skills/Reminders.svg",
    action: "Connect",
  },
  {
    name: "Notion",
    desc: "Create, search, update and organize your Notion pages",
    icon: "/atomicbot/set-up-skills/Notion.svg",
    action: "Connect",
  },
  {
    name: "GitHub",
    desc: "Review pull requests, manage issues and workflows",
    icon: "/atomicbot/set-up-skills/GitHub.svg",
    action: "Connect",
  },
  {
    name: "Trello",
    desc: "Track tasks, update boards and manage your projects",
    icon: "/atomicbot/set-up-skills/Trello.svg",
    action: "Connect",
  },
  {
    name: "Slack",
    desc: "Send messages, search info and manage pins in workspace",
    icon: "/atomicbot/set-up-skills/Slack.svg",
    action: "Connect",
  },
  {
    name: "Obsidian",
    desc: "Search and manage your Obsidian vaults",
    icon: "/atomicbot/set-up-skills/Obsidian.svg",
    action: "Connect",
  },
  {
    name: "Media Analysis",
    desc: "Analyze images, audio and video from external sources",
    icon: "/atomicbot/set-up-skills/Media.svg",
    action: "Connect",
  },
  {
    name: "Advanced Web Search",
    desc: "Lets the bot fetch fresh web data using external providers",
    icon: "/atomicbot/set-up-skills/Web-Search.svg",
    action: "Connect",
  },
  {
    name: "Eleven Labs",
    desc: "Create lifelike speech with AI voice generator",
    icon: "/atomicbot/set-up-skills/Sag.svg",
    action: "Coming Soon",
  },
  {
    name: "Nano Banana (Images)",
    desc: "Generate AI images from text prompts",
    icon: "/atomicbot/set-up-skills/Nano-Banana.svg",
    action: "Coming Soon",
  },
];

function resolveActiveTab(activeSection: string | null): SettingsTabId {
  return SETTINGS_TABS.some((tab) => tab.id === activeSection)
    ? (activeSection as SettingsTabId)
    : "ai-models";
}

function renderCardGrid(items: Array<{ name: string; desc: string; icon: string; action: string }>) {
  return html`
    <div class="ab-grid">
      ${items.map(
        (item) => html`
          <article class="ab-card">
            <div class="ab-card__head">
              <img class="ab-card__icon" src=${item.icon} alt="" />
              <button class="ab-action ${item.action === "Coming Soon" ? "ab-action--ghost" : ""}">
                ${item.action}
              </button>
            </div>
            <div class="ab-card__title">${item.name}</div>
            <div class="ab-card__desc">${item.desc}</div>
          </article>
        `,
      )}
    </div>
  `;
}

function renderAiModelsTab() {
  const models = [
    ["Arcee AI: Trinity Large Preview (free)", "ctx 131K · reasoning"],
    ["MoonshotAI: Kimi K2.5", "ctx 262K · reasoning"],
    ["Google: Gemini 3 Flash Preview", "ctx 1.0M · reasoning"],
    ["AI21: Jamba Large 1.7", "ctx 256K"],
    ["AllenAI: Olmo 3.1 32B Instruct", "ctx 66K"],
    ["Amazon: Nova 2 Lite", "ctx 1.0M · reasoning"],
    ["Amazon: Nova Lite 1.0", "ctx 300K"],
    ["Amazon: Nova Micro 1.0", "ctx 128K"],
  ];

  return html`
    <section>
      <h2 class="ab-section-title">AI Models</h2>
      <div class="ab-label">Live Model</div>
      <div class="ab-input-like">
        <div class="ab-input-main">Z.ai: GLM 4.5</div>
        <div class="ab-input-sub">OpenRouter · ctx 131K · reasoning</div>
      </div>

      <div class="ab-label ab-gap">Change Model</div>
      <input class="ab-search" placeholder="Search models..." />
      <button class="ab-pill">+ Add Provider</button>

      <div class="ab-provider-title">OPENROUTER</div>
      <div class="ab-model-list">
        ${models.map(
          (model) => html`
            <div class="ab-model-row">
              <span class="ab-model-dot"></span>
              <div>
                <div class="ab-model-name">${model[0]}</div>
                <div class="ab-model-meta">${model[1]}</div>
              </div>
            </div>
          `,
        )}
      </div>
    </section>
  `;
}

function renderOtherTab() {
  return html`
    <section>
      <h2 class="ab-section-title">Other</h2>

      <div class="ab-label">OpenClaw Folder</div>
      <div class="ab-row">
        <span>OpenClaw folder</span>
        <a href="#">Open folder</a>
      </div>
      <div class="ab-help">Contains your local OpenClaw state and app data.</div>

      <div class="ab-label ab-gap">Workspace</div>
      <div class="ab-row">
        <span>Agent workspace</span>
        <a href="#">Open folder</a>
      </div>
      <div class="ab-help">
        Contains editable .md files (AGENTS, SOUL, USER, IDENTITY, TOOLS, HEARTBEAT, BOOTSTRAP) that shape the agent.
      </div>

      <div class="ab-label ab-gap">Terminal</div>
      <div class="ab-group">
        <div class="ab-row">
          <span>Show in sidebar</span>
          <label class="ab-switch"><input type="checkbox" /><span></span></label>
        </div>
        <div class="ab-row"><a href="#">Open Terminal</a></div>
      </div>
      <div class="ab-help">Built-in terminal with openclaw and bundled tools in PATH.</div>

      <div class="ab-label ab-gap">App</div>
      <div class="ab-group">
        <div class="ab-row"><span>Version</span><span>Atomic Bot v1.0.4</span></div>
        <div class="ab-row">
          <span>Auto start</span>
          <label class="ab-switch"><input type="checkbox" /><span></span></label>
        </div>
        <div class="ab-row"><span>License</span><a href="#">PolyForm Noncommercial 1.0.0</a></div>
        <div class="ab-row"><a href="#">Legacy</a></div>
      </div>

      <div class="ab-label ab-gap">About</div>
      <div class="ab-group">
        <div class="ab-row"><span>© 2026 Atomic Bot</span></div>
        <div class="ab-row"><span>Support</span><a href="mailto:support@atomicbot.ai">support@atomicbot.ai</a></div>
      </div>
    </section>
  `;
}

export function renderConfig(props: ConfigProps) {
  const activeTab = resolveActiveTab(props.activeSection);

  return html`
    <div class="ab-settings">
      <header class="ab-header">
        <h1>Settings</h1>
        <nav class="ab-tabs" aria-label="Settings sections">
          ${SETTINGS_TABS.map(
            (tab) => html`
              <button
                class="ab-tab ${activeTab === tab.id ? "is-active" : ""}"
                @click=${() => props.onSectionChange(tab.id)}
              >
                ${tab.label}
              </button>
            `,
          )}
        </nav>
      </header>

      <div class="ab-content">
        ${activeTab === "ai-models" ? renderAiModelsTab() : nothing}
        ${
          activeTab === "ai-providers"
            ? html`
                <section>
                  <h2 class="ab-section-title">Providers & API Keys</h2>
                  ${renderCardGrid(PROVIDERS)}
                </section>
              `
            : nothing
        }
        ${
          activeTab === "messengers"
            ? html`
                <section>
                  <h2 class="ab-section-title">Messengers</h2>
                  ${renderCardGrid(MESSENGERS)}
                </section>
              `
            : nothing
        }
        ${
          activeTab === "skills"
            ? html`
                <section>
                  <div class="ab-title-row">
                    <h2 class="ab-section-title">Skills and Integrations</h2>
                    <button class="ab-custom-skill">+ Add custom skill</button>
                  </div>
                  <input class="ab-search" placeholder="Search by skills..." />
                  ${renderCardGrid(SKILLS)}
                </section>
              `
            : nothing
        }
        ${activeTab === "other" ? renderOtherTab() : nothing}
      </div>
    </div>
  `;
}
