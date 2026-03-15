# Libri Crawler

This service is the data acquisition engine for the Libri ecosystem. It extracts book metadata and covers from external providers and persists them to a database and object storage.

### Features

- **Parallel Processing**: Separate worker pools scrape pages and download images simultaneously for high throughput.
- **Flexible Storage**: Interface-driven design supports both local disk and S3-compatible (Cloudflare R2) storage.
- **Manual Extraction**: Uses `net/http` and `htmlquery` (XPath) for precise, low-memory data mining.
- **Data Integrity**: Ent ORM handles schema management with additional logic for ISBN validation.
- **Reliability**: Context-aware workers ensure timeouts and graceful shutdowns to prevent hanging processes.

### Supported Sources

- kniga.lv
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
