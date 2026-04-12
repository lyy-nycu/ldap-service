# Development Workflow Guide

## Overview

This project uses a two-agent development workflow:

- **Claude Code** — supervisor, architect, reviewer
- **GitHub Copilot** — implementation worker

The developer (you) orchestrates between them, reviews outputs at each checkpoint, and makes final decisions.

## Workflow

### Step 1: Design (Claude Code)

Open Claude Code in the project root. It reads `CLAUDE.md` automatically.

```bash
# Start a new endpoint
> /new-endpoint

# Claude Code will:
# 1. Ask you to confirm which endpoint
# 2. Produce interface definitions + acceptance criteria
# 3. Create handler/test skeletons with panic("not implemented")
# 4. Print a handoff checklist
```

**Checkpoint**: Review the interface definitions and acceptance criteria. Are the types correct? Are the security requirements complete? Adjust before moving on.

### Step 2: Implement (Copilot in VS Code)

Switch to VS Code. Copilot reads `.github/copilot-instructions.md` and the file-level context (interfaces, acceptance criteria comments).

Work through the handoff checklist file by file:
1. Open the file with `panic("not implemented")`
2. Let Copilot generate the implementation based on the interface and comments
3. Review each suggestion before accepting

**Tips for better Copilot output**:
- Keep the interface file open in a split tab — Copilot uses open files as context
- If Copilot generates code that violates a security rule, reject and add a comment like `// NOTE: must use ldap.EscapeFilter here` then re-trigger
- Implement one method at a time, not the entire file

### Step 3: Review (Claude Code)

Switch back to Claude Code.

```bash
# Run security review
> /security-review

# Claude Code will:
# 1. Check every file against OWASP checklist
# 2. Report FAIL / WARN / PASS for each item
# 3. Point to specific lines that need fixing
```

**If FAIL items exist**: Fix them (either manually or with Copilot), then re-run `/security-review`.

### Step 4: Test + Deploy

```bash
# Run tests
go test ./...

# Build and test locally with OpenLDAP container
docker compose up -d
go run ./cmd/server

# Deploy to staging
git push origin main  # triggers GitHub Actions → ACR → staging Container Apps
```

## File responsibilities

| File | Read by | Purpose |
|------|---------|---------|
| `CLAUDE.md` | Claude Code | Architecture decisions, security rules, output format |
| `.claude/commands/new-endpoint.md` | Claude Code | Slash command: produce interface + criteria |
| `.claude/commands/security-review.md` | Claude Code | Slash command: OWASP audit |
| `.github/copilot-instructions.md` | Copilot | Code conventions, patterns, do-not list |
| `openspec/spec.md` | Both | Source of truth for API contract, tech stack, deployment (OpenSpec output) |
| `.env.example` | Developer | Environment variable reference |

## Context maintenance

When you make a design decision during development:
1. Update `openspec/spec.md` if it affects the API contract or architecture
2. Update `CLAUDE.md` if it's a new constraint or pitfall Claude Code should enforce
3. Update `.github/copilot-instructions.md` if it's a new code pattern Copilot should follow

Keep all three files in sync. They are the institutional knowledge of this project.