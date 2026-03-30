# Libri Crawler

This service is the data acquisition engine for the Libri ecosystem. It extracts book metadata and cover images from external sources, sending books to [libri-api](https://github.com/masiama/libri-api) and storing images on local disk.

### Features

- **Parallel Processing**: Separate worker pools scrape pages and download images simultaneously for high throughput.
- **Flexible Storage**: Interface-driven design supports both local disk and S3-compatible (Cloudflare R2) storage. Local storage is the default.
- **Manual Extraction**: Uses `net/http` and `htmlquery` (XPath) for precise, low-memory data mining.
- **Structured Logging**: JSON log output via `slog` for machine-readable logs and Kotlin-side monitoring.
- **Reliability**: Context-aware workers ensure timeouts and graceful shutdowns to prevent hanging processes.

### Supported Sources

- kniga.lv
- mnogoknig.com
- _More sources coming soon_

---

### Getting Started

**1. Configuration**
Copy the template and add credentials:

```bash
cp .env.example .env
```

**2. Installation**

```bash
go mod tidy
```

**3. Execution**

```bash
make run
```

### Architecture

The crawler does not write to the database directly. Scraped books are sent in batches to `libri-api` via an internal HTTP endpoint. Images are downloaded to a local directory shared with `libri-api` via a Docker volume.
