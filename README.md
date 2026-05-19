# Libri Crawler

Go crawler that scrapes supported book sources, writes crawl events to Redis,
and downloads cover images to shared local storage.

## Features

- Concurrent scraping, Redis publishing, and image downloading
- Structured JSON logging with `slog`
- Redis-backed dedupe and crawl event publishing
- Shared filesystem storage for downloaded covers

## Supported sources

- `kniga.lv`
- `mnogoknig.com`

## Requirements

- Go 1.26+
- A running Redis instance
- An images directory shared with `libri-api`

## Configuration

Set these environment variables before running:

```bash
PORT=8081
REDIS_ADDR=localhost:6379
IMAGES_DIR=/path/to/images
LOG_LEVEL=info # optional: debug | info | warn | error
```

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
