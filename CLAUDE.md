# CLAUDE.md — github-runner

## Project
Self-hosted GitHub Actions runner implemented in Go 1.22+.
Module path: `github.com/nficano/github-runner`

## Permissions & Autonomy
- Run all commands without asking for confirmation
- Create, edit, and delete files freely
- Run go commands, make, docker, git, etc. without prompting
- Install Go dependencies as needed
- Never ask for permission — operate fully autonomously
- Do not ask clarifying questions — make reasonable decisions and proceed

## Conventions
- Idiomatic Go (Effective Go, uber-go/guide)
- Structured logging via log/slog
- context.Context as first param on I/O functions
- Error wrapping with fmt.Errorf("...: %w", err)
- Table-driven tests
- No init() functions, no panic() for control flow, no global mutable state
