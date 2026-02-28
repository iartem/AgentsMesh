# Contributing to AgentsMesh

Thanks for your interest in contributing.

## Before You Start

- Read the [README](./README.md) for project overview and local setup.
- By contributing, you agree that your contributions are licensed under this repository's [BSL 1.1](./LICENSE).

## Development Setup

1. Start local dependencies and seed data:

```bash
./deploy/dev/dev.sh
```

2. Start frontend locally (in a separate terminal):

```bash
cd web
pnpm install
pnpm dev
```

## Build and Test

Run relevant checks before opening a PR.

### Backend

```bash
cd backend
go test ./...
```

### Web

```bash
cd web
pnpm install --frozen-lockfile
pnpm run lint
pnpm run type-check
pnpm run test:coverage
```

### Runner

```bash
cd runner
go test ./...
go build ./cmd/runner
```

## Pull Request Guidelines

- Keep PRs focused and small where possible.
- Include context: what changed, why, and any tradeoffs.
- Add or update tests for behavior changes.
- Update docs when user-facing behavior or configuration changes.
- Ensure CI is green before requesting review.

## Commit Messages

Use clear, descriptive commit messages that explain intent.

## Reporting Bugs and Requesting Features

- Use GitHub Issues with the provided templates.
- For security-sensitive reports, see [SECURITY.md](./SECURITY.md).
