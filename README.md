# Libri Crawler

Go crawler that scrapes supported book sources, sends book metadata to
[`libri-api`](https://github.com/masiama/libri-api), and downloads cover images to
shared local storage.

## Features

- Concurrent scraping, saving, and image downloading
- Structured JSON logging with `slog`
- Internal API integration for batched book upserts
- Shared filesystem storage for downloaded covers

## Supported sources

- `kniga.lv`
- `mnogoknig.com`

## Requirements

- Go 1.26+
- A running [`libri-api`](https://github.com/masiama/libri-api) instance
- An images directory shared with `libri-api`

## Configuration

Set these environment variables before running:

```bash
API_URL=http://localhost:8080
INTERNAL_API_KEY=change-me
IMAGES_DIR=/absolute/path/to/images
```

`API_URL` should point to the
[`libri-api`](https://github.com/masiama/libri-api) instance that exposes the
internal crawler endpoints.

## Running locally

Run the crawler:

```bash
make run
```

## Build

Build the binary:

```bash
make build
```

Clean build output:

```bash
make clean
```

## CLI options

The crawler entrypoint is `cmd/crawler`.

Available flags:

- `--source=<name|all>`
- `--level=<debug|info|warn|error>`

Example:

```bash
go run ./cmd/crawler --source=kniga.lv --level=debug
```
