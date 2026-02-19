# Contributing to HighClaw

Thanks for your interest in contributing to HighClaw! This guide will help you get started.

## Development Setup

```bash
# Clone the repo
git clone https://github.com/903174293/highclaw.git
cd highclaw

# Enable pre-push hook (runs fmt, vet, tests before every push)
git config core.hooksPath .githooks

# Build
make build

# Run tests (all must pass)
go test ./...

# Format & lint (must pass before PR)
gofmt -w .
go vet ./...
```

### Pre-push Hook

The repo includes a pre-push hook in `.githooks/` that enforces `gofmt`, `go vet`, and `go test` before every push. Enable it with:

```bash
git config core.hooksPath .githooks
```

To skip during rapid iteration:

```bash
git push --no-verify
```

> **Note:** CI runs the same checks, so skipped hooks will be caught on the PR.

## IP and Sign-off Requirement (DCO)

All commits must include a sign-off line:

```text
Signed-off-by: Your Name <you@example.com>
```

Use:

```bash
git commit -s -m "feat: your change"
```

By signing off, you certify the Developer Certificate of Origin (DCO 1.1): you have the right to submit the work and agree to license it under this repository's license.

See [DCO](DCO) for the full text.

## CLA Requirement (Individual)

In addition to DCO sign-off, contributors must accept the individual CLA:

- [CLA-INDIVIDUAL.md](CLA-INDIVIDUAL.md)

Add this line to PR description (or first PR comment):

```text
I have read and agree to the HighClaw Individual CLA (CLA-INDIVIDUAL.md).
```

PRs without clear CLA acceptance may be closed or delayed.

## High-Volume Collaboration Rules

When PR traffic is high (especially with AI-assisted contributions), these rules keep quality and throughput stable:

- **One concern per PR:** Avoid mixing refactor + feature + infra in one change.
- **Small PRs first:** Prefer PR size `XS/S/M`; split large work into stacked PRs.
- **Template is mandatory:** Complete every section in `.github/pull_request_template.md`.
- **Explicit rollback:** Every PR must include a fast rollback path.
- **Security-first review:** Changes in `internal/agent/policy.go`, gateway, and auth need stricter validation.

## Agent Collaboration Guidance

Agent-assisted contributions are welcome and treated as first-class contributions.

For smoother agent-to-agent and human-to-agent review:

- Keep PR summaries concrete (problem, change, non-goals).
- Include reproducible validation evidence (`fmt`, `vet`, `test`, scenario checks).
- Add brief workflow notes when automation materially influenced design/code.
- Call out uncertainty and risky edges explicitly.

We do **not** require PRs to declare an AI-vs-human line ratio.

## Architecture: Interface-Based Pluggability

HighClaw's architecture is built on **interfaces** — every subsystem is swappable. Contributing a new integration is as simple as implementing an interface and registering it.

```
internal/
├── agent/providers/   # LLM backends     → Provider interface
├── channels/          # Messaging        → Channel interface
├── infrastructure/    # Platform adapters
├── skills/            # Skill loader
└── gateway/           # Session routing
```

## How to Add a New Provider

Create `internal/agent/providers/your_provider.go`:

```go
package providers

import "context"

type YourProvider struct {
    apiKey string
    // ... other fields
}

func NewYourProvider(apiKey string) *YourProvider {
    return &YourProvider{apiKey: apiKey}
}

func (p *YourProvider) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error) {
    // Your API call here
    return nil, nil
}

func (p *YourProvider) Name() string {
    return "your_provider"
}
```

Then register it in the provider factory.

## How to Add a New Channel

Create `internal/infrastructure/channels/your_channel/your_channel.go`:

```go
package yourchannel

import "context"

type YourChannel struct {
    // config fields
}

func (c *YourChannel) Name() string {
    return "your_channel"
}

func (c *YourChannel) Send(ctx context.Context, message, recipient string) error {
    // Send message via your platform
    return nil
}

func (c *YourChannel) Listen(ctx context.Context) (<-chan IncomingMessage, error) {
    // Listen for incoming messages
    return nil, nil
}

func (c *YourChannel) HealthCheck(ctx context.Context) bool {
    return true
}
```

## Pull Request Checklist

- [ ] PR template sections are completed (including security + rollback)
- [ ] `gofmt -d .` — code is formatted (no diff output)
- [ ] `go vet ./...` — no warnings
- [ ] `go test ./...` — all tests pass
- [ ] New code has `*_test.go` tests where appropriate
- [ ] No new dependencies unless clearly justified
- [ ] README updated if adding user-facing features
- [ ] Follows existing code patterns and conventions
- [ ] DCO sign-off present on all commits
- [ ] CLA acceptance stated in PR

## Commit Convention

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add Anthropic provider
feat(provider): add Anthropic provider
fix: path traversal edge case with symlinks
docs: update contributing guide
test: add heartbeat unicode parsing tests
refactor: extract common security checks
chore: bump dependencies
```

Recommended scope keys:

- `provider`, `channel`, `memory`, `security`, `gateway`, `cli`, `docs`, `tests`

## Code Style

- **Minimal dependencies** — every module adds complexity; justify additions
- **Table-driven tests** — prefer `*_test.go` with table-driven patterns
- **Interface-first** — define the interface, then implement
- **Security by default** — sandbox everything, allowlist, never blocklist
- **No panic in production** — use error returns; reserve panic for truly unrecoverable states
- **Context propagation** — pass `context.Context` through call chains

## Reporting Issues

- **Bugs:** Include OS, Go version, steps to reproduce, expected vs actual
- **Features:** Describe the use case, propose which interface to extend
- **Security:** See [SECURITY.md](SECURITY.md) for responsible disclosure

## Security-Sensitive Changes

Changes under auth, gateway, command execution, or filesystem boundaries should include:

- Threat/risk notes
- Failure-mode tests
- Rollback notes

See [SECURITY.md](SECURITY.md) for vulnerability reporting.

## Maintainer Merge Policy

- Require passing CI checks before merge.
- Require review approval for non-trivial changes.
- Require CODEOWNERS review for protected paths.
- Prefer squash merge with conventional commit title.
- Revert fast on regressions; re-land with tests.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

See also:
- [LICENSE](LICENSE) — MIT License
- [INTELLECTUAL_PROPERTY.md](INTELLECTUAL_PROPERTY.md) — IP policy
- [TRADEMARKS.md](TRADEMARKS.md) — Trademark policy
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) — Community guidelines
