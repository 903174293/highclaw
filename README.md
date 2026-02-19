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
  <a href="https://buymeacoffee.com/z903174293h"><img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-Donate-yellow.svg?style=flat&logo=buy-me-a-coffee" alt="Buy Me a Coffee" /></a>
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

## Session Management — Unified Multi-Session Engine

HighClaw implements a unified local session store (`~/.highclaw/sessions`) shared by **CLI, TUI, and all channels**. Every conversation is tracked, persisted, and switchable — giving you full control over multi-task workflows.

### Core Capabilities

| Feature | Description |
|---------|-------------|
| **Auto-create** | Every `highclaw agent -m "..."` creates a new session automatically |
| **Multi-turn resume** | `highclaw agent -m "..." --session <key>` resumes an existing session with full history |
| **TUI integration** | TUI sidebar shows all sessions from CLI and TUI, with live switching |
| **Session persistence** | All sessions saved as JSON files in `~/.highclaw/sessions/` |
| **Auto-cleanup** | `sessions prune` removes stale sessions (default: 30 days, max 500) |
| **Channel binding** | External channels (Telegram/Discord/etc.) bind to sessions via routing |
| **Memory association** | Every memory entry tagged with `session_key` for per-session recall |

### Session Key Format

```
agent:{agentId}:{sessionName}
```

Examples:
- `agent:main:cli-1234567890` — CLI one-shot session
- `agent:main:session-42601` — TUI interactive session
- `agent:main:main` — Default session for external channels

### CLI Commands

```bash
# Send a single message (creates new session)
highclaw agent -m "Hello!"

# Resume an existing session (multi-turn)
highclaw agent -m "Continue our topic" --session agent:main:cli-1234567890

# Session management
highclaw sessions list                    # List all sessions
highclaw sessions get <key>               # Show session details (JSON)
highclaw sessions current                 # Show current active session
highclaw sessions switch <key>            # Switch active session
highclaw sessions reset <key>             # Clear message history
highclaw sessions delete <key>            # Delete a session permanently

# Session cleanup
highclaw sessions prune                   # Clean up stale sessions (default: 30d / 500 max)
highclaw sessions prune --max-age 7       # Prune sessions idle > 7 days
highclaw sessions prune --max-count 100   # Cap at 100 sessions

# External channel bindings
highclaw sessions bindings                # List all bindings
highclaw sessions bind <ch> <conv> <key>  # Bind channel conversation to session
highclaw sessions unbind <ch> <conv>      # Remove binding
```

### External Channels — DM Scope Routing (Inspired by OpenClaw)

HighClaw implements **4-level DM Scope** for per-sender session isolation, matching OpenClaw's industry-leading design:

| `dmScope` | Session Key Format | Use Case |
|-----------|-------------------|----------|
| `main` | `agent:main:main` | All DMs share one session (legacy mode) |
| `per-peer` | `agent:main:direct:<peerId>` | Each user gets own session, merged across channels |
| `per-channel-peer` | `agent:main:<channel>:direct:<peerId>` | Each user/channel pair isolated (**recommended**) |
| `per-account-channel-peer` | `agent:main:<channel>:<accountId>:direct:<peerId>` | Full isolation for multi-bot setups |

**Group/Channel messages** are always routed by group ID:
```
agent:main:<channel>:<peerKind>:<groupId>
```

**Configuration** (`~/.highclaw/config.yaml`):

```yaml
session:
  scope: per-sender         # 全局启用 per-sender 路由
  dmScope: per-channel-peer # DM 隔离级别
  mainKey: main             # 主会话名
  identityLinks:            # 跨渠道身份合并
    alice:
      - telegram:alice_tg
      - whatsapp:+1234567890
```

**Explicit Binding Override** — CLI/TUI keep explicit session switching support:

```bash
highclaw sessions bind telegram 123456789 agent:main:custom-session
highclaw sessions unbind telegram 123456789
highclaw sessions bindings
```

### TUI Key Actions

| Key | Action |
|-----|--------|
| `Tab` | Switch focus between sidebar and input |
| `↑` / `↓` | Navigate session list |
| `Enter` | Send message (input) / Open session (sidebar) |
| `Ctrl+N` | Create a new session |
| `Ctrl+L` | Clear current view |
| `Ctrl+R` | Reload session list |
| `Ctrl+C` | Quit |

### Quick Verification

```bash
# Create two sessions
highclaw agent -m "first message"
highclaw agent -m "second message"

# Resume the first session
highclaw sessions list
highclaw agent -m "continue first topic" --session <first-session-key>

# Open TUI and see all sessions in sidebar
highclaw tui
```

You should see at least two CLI sessions in the TUI sidebar, with the first having 2 turns of conversation.

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

