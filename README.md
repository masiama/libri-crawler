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
REDIS_URL=redis://localhost:6379
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

The crawler publishes all events via Redis `LPUSH` into the `crawler:events`
queue.

### Event types

#### Book event

Fired when a new book is successfully scraped.

```json
{
	"type": "book",
	"crawlId": 1,
	"book": {
		// ...
	}
}
```

#### Progress event

Fired periodically to broadcast the current count of books found during the
crawl.

```json
{
	"type": "progress",
	"crawlId": 1,
	"booksFound": 42
}
```

#### Completed event

Fired when the crawling process finishes successfully.

```json
{
	"type": "completed",
	"crawlId": 1,
	"booksFound": 120
}
```

#### Cancelled event

Fired if the crawl job is explicitly aborted before finishing.

```json
{
	"type": "cancelled",
	"crawlId": 1,
	"booksFound": 15
}
```

#### Error event

Fired when a global or system-level error stops the crawl entirely (e.g., source
locking issues).

```json
{
	"type": "error",
	"crawlId": 1,
	"error": "source mnogoknig.com is already running"
}
```

#### Crawl error event

Fired when a non-fatal or page-specific error occurs while processing a specific
URL.

```json
{
	"type": "crawl_error",
	"crawlId": 1,
	"error": "failed to parse page body: timeout",
	"url": "https://example.com/book/123"
}
```

#### Heartbeat Event

Fired periodically to signal that the crawler instance is alive and actively
working on the task.

```json
{
	"type": "heartbeat",
	"crawlId": 1
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

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md).
