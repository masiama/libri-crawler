# Contributing to libri-crawler

## Prerequisites

- Go 1.26+
- Redis (local instance for development)
- golangci-lint — see
  [installation guide](https://golangci-lint.run/docs/welcome/install/)

## Setup

```bash
git clone https://github.com/masiama/libri-crawler
cd libri-crawler
go mod download
cp .env.example .env  # fill in your local values
```

## Development

Run locally:

```bash
make run
```

Run checks before submitting a PR:

```bash
make check
```

This runs `go mod tidy`, `go fmt`, `go vet`, and `golangci-lint`. CI will fail
if any of these don't pass.

## Pull Request Guidelines

- One feature or fix per PR
- Keep PRs small and focused
- CI must pass before merging
- Write meaningful commit messages

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/). Quick
reference: https://gist.github.com/qoomon/5dfcdf8eec66a051ecd85625518cfd13

Examples:

```
feat: add retry logic for transient HTTP errors
fix: release redis lock on stale job cleanup
refactor: extract url cache to separate package
```