### Memory System — Industry-Leading Full-Stack Search Engine

**Zero external dependencies. No Pinecone. No Elasticsearch. No LangChain. No Redis. Everything runs inside a single SQLite file.**

HighClaw ships with one of the most complete memory systems in the open-source AI agent space. Most agent frameworks delegate memory to cloud vector databases or heavyweight search engines — HighClaw implements the entire pipeline natively in Go, with **zero network dependencies** for core memory operations.

#### Architecture

<p align="center">
  <img src="images/memory-architecture.svg" alt="HighClaw Memory Architecture" width="900" />
</p>

#### Performance — modernc.org/sqlite In-Process Engine

HighClaw uses **modernc.org/sqlite** — a pure Go translation of SQLite, running in-process with zero CGO and zero system dependencies. This delivers **30–60x performance improvement** over the previous `os/exec` CLI approach, while maintaining perfect cross-compilation support.

| Approach | Latency per Operation | Relative Speed | Trade-off |
|----------|----------------------|----------------|-----------|
| `os/exec` → sqlite3 CLI (old) | 5,000,000–15,000,000 ns (5–15ms) | 1x (baseline) | Process fork overhead per query |
| **modernc.org/sqlite (current)** | **20,000–250,000 ns (20–250μs)** | **30–60x faster** | **Pure Go, zero deps** |
| CGO mattn/sqlite3 (C native) | 10,000–100,000 ns (10–100μs) | 60–150x | Requires gcc/clang, breaks cross-compile |

```
os/exec (old)      ████████████████████████████████████████████████████  5~15ms
modernc (current)  █                                                    20~250μs
CGO (C native)     ▌                                                    10~100μs
```

**Why modernc.org/sqlite over CGO?**

| Dimension | modernc.org/sqlite | CGO mattn/sqlite3 |
|-----------|-------------------|-------------------|
| Performance gap | Only 1–2.5x slower | Baseline |
| Cross-compilation | ✅ Works everywhere | ❌ Needs C cross-compile toolchain |
| Build dependencies | ✅ Zero (pure Go) | ❌ Requires gcc/clang |
| Single binary | ✅ Guaranteed | ⚠️ Dynamic linking or static compile |
| CI/CD complexity | ✅ Minimal | ❌ Matrix builds per platform |

**Conclusion:** modernc.org/sqlite delivers the best cost-performance ratio — only 1–2.5x slower than raw C, but with zero engineering overhead.

#### Full-Stack Capabilities

| Layer | Implementation | Why it matters |
|-------|---------------|----------------|
| **Hybrid Search** | FTS5 keyword (BM25) + vector cosine similarity, weighted merge | Better recall than keyword-only or vector-only — catches both exact terms and semantic meaning |
| **FTS5 content= Mode** | Zero-redundancy FTS5 with 3 auto-sync triggers (INSERT/DELETE/UPDATE) | Index stays in sync without duplicating data — same approach as ZeroClaw |
| **BM25 Normalization** | Raw BM25 scores normalized to [0,1] range | Consistent scoring across different query lengths and document sizes |
| **CJK Fallback** | FTS5 → LIKE auto-fallback for Chinese/Japanese/Korean | Most FTS engines silently fail on CJK; HighClaw handles it transparently |
| **Vector DB** | Embeddings stored as BLOB in SQLite, computed cosine similarity | No external vector database needed — zero infra cost |
| **Embedding Provider** | OpenAI-compatible API, custom URL, or noop | Works offline (noop), or plug any embedding service |
| **Batch Embedding** | `embedBatch()` API — 100 texts per API call | 50–100x faster reindexing; reduces API round-trips dramatically |
| **Embedding Cache** | SQLite `embedding_cache` table with LRU eviction (default: 10,000 entries) | Avoids redundant API calls, saves cost and latency |
| **Keyword Search** | FTS5 virtual tables with BM25 scoring | Fast, battle-tested full-text search built into SQLite |
| **Markdown Chunker** | Heading-aware document splitter with configurable token limits | Preserves document structure, respects heading boundaries |
| **Memory Hygiene** | Auto-archive (7d), auto-purge (30d), conversation retention pruning, 12h throttle | Self-maintaining — no manual cleanup needed |
| **Safe Reindex** | Rebuild FTS5 + batch re-embed missing vectors atomically | Zero downtime index rebuilds via `highclaw memory sync` |
| **Session-Aware** | Every memory entry tagged with `session_key`, `channel`, `sender` | Cross-session recall with per-session isolation when needed |
| **Dual Backend** | SQLite (full-featured) + Markdown (append-only, human-readable) | Choose power or simplicity; `none` falls back to Markdown safely |
| **Parameterized Queries** | `database/sql` with `?` placeholders throughout | SQL injection eliminated; WAL mode + busy_timeout for concurrency |
| **CLI Access** | `memory search / get / list / status / sync / reset` | Full memory inspection without code — debug and verify in seconds |

