<p align="center">
  <img src="images/highclaw.png" alt="HighClaw" width="360" />
</p>

<h1 align="center">HighClaw <img src="images/highclaw.png" alt="HighClaw" style="height:1em;width:auto;vertical-align:-0.12em;" /></h1>

<p align="center">
  <a href="./README.zh.md"><img src="https://img.shields.io/badge/📖_中文文档-README.zh.md-0A66C2?style=for-the-badge" height="36" alt="中文文档" /></a>
  &nbsp;
  <a href="./README.md"><img src="https://img.shields.io/badge/📖_English-README.md-2EA043?style=for-the-badge" height="36" alt="English" /></a>
</p>

<p align="center">
  <strong>High performance. Built for speed and reliability. 100% Go. 100% Agnostic.</strong><br>
  ⚡️ <strong>HighClaw keeps full feature coverage with an independent Go implementation.</strong>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT" /></a>
  <a href="https://buymeacoffee.com/argenistherose"><img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-Donate-yellow.svg?style=flat&logo=buy-me-a-coffee" alt="Buy Me a Coffee" /></a>
</p>

Fast, small, and fully autonomous AI assistant infrastructure — deploy anywhere, swap anything.

```
Go binary · modular traits · 22+ providers · pluggable channels/tools/memory · production-ready gateway
```

### ✨ Features

- 🏎️ **High Performance:** Optimized Go runtime with low-overhead startup and stable long-running execution.
- 💰 **Low Deployment Cost:** Single binary deployment for edge devices, VMs, and cloud hosts.
- 🚀 **Deployment Efficiency Advantage:** No Node/Python runtime bootstrap required; install + start in minutes.
- ⚡ **Operationally Reliable:** Strong defaults for gateway auth, memory persistence, and channel safety.
- 🌍 **True Portability:** Cross-platform binaries for macOS, Linux, and Windows (amd64/arm64).

### Why teams pick HighClaw

- **Lean by default:** small Go binary, fast startup, low memory footprint.
- **Secure by design:** pairing, strict sandboxing, explicit allowlists, workspace scoping.
- **Fully swappable:** core systems are traits (providers, channels, tools, memory, tunnels).
- **No lock-in:** OpenAI-compatible provider support + pluggable custom endpoints.

## HighClaw vs OpenClaw

### 1) Positioning and Goals

| Dimension | HighClaw | OpenClaw |
|---|---|---|
| Core Position | High-performance, self-hosted, single-binary-first AI assistant infrastructure (Go implementation) | Feature-rich personal AI assistant platform (Node/TS ecosystem) |
| Goal | Keep feature coverage while improving stability, deployment efficiency, and secure defaults | Deliver a complete user experience with broad ecosystem integrations |
| Typical Scenarios | Backend services, edge devices, low-resource hosts, long-running daemons | Desktop-first use, frontend-centric workflows, deep Node ecosystem integration |

You can think of HighClaw as an engineering-focused Go runtime in the Claw ecosystem: deployable, operable, and extensible.

### 2) Tech Stack and Runtime Shape

| Dimension | HighClaw | OpenClaw |
|---|---|---|
| Main Language | Go | TypeScript (Node.js) |
| Runtime | Native binary | Node.js runtime |
| Execution Form | Single binary + `config.yaml` | Node runtime + JS/TS artifacts |
| Dependency Model | Go modules + minimal system dependencies | Heavier npm ecosystem dependency chain |
| Delivery | `make build` / `make release` for multi-platform artifacts | Usually depends on Node environment and package workflows |

### 3) Resource Efficiency and Operations

| Metric | HighClaw | OpenClaw |
|---|---|---|
| Deployment Complexity | Low (single-file-first) | Medium (Node + dependencies required) |
| Multi-platform Release | Built-in `make release` (linux/darwin/windows, amd64/arm64) | Depends on Node build and packaging toolchain |
| Secure Defaults | Pairing auth, rate limit, workspace scope, command allowlist | Similar security possible, but default path differs |
| Ops Controllability | More backend-observability and daemon-oriented | More application-layer feature oriented |

