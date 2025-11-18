# go-jira-scraper

A lightweight, efficient Go-based JIRA scraper that fetches all issues and their complete history from specified JIRA projects with aggressive disk caching to minimize API calls.

## Features

- ✅ Complete issue and changelog/history scraping
- ✅ Efficient disk caching (JSON files)
- ✅ Smart incremental updates (only fetch changed issues)
- ✅ Concurrent fetching with worker pools
- ✅ Rate limiting and retry logic
- ✅ Simple CLI with subcommands

## Installation

```bash
go install github.com/jctanner/go-jira-scraper/cmd/go-jira-scraper@latest
```

Or build from source:

```bash
git clone https://github.com/jctanner/go-jira-scraper
cd go-jira-scraper
go build -o bin/go-jira-scraper ./cmd/go-jira-scraper
```

## Usage

### Setting your JIRA Token

The JIRA API token can be provided in three ways (in priority order):

1. **Command-line flag** (highest priority):
   ```bash
   go-jira-scraper --jira-token "your-token" scrape project --project AAH
   ```

2. **Config file** (see Configuration section below):
   ```yaml
   jira:
     token: "your-token"
   ```

3. **Environment variable** (lowest priority):
   ```bash
   export JIRA_TOKEN="your-token-here"
   ```

**Security Note:** For better security, use environment variables or a secrets manager rather than storing tokens in config files.

### Basic Commands

Scrape a project (full sync):

```bash
go-jira-scraper scrape project --project AAH --full
```

Incremental update:

```bash
go-jira-scraper scrape project --project AAH
```

Adjust batch size if you hit rate limits:

```bash
# Default is 10, which works well with JIRA rate limits
# Larger values (e.g., 100) may cause 429 rate limit errors
go-jira-scraper scrape project --project AAH --batch-size 10
```

Limit the number of issues for testing:

```bash
# Fetch only the first 20 issues (stops search pagination early)
go-jira-scraper scrape project --project AAH --limit 20
```

Scrape a single issue:

```bash
# Fetch a specific issue and its history
go-jira-scraper scrape issue AAH-123

# Force re-fetch even if already cached
go-jira-scraper scrape issue AAH-123 --force
```

List cached issues:

```bash
go-jira-scraper list --project AAH
```

Show cache statistics:

```bash
go-jira-scraper stats
```

## Cache Directory Structure

The tool uses a hierarchical cache structure to support multiple JIRA instances:

```
.data/
└── jira/
    └── issues.redhat.com/
        ├── by_id/
        │   ├── 10001.json
        │   └── 10002.json
        └── by_key/
            ├── AAH-1.json -> ../by_id/10001.json
            └── AAH-2.json -> ../by_id/10002.json
```

This structure allows you to scrape from multiple JIRA instances without conflicts:
- `issues.redhat.com` - Red Hat JIRA
- `jira.atlassian.com` - Atlassian public JIRA
- Any other JIRA instance

**Migration Note**: If you were using an earlier version with the old flat structure (`.data/by_id/` and `.data/by_key/`), the tool will continue to work with that structure if no host is configured. However, new data will be stored in the hierarchical structure.

## Configuration

The tool searches for a config file in the following locations (in order):

1. `./.go-jira-scraper.yaml` (current directory)
2. `~/.go-jira-scraper.yaml` (home directory)
3. `/etc/.go-jira-scraper.yaml` (system-wide)

Or specify a custom location with `--config /path/to/config.yaml`

**Example config file:**

```yaml
jira:
  url: "https://issues.redhat.com"
  token: "${JIRA_TOKEN}"  # Can reference environment variables

cache:
  directory: ".data"

scraper:
  workers: 4
  batch_size: 100

# Optional logging configuration
log_level: "info"      # debug, info, warn, error
log_format: "text"     # text or json
```

**Note:** The config file is optional. All settings can be provided via command-line flags or environment variables.

## Tips & Troubleshooting

### Rate Limits

JIRA APIs have rate limits. If you encounter `429 Rate limit exceeded` errors:

1. **Reduce batch size**: Use `--batch-size 10` (or even `--batch-size 5`)
2. **Use limit flag**: Start with `--limit 20` to test with a small number of issues. This stops the search pagination early, preventing unnecessary API calls.
3. **Wait between runs**: Wait 5-10 minutes before retrying if you hit sustained rate limits
4. **Use incremental mode**: After the initial full sync, use incremental updates (default) which fetch fewer issues

The tool automatically retries on rate limits with exponential backoff (2s, 4s, 8s) and respects `Retry-After` headers.

### Batch Size Notes

- **Default: 10** - Works reliably with JIRA rate limits
- **Higher values (50-100)**: May trigger rate limits depending on your JIRA instance
- **Lower values (5)**: Slower but safest if you have strict rate limits

## Development Status

🚧 Phase 1 (MVP) Complete ✅
- All core features implemented and tested
- Ready for production use with JIRA instances

## License

MIT