#### Comparison with ZeroClaw Memory System

| Capability | HighClaw (Go) | ZeroClaw (Rust) | Winner |
|---|---|---|---|
| SQLite backend (in-process) | ✅ modernc.org/sqlite | ✅ rusqlite | Tie |
| FTS5 + content= + triggers | ✅ | ✅ | Tie |
| Vector search + cosine similarity | ✅ | ✅ | Tie |
| Hybrid merge (weighted fusion) | ✅ | ✅ | Tie |
| Batch embedding API | ✅ 100/batch | ❌ One-by-one | **HighClaw** |
| CJK auto-fallback | ✅ FTS5 → LIKE | ❌ | **HighClaw** |
| Session-aware storage | ✅ Full implementation | ⚠️ Field exists, unused | **HighClaw** |
| Complete CLI memory ops | ✅ 6 commands | ⚠️ Limited | **HighClaw** |
| Cross-compile (no C deps) | ✅ Pure Go | ❌ Needs C toolchain | **HighClaw** |
| Raw SQLite performance | 20–250μs | 10–100μs | **ZeroClaw** |

#### Comparison with Other Agent Frameworks

| Capability | HighClaw | LangChain | AutoGPT | CrewAI |
|---|---|---|---|---|
| Hybrid Search (keyword + vector) | ✅ Native | ❌ Requires Pinecone/Weaviate | ❌ | ❌ |
| Zero external dependencies | ✅ Single SQLite file | ❌ Requires vector DB service | ❌ Requires Redis | ❌ Requires ChromaDB |
| In-process SQLite (no CLI) | ✅ modernc.org/sqlite | ❌ | ❌ | ❌ |
| CJK full-text search | ✅ Auto-fallback | ❌ | ❌ | ❌ |
| Batch embedding API | ✅ 100/batch | ❌ | ❌ | ❌ |
| Embedding cache with LRU | ✅ Built-in | ❌ | ❌ | ❌ |
| Memory hygiene (auto-cleanup) | ✅ Archive + Purge + Prune | ❌ Manual | ❌ Manual | ❌ |
| Session-aware memory | ✅ Per-session tagging | ❌ | ❌ | ❌ |
| CLI memory inspection | ✅ search/get/list/status/sync/reset | ❌ | ❌ | ❌ |
| Offline operation | ✅ Works without network | ❌ | ❌ | ❌ |
| Single binary deployment | ✅ | ❌ | ❌ | ❌ |
| Parameterized queries (no SQL injection) | ✅ | N/A | N/A | N/A |
| Parameterized queries (no SQL injection) | ✅ | N/A | N/A | N/A |

#### Configuration

```yaml
memory:
  backend: "sqlite"            # "sqlite" | "markdown" | "none"
  autoSave: true               # auto-save user/assistant messages
  hygieneEnabled: true          # auto archive + purge + prune
  archiveAfterDays: 7           # archive daily files after N days
  purgeAfterDays: 30            # purge archives after N days
  conversationRetentionDays: 30 # prune old conversation entries
  embeddingProvider: "openai"   # "none" | "openai" | "custom:https://..."
  embeddingModel: "text-embedding-3-small"
  embeddingDimensions: 1536
  vectorWeight: 0.7             # hybrid search: vector weight
  keywordWeight: 0.3            # hybrid search: keyword weight
  embeddingCacheSize: 10000     # LRU cache capacity
  chunkMaxTokens: 512           # markdown chunker token limit
```

#### Quick Demo

```bash
# Search memory (supports Chinese, English, mixed queries)
highclaw memory search "项目架构"
highclaw memory search "deployment strategy"

# Inspect a specific entry
highclaw memory get user_msg_abc123

# List all entries, filtered by category
highclaw memory list --category conversation --limit 10

# Check memory health
highclaw memory status

# Rebuild search index
highclaw memory sync
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

### Sandbox Policy (Workspace Scope)

By default, HighClaw restricts all file and shell operations to the workspace directory (`~/.highclaw/workspace`). This prevents accidental access to sensitive system paths like `~/Desktop`, `/etc`, or `~/.ssh`.

**Default behavior:** `workspaceOnly: true` (secure by default, matches ZeroClaw)

#### Configuration Methods

**Method 1: Permanent configuration (recommended)**

```bash
# Check current setting
highclaw config get autonomy.workspaceOnly
# Output: true

# Allow access to absolute paths (e.g., ~/Desktop)
highclaw config set autonomy.workspaceOnly false