### Core Positioning (Conclusion)

- HighClaw is not a reduced OpenClaw clone; it is an independent **Go engineering implementation**.
- In the Claw ecosystem, HighClaw focuses on **high-performance deployment, backend stability, low-resource operation, and secure defaults**.
- If your priority is delivery and operations of AI assistant infrastructure, HighClaw is the better fit.

## Quick Start

```bash
git clone https://github.com/903174293/highclaw.git
cd highclaw
make build
make install

# Quick setup (no prompts)
highclaw onboard --api-key sk-... --provider openrouter

# Or interactive wizard
highclaw onboard --interactive

# Or quickly repair channels/allowlists only
highclaw onboard --channels-only

# Chat
highclaw agent -m "Hello, HighClaw!"

# Interactive mode
highclaw agent

# Start the gateway (webhook server)
highclaw gateway                # default: 127.0.0.1:8080
highclaw gateway --port 0       # random port (security hardened)

# Start full autonomous runtime
highclaw daemon

# Check status
highclaw status

# Run system diagnostics
highclaw doctor

# Check channel health
highclaw channel doctor

# Get integration setup details
highclaw integrations info Telegram

# Manage background service
highclaw service install
highclaw service status

# Migrate memory from OpenClaw (safe preview first)
highclaw migrate openclaw --dry-run
highclaw migrate openclaw
```

> **Dev fallback (no global install):** prefix commands with `go run ./cmd/highclaw --` (example: `go run ./cmd/highclaw -- status`).

## Session Flow: `agent -m` + `tui`

HighClaw now uses one unified local session store (`~/.highclaw/sessions`) for both CLI and TUI:

- Every `highclaw agent -m "..."` run is persisted as a **new session**.
- You can bind a message to an existing session: `highclaw agent -m "..." --session <key>`.
- `highclaw tui` loads those sessions in the left sidebar.
- You can switch sessions in TUI and continue chatting from any saved context.
- The sidebar groups sessions by source: **CLI / TUI / OTHER**.
- Sidebar supports live filter: focus sidebar and type keywords.

### Session management commands

```bash
highclaw sessions list
highclaw sessions get <key>
highclaw sessions current
highclaw sessions switch <key>
highclaw sessions reset <key>
highclaw sessions delete <key>
highclaw sessions bindings
highclaw sessions bind <channel> <conversation> <sessionKey>
highclaw sessions unbind <channel> <conversation>
```

### External channels default session policy

- Non-switchable terminals/channels (e.g. WhatsApp/Telegram/webhook/websocket) use a default external session:
  - `agent:main:main`
- You can override per conversation with binding commands:
  - `sessions bind <channel> <conversation> <sessionKey>`
- CLI/TUI keep explicit session switching support.

### TUI key actions

- `Tab`: switch focus between sidebar and input
- `↑` / `↓`: select session in sidebar
- `Enter`: send message (input focus) / open session (sidebar focus)
- `Ctrl+N`: create a new session
- `Ctrl+R`: reload session list
- `Ctrl+C`: quit

### Quick verification

```bash
highclaw agent -m "first message"
highclaw agent -m "second message"
highclaw tui
```

You should see at least two new CLI-created sessions in the TUI sidebar.

## Deployment Playbook (Windows / Ubuntu / CentOS / macOS)

### macOS (Intel/Apple Silicon)

```bash
git clone https://github.com/903174293/highclaw.git
cd highclaw
make build
./dist/highclaw onboard
./dist/highclaw gateway
```

### Ubuntu (20.04/22.04/24.04)

```bash
sudo apt-get update
sudo apt-get install -y make golang-go
git clone https://github.com/903174293/highclaw.git
cd highclaw
make build
sudo make install
highclaw onboard
highclaw daemon
```

### CentOS / RHEL / Rocky

