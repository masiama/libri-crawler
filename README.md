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
PORT=8081
API_URL=http://localhost:8080
INTERNAL_API_KEY=change-me
IMAGES_DIR=/path/to/images
LOG_LEVEL=info # optional: debug | info | warn | error
```

`API_URL` should point to the
[`libri-api`](https://github.com/masiama/libri-api) instance that exposes the
internal crawler endpoints.

## Running locally

Run the crawler server:

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

## Server

Trigger a crawl with:

```bash
curl -X POST "http://localhost:8081/crawl"
```

To run a specific source:

```bash
curl -X POST "http://localhost:8081/crawl?source=kniga.lv"
```
