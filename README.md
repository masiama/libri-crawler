# Libri Crawler

Go crawler daemon that scrapes supported book sources, publishes crawl events to
Redis, and downloads cover images to shared local storage.

## Features

- Redis-driven crawl orchestration
- Concurrent scraping, Redis publishing, and image downloading
- Structured JSON logging with `slog`
- Redis-backed dedupe and distributed crawl locking
- Event-driven architecture using Redis queues
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
REDIS_ADDR=localhost:6379
IMAGES_DIR=/path/to/images
LOG_LEVEL=info # optional: debug | info | warn | error
```

## Running locally

Run the crawler daemon:

```bash
make run
```

The crawler listens for commands from Redis queue:

```text
crawler:commands
```

and publishes crawl events to:

```text
crawl:events
```

## Triggering a crawl

Push a crawl command into Redis:

```json
{
  "crawlId": 1,
  "source": "kniga.lv"
}
```

Example using `redis-cli`:

```bash
redis-cli LPUSH crawler:commands \
  '{"crawlId":1,"source":"kniga.lv"}'
```

To crawl multiple sources, enqueue multiple commands.

## Crawl events

The crawler publishes all events using Redis `LPUSH` into `crawl:events`.

### Event types

#### Book event

```json
{
  "type": "book",
  "crawlId": 1,
  "book": {
    ...
  }
}
```

#### Progress event

```json
{
  "type": "progress",
  "crawlId": 1,
  "booksFound": 42
}
```

#### Completed event

```json
{
  "type": "completed",
  "crawlId": 1,
  "booksFound": 120
}
```

#### Error event

```json
{
  "type": "error",
  "crawlId": 1,
  "error": "source mnogoknig.com is already running"
}
```

## Distributed locking

Only one crawl per source can run at a time. Duplicate crawl requests are
rejected and emitted as error events.

## Build

Build the binary:

```bash
make build
```

Clean build output:

```bash
make clean
```