```bash
sudo yum install -y make golang git
git clone https://github.com/903174293/highclaw.git
cd highclaw
make build
sudo make install
highclaw onboard
highclaw daemon
```

### Windows (PowerShell)

```powershell
git clone https://github.com/903174293/highclaw.git
cd highclaw
go build -o dist/highclaw.exe ./cmd/highclaw
.\dist\highclaw.exe onboard
.\dist\highclaw.exe gateway
```

### Why deployment is a product advantage

- Single Go binary delivery minimizes environment drift.
- Fast cold-start and low memory footprint improve edge/server density.
- Same command surface across platforms reduces ops friction.

## Architecture

Every subsystem is a **trait** — swap implementations with a config change, zero code changes.

<p align="center">
  <img src="images/architecture.svg" alt="HighClaw Architecture" width="900" />
</p>

| Subsystem | Trait | Ships with | Extend |
|-----------|-------|------------|--------|
| **AI Models** | `Provider` | 22+ providers (OpenRouter, Anthropic, OpenAI, Ollama, Venice, Groq, Mistral, xAI, DeepSeek, Together, Fireworks, Perplexity, Cohere, Bedrock, etc.) | `custom:https://your-api.com` — any OpenAI-compatible API |
| **Channels** | `Channel` | CLI, Telegram, Discord, Slack, iMessage, Matrix, WhatsApp, Webhook | Any messaging API |
| **Memory** | `Memory` | SQLite with hybrid search (FTS5 + vector cosine similarity), Markdown | Any persistence backend |
| **Tools** | `Tool` | shell, file_read, file_write, memory_store, memory_recall, memory_forget, browser_open (Brave + allowlist), composio (optional) | Any capability |
| **Observability** | `Observer` | Noop, Log, Multi | Prometheus, OTel |
| **Runtime** | `RuntimeAdapter` | Native, Docker (sandboxed) | WASM (planned; unsupported kinds fail fast) |
| **Security** | `SecurityPolicy` | Gateway pairing, sandbox, allowlists, rate limits, filesystem scoping, encrypted secrets | — |
| **Identity** | `IdentityConfig` | OpenClaw (markdown), AIEOS v1.1 (JSON) | Any identity format |
| **Tunnel** | `Tunnel` | None, Cloudflare, Tailscale, ngrok, Custom | Any tunnel binary |
| **Heartbeat** | Engine | HEARTBEAT.md periodic tasks | — |
| **Skills** | Loader | TOML manifests + SKILL.md instructions | Community skill packs |
| **Integrations** | Registry | 50+ integrations across 9 categories | Plugin system |

### Runtime support (current)

- ✅ Supported today: `runtime.kind = "native"` or `runtime.kind = "docker"`
- 🚧 Planned, not implemented yet: WASM / edge runtimes

When an unsupported `runtime.kind` is configured, HighClaw now exits with a clear error instead of silently falling back to native.

### Memory System (Full-Stack Search Engine)

All custom, zero external dependencies — no Pinecone, no Elasticsearch, no LangChain:

| Layer | Implementation |
|-------|---------------|
| **Vector DB** | Embeddings stored as BLOB in SQLite, cosine similarity search |
| **Keyword Search** | FTS5 virtual tables with BM25 scoring |
| **Hybrid Merge** | Custom weighted merge function (`vector.rs`) |
| **Embeddings** | `EmbeddingProvider` trait — OpenAI, custom URL, or noop |
| **Chunking** | Line-based markdown chunker with heading preservation |
| **Caching** | SQLite `embedding_cache` table with LRU eviction |
| **Safe Reindex** | Rebuild FTS5 + re-embed missing vectors atomically |

The agent automatically recalls, saves, and manages memory via tools.

```yaml
memory:
  backend: "sqlite" # "sqlite", "markdown", "none"
  auto_save: true
  embedding_provider: "openai"
  vector_weight: 0.7
  keyword_weight: 0.3
```

## Security

HighClaw enforces security at **every layer** — not just the sandbox. It passes all items from the community security checklist.