# Restore restriction
highclaw config set autonomy.workspaceOnly true
```

**Method 2: Temporary override (single command)**

```bash
# Only affects this command, does not modify config file
highclaw agent -m "Organize desktop images" --no-sandbox
```

**Method 3: Edit config file directly**

```yaml
# ~/.highclaw/config.yaml
autonomy:
  level: supervised
  workspaceOnly: false  # Allow access to absolute paths
  allowedCommands:      # Additional commands to allow (merged with defaults)
    - docker
    - kubectl
  forbiddenPaths:       # Always blocked even when workspaceOnly=false
    - /etc
    - /root
```

#### Behavior Summary

| Setting | Behavior |
|---------|----------|
| `workspaceOnly: true` (default) | Operations restricted to workspace; absolute paths blocked |
| `workspaceOnly: false` | Allow access to absolute paths (e.g., `~/Desktop`) |
| `--no-sandbox` flag | Temporary bypass for a single command |

#### Approval Mechanism

For medium/high-risk commands, HighClaw uses a **model self-approval** mechanism (same as ZeroClaw):

- Shell tool accepts an `approved: bool` parameter
- When executing risky commands, the model must set `"approved": true` in the tool call
- If not set, the command returns an error, and the model retries with approval

This is **not** manual user approval — the AI model decides whether to approve based on context and risk assessment.

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
| `onboard --interactive` | Full interactive 9-step wizard |
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
| `skills list` | List all installed skills (open-skills + workspace) |
| `skills install <url\|path>` | Install a skill from GitHub URL or local path |
| `skills uninstall <name>` | Remove an installed skill |
| `skills status` | Show skills summary and counts |

## Skills System

HighClaw features a pure-Go, SKILL.md-driven skill system that extends your agent's capabilities with specialized knowledge — zero Node.js or npm dependencies.

<p align="center">
  <img src="images/skill-architecture.svg" alt="Skill Architecture" width="100%"/>
</p>

### How Skills Work

Skills are Markdown files (`SKILL.md`) that contain instructions, prompts, and context. On every conversation turn, all active skills are injected into the agent's system prompt, giving it specialized knowledge and behavior.

```
Skill Sources                    Skill Manager                   Agent Runtime
─────────────                    ─────────────                   ─────────────
~/open-skills/skills/  ──┐       LoadAll()                       buildSystemPrompt()
                         ├──►    loadSkillFromMD()  ──►          ToSystemPrompt()
<workspace>/skills/    ──┘       Install / Remove                → active in all turns
```

### Skill Directory Structure

```
~/open-skills/skills/           # Community skills (auto-synced)
  ├── chat-logger/SKILL.md
  ├── web-search/SKILL.md
  ├── browser-automation-agent/SKILL.md
  └── ... (19 community skills)

<workspace>/skills/             # Your custom skills
  ├── my-custom-skill/SKILL.md
  └── another-skill/SKILL.md
```

### CLI Commands

```bash
# List all installed skills
highclaw skills list

# Show skills summary
highclaw skills status

# Install from GitHub
highclaw skills install https://github.com/user/my-skill

# Install from local path
highclaw skills install /path/to/local/skill

# Remove a skill
highclaw skills uninstall my-skill
```

### Creating a Custom Skill

1. Create a directory under `<workspace>/skills/`:

```bash
mkdir -p ~/.highclaw/workspace/skills/my-skill
```

2. Write a `SKILL.md` file:

```markdown
# My Custom Skill

This skill teaches the agent specialized behavior for code review.

## Instructions

When the user asks for a code review:
1. Check for security vulnerabilities first
2. Then check for performance issues
3. Finally suggest style improvements
```

3. Verify it loads:

```bash
highclaw skills list
# Should show: my-skill v0.1.0
```

### Open-Skills Community Repository

HighClaw automatically syncs the [open-skills](https://github.com/besoeasy/open-skills) community repository:

- **Location**: `~/open-skills/`
- **Sync interval**: Every 7 days (auto `git pull`)
- **First run**: Auto `git clone --depth 1`
- **Disable**: `export HIGHCLAW_OPEN_SKILLS_ENABLED=false`

### Onboard Integration

The `highclaw onboard --interactive` wizard includes Step 8/9 "Skills Configuration" where you can:
- View all loaded skills
- Install additional skills from URL or local path

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `HIGHCLAW_OPEN_SKILLS_ENABLED` | `true` | Enable/disable open-skills auto-sync |

Skills directory paths:
- Open-skills: `~/open-skills/skills/`
- Workspace skills: `<workspace>/skills/`


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

<a href="https://buymeacoffee.com/z903174293h"><img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-Donate-yellow.svg?style=for-the-badge&logo=buy-me-a-coffee" alt="Buy Me a Coffee" /></a>

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
