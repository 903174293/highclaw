# Contributing to HighClaw

Thanks for contributing.

## Development Setup

```bash
git clone https://github.com/903174293/highclaw.git
cd highclaw
make build
go test ./...
```

## Required Checks

Run before creating a PR:

```bash
gofmt -w .
go vet ./...
go test ./...
```

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

## CLA Requirement (Individual)

In addition to DCO sign-off, contributors must accept the individual CLA:

- [CLA-INDIVIDUAL.md](CLA-INDIVIDUAL.md)

Add this line to PR description (or first PR comment):

```text
I have read and agree to the HighClaw Individual CLA (CLA-INDIVIDUAL.md).
```

PRs without clear CLA acceptance may be closed or delayed.

## Contribution Rules

- Keep PR scope focused (one concern per PR).
- Do not include unlicensed third-party code or assets.
- If importing third-party snippets/assets, include attribution and license details in the PR.
- CLA acceptance is mandatory for all external contributions.

## Security-Sensitive Changes

Changes under auth, gateway, command execution, or filesystem boundaries should include:

- Threat/risk notes
- Failure-mode tests
- Rollback notes

See `SECURITY.md` for vulnerability reporting.
