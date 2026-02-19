# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| main    | :white_check_mark: |

## Reporting a Vulnerability

**Do NOT open a public GitHub issue for security vulnerabilities.**

Please report them responsibly:

1. **GitHub Security Advisories:** Use [GitHub Security Advisories](https://github.com/903174293/highclaw/security/advisories/new)
2. **Private Contact:** Reach maintainers via the repository profile

### What to Include

- Affected component and version/commit
- Steps to reproduce
- Impact assessment and exploitability
- Suggested fix (if available)

### Response Timeline

- **Acknowledgment:** Within 72 hours
- **Triage Decision:** Within 7 days
- **Fix Timeline:** Based on severity — critical issues within 2 weeks

## Security Architecture

HighClaw implements defense-in-depth security across multiple layers:

### Autonomy Levels

- **ReadOnly** — Agent can only read; no shell or write access
- **Supervised** — Agent acts within configured allowlists (default)
- **Full** — Agent has full access within workspace sandbox

### Sandboxing Layers

1. **Workspace isolation** — All file operations confined to workspace directory
2. **Path traversal blocking** — `..` sequences, absolute paths, and null bytes rejected
3. **Symlink escape detection** — Canonicalization + resolved-path workspace checks
4. **Command allowlisting** — Only explicitly approved commands can execute
5. **Forbidden path list** — Critical system paths (`/etc`, `/root`, `~/.ssh`, `~/.aws`) always blocked
6. **Rate limiting** — Max actions per interval and cost caps

### What We Protect Against

- Path traversal attacks (`../../../etc/passwd`)
- Command injection (`rm -rf /`, `curl | sh`)
- Workspace escape via symlinks or absolute paths
- Null byte injection attacks
- Unauthorized shell command execution
- Runaway costs from LLM API calls

## Security Testing

Security mechanisms are covered by automated tests:

```bash
go test ./... -run Security
go test ./... -run Sandbox
go test ./internal/agent -run Policy
```

## Gateway Security

| Control | Implementation |
|---------|----------------|
| **Localhost binding** | Gateway binds `127.0.0.1` by default; refuses `0.0.0.0` without tunnel or explicit opt-in |
| **Pairing required** | 6-digit one-time code on startup; bearer token for all authenticated endpoints |
| **Tunnel enforcement** | Public exposure requires active tunnel (Tailscale, Cloudflare, ngrok) |

## Container Security (Docker Runtime)

When using `runtime.kind = "docker"`:

| Control | Implementation |
|---------|----------------|
| **Network isolation** | `network: none` by default |
| **Memory limits** | Configurable `memory_limit_mb` |
| **CPU limits** | Configurable `cpu_limit` |
| **Read-only rootfs** | `read_only_rootfs: true` supported |
| **Minimal base image** | Recommended Alpine or distroless |

## Channel Security

- Empty allowlist = **deny all inbound messages** (secure by default)
- `"*"` = allow all (explicit opt-in only)
- Otherwise = exact-match sender allowlist

## Secrets Management

- Secrets stored encrypted when `secrets.encrypt: true`
- API keys never logged in plaintext
- Sensitive fields redacted in diagnostics output