### Security Checklist

| # | Item | Status | How |
|---|------|--------|-----|
| 1 | **Gateway not publicly exposed** | ✅ | Binds `127.0.0.1` by default. Refuses `0.0.0.0` without tunnel or explicit `allow_public_bind = true`. |
| 2 | **Pairing required** | ✅ | 6-digit one-time code on startup. Exchange via `POST /pair` for bearer token. All `/webhook` requests require `Authorization: Bearer <token>`. |
| 3 | **Filesystem scoped (no /)** | ✅ | `workspace_only = true` by default. 14 system dirs + 4 sensitive dotfiles blocked. Null byte injection blocked. Symlink escape detection via canonicalization + resolved-path workspace checks in file read/write tools. |
| 4 | **Access via tunnel only** | ✅ | Gateway refuses public bind without active tunnel. Supports Tailscale, Cloudflare, ngrok, or any custom tunnel. |

> **Run your own nmap:** `nmap -p 1-65535 <your-host>` — HighClaw binds to localhost only, so nothing is exposed unless you explicitly configure a tunnel.

### Channel allowlists (Telegram / Discord / Slack)

Inbound sender policy is now consistent:

- Empty allowlist = **deny all inbound messages**
- `"*"` = **allow all** (explicit opt-in)
- Otherwise = exact-match allowlist

This keeps accidental exposure low by default.

Recommended low-friction setup (secure + fast):

- **Telegram:** allowlist your own `@username` (without `@`) and/or your numeric Telegram user ID.
- **Discord:** allowlist your own Discord user ID.
- **Slack:** allowlist your own Slack member ID (usually starts with `U`).
- Use `"*"` only for temporary open testing.

If you're not sure which identity to use:

1. Start channels and send one message to your bot.
2. Read the warning log to see the exact sender identity.
3. Add that value to the allowlist and rerun channels-only setup.

If you hit authorization warnings in logs (for example: `ignoring message from unauthorized user`),
rerun channel setup only:

```bash
highclaw onboard --channels-only
```

### WhatsApp Business Cloud API Setup

WhatsApp uses Meta's Cloud API with webhooks (push-based, not polling):

1. **Create a Meta Business App:**
   - Go to [developers.facebook.com](https://developers.facebook.com)
   - Create a new app → Select "Business" type
   - Add the "WhatsApp" product

2. **Get your credentials:**
   - **Access Token:** From WhatsApp → API Setup → Generate token (or create a System User for permanent tokens)
   - **Phone Number ID:** From WhatsApp → API Setup → Phone number ID
   - **Verify Token:** You define this (any random string) — Meta will send it back during webhook verification

3. **Configure HighClaw:**
   ```yaml
   channels_config:
     whatsapp:
       access_token: "EAABx..."
       phone_number_id: "123456789012345"
       verify_token: "my-secret-verify-token"
       allowed_numbers:
         - "+1234567890" # E.164 format, or ["*"] for all
   ```

4. **Start the gateway with a tunnel:**
   ```bash
   highclaw gateway --port 8080
   ```
   WhatsApp requires HTTPS, so use a tunnel (ngrok, Cloudflare, Tailscale Funnel).

5. **Configure Meta webhook:**
   - In Meta Developer Console → WhatsApp → Configuration → Webhook
   - **Callback URL:** `https://your-tunnel-url/whatsapp`
   - **Verify Token:** Same as your `verify_token` in config
   - Subscribe to `messages` field

6. **Test:** Send a message to your WhatsApp Business number — HighClaw will respond via the LLM.

## Configuration

Config: `~/.highclaw/config.yaml` (created by `onboard`)

```yaml
api_key: "sk-..."
default_provider: "openrouter"
default_model: "anthropic/claude-sonnet-4"
default_temperature: 0.7

memory:
  backend: "sqlite" # "sqlite", "markdown", "none"
  auto_save: true
  embedding_provider: "openai" # "openai", "noop"
  vector_weight: 0.7
  keyword_weight: 0.3

gateway:
  require_pairing: true
  allow_public_bind: false

autonomy:
  level: "supervised" # "readonly", "supervised", "full"
  workspace_only: true
  allowed_commands: ["git", "go", "make", "ls", "cat", "grep"]
  forbidden_paths: ["/etc", "/root", "/proc", "/sys", "~/.ssh", "~/.gnupg", "~/.aws"]

runtime:
  kind: "native" # "native" or "docker"
  docker:
    image: "alpine:3.20"
    network: "none"
    memory_limit_mb: 512
    cpu_limit: 1.0
    read_only_rootfs: true
    mount_workspace: true
    allowed_workspace_roots: []

heartbeat:
  enabled: false
  interval_minutes: 30

tunnel:
  provider: "none" # "none", "cloudflare", "tailscale", "ngrok", "custom"

secrets:
  encrypt: true

browser:
  enabled: false
  allowed_domains: ["docs.rs"]

composio:
  enabled: false

identity:
  format: "openclaw" # "openclaw" or "aieos"
# aieos_path: "identity.json"
# aieos_inline: '{"identity":{"names":{"first":"Nova"}}}'
```

## Identity System (AIEOS Support)

HighClaw supports **identity-agnostic** AI personas through two formats:

### OpenClaw (Default)

Traditional markdown files in your workspace:
- `IDENTITY.md` — Who the agent is
- `SOUL.md` — Core personality and values
- `USER.md` — Who the agent is helping
- `AGENTS.md` — Behavior guidelines

### AIEOS (AI Entity Object Specification)

[AIEOS](https://aieos.org) is a standardization framework for portable AI identity. HighClaw supports AIEOS v1.1 JSON payloads, allowing you to:

- **Import identities** from the AIEOS ecosystem
- **Export identities** to other AIEOS-compatible systems
- **Maintain behavioral integrity** across different AI models

#### Enable AIEOS

```yaml
identity:
  format: "aieos"
  aieos_path: "identity.json" # relative to workspace or absolute path
```

Or inline JSON:

```yaml
identity:
  format: "aieos"
  aieos_inline: |
{
  "identity": {
    "names": { "first": "Nova", "nickname": "N" }
  },
  "psychology": {
    "neural_matrix": { "creativity": 0.9, "logic": 0.8 },
    "traits": { "mbti": "ENTP" },
    "moral_compass": { "alignment": "Chaotic Good" }
  },
  "linguistics": {
    "text_style": { "formality_level": 0.2, "slang_usage": true }
  },
  "motivations": {
    "core_drive": "Push boundaries and explore possibilities"
  }
}
```

#### AIEOS Schema Sections

| Section | Description |
|---------|-------------|
| `identity` | Names, bio, origin, residence |
| `psychology` | Neural matrix (cognitive weights), MBTI, OCEAN, moral compass |
| `linguistics` | Text style, formality, catchphrases, forbidden words |
| `motivations` | Core drive, short/long-term goals, fears |
| `capabilities` | Skills and tools the agent can access |
| `physicality` | Visual descriptors for image generation |
| `interests` | Hobbies, favorites, lifestyle |

See [aieos.org](https://aieos.org) for the full schema and live examples.

## Gateway API

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/health` | GET | None | Health check (always public, no secrets leaked) |
| `/pair` | POST | `X-Pairing-Code` header | Exchange one-time code for bearer token |
| `/webhook` | POST | `Authorization: Bearer <token>` | Send message: `{"message": "your prompt"}` |
| `/whatsapp` | GET | Query params | Meta webhook verification (hub.mode, hub.verify_token, hub.challenge) |
| `/whatsapp` | POST | None (Meta signature) | WhatsApp incoming message webhook |

## Commands

| Command | Description |
|---------|-------------|
| `onboard` | Quick setup (default) |
| `onboard --interactive` | Full interactive 8-step wizard |
| `onboard --channels-only` | Reconfigure channels/allowlists only (fast repair flow) |
| `agent -m "..."` | Single message mode |
| `agent -m "..." --session <key>` | Continue in an existing session |
| `agent` | Interactive chat mode |
| `sessions list/get/current/switch/reset/delete` | Session query + switching + cleanup |
| `sessions bindings/bind/unbind` | External conversation → session routing |
| `gateway` | Start webhook server (default: `127.0.0.1:8080`) |
| `gateway --port 0` | Random port mode |
| `daemon` | Start long-running autonomous runtime |
| `service install/start/stop/status/uninstall` | Manage user-level background service |
| `doctor` | Diagnose daemon/scheduler/channel freshness |
| `status` | Show full system status |
| `channel doctor` | Run health checks for configured channels |
| `integrations info <name>` | Show setup/status details for one integration |

## Development

```bash
make build-dev           # Dev build
make build               # Release build
make test                # Run tests
make lint                # Lint (golangci-lint)
make fmt                 # Format

# Cross-platform build artifacts
make release
```

## Makefile Usage Guide

HighClaw ships with a full Makefile automation flow for development, testing, packaging, installation, and release.

### Common Targets

```bash
make help          # list all targets
make build         # build local binary: dist/highclaw
make build-dev     # dev binary: dist/highclaw-dev
make test          # run tests (race enabled)
make check         # vet + test
make clean         # clean artifacts
```

### Multi-platform Build and Release

```bash
make build-all     # linux/darwin/windows + amd64/arm64
make package       # tar.gz / zip packages to dist/release
make release       # build-all + package
```

Default artifacts:

- `dist/highclaw-linux-amd64`
- `dist/highclaw-darwin-arm64`
- `dist/highclaw-windows-amd64.exe`
- `dist/release/highclaw-<version>-<os>-<arch>.tar.gz|zip`

### Install and Uninstall

```bash
make install       # install to GOBIN / GOPATH/bin / ~/go/bin
make uninstall     # uninstall highclaw
```

Install path priority:

1. `GOBIN`
2. `GOPATH/bin`
3. `~/go/bin`

### Code Quality and Diagnostics

```bash
make fmt           # gofmt format
make vet           # go vet checks
make lint          # golangci-lint (install first)
make doctor        # print version, Go env, host platform, install path
```

### Pre-push hook

A git hook runs `gofmt`, `go vet`, and `go test` before every push. Enable it once:

```bash
git config core.hooksPath .githooks
```

To skip the hook when you need a quick push during development:

```bash
git push --no-verify
```

## Support

HighClaw is an open-source project maintained with passion. If you find it useful and would like to support its continued development, hardware for testing, and coffee for the maintainer, you can support me here:

<a href="https://buymeacoffee.com/argenistherose"><img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-Donate-yellow.svg?style=for-the-badge&logo=buy-me-a-coffee" alt="Buy Me a Coffee" /></a>

## License

MIT — see [LICENSE](LICENSE)
- IP policy — see [INTELLECTUAL_PROPERTY.md](INTELLECTUAL_PROPERTY.md)
- Trademark policy — see [TRADEMARKS.md](TRADEMARKS.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Implement a trait, submit a PR:
- Individual CLA (required) — see [CLA-INDIVIDUAL.md](CLA-INDIVIDUAL.md)
- Security reporting — see [SECURITY.md](SECURITY.md)
- CI workflow guide: [docs/ci-map.md](docs/ci-map.md)
- New `Provider` → `src/providers/`
- New `Channel` → `src/channels/`
- New `Observer` → `src/observability/`
- New `Tool` → `src/tools/`
- New `Memory` → `src/memory/`
- New `Tunnel` → `src/tunnel/`
- New `Skill` → `~/.highclaw/workspace/skills/<name>/`

---

**HighClaw** — High performance. Built for speed and reliability. Deploy anywhere. Swap anything. <img src="images/highclaw.png" alt="HighClaw" style="height:1em;width:auto;vertical-align:-0.12em;" />
