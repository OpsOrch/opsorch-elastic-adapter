# OpsOrch Elasticsearch Log Adapter

[![Version](https://img.shields.io/github/v/release/opsorch/opsorch-elastic-adapter)](https://github.com/opsorch/opsorch-elastic-adapter/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/opsorch/opsorch-elastic-adapter)](https://github.com/opsorch/opsorch-elastic-adapter/blob/main/go.mod)
[![License](https://img.shields.io/github/license/opsorch/opsorch-elastic-adapter)](https://github.com/opsorch/opsorch-elastic-adapter/blob/main/LICENSE)
[![CI](https://github.com/opsorch/opsorch-elastic-adapter/workflows/CI/badge.svg)](https://github.com/opsorch/opsorch-elastic-adapter/actions)

This adapter integrates OpsOrch with Elasticsearch, enabling normalized log search capabilities using the Elasticsearch Query DSL.

## Capabilities

This adapter provides one capability:

1. **Log Provider**: Search and retrieve logs from Elasticsearch

## Features

- **Log Search**: Query logs using Elasticsearch Query DSL with full-text search
- **Time Range Filtering**: Query logs within specific time windows
- **Severity Filtering**: Filter by log severity levels (error, warn, info, debug)
- **Structured Filters**: Field-level filters with operators (equality, inequality, contains, regex)
- **Scope Filtering**: Filter by service, environment, team metadata
- **Result Normalization**: Returns standardized OpsOrch LogEntry objects
- **Multiple Authentication Methods**: Supports basic auth, API keys, and Elastic Cloud

### Version Compatibility

- **Adapter Version**: 0.1.0
- **Requires OpsOrch Core**: >=0.1.0
- **Elasticsearch Client**: v8.11.1 (go-elasticsearch/v8)
- **Supported Elasticsearch**: 7.x, 8.x
- **Go Version**: 1.22+

## Configuration

The log adapter requires the following configuration:

| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `addresses` | []string | Yes* | Elasticsearch cluster URLs (e.g., `["http://localhost:9200"]`) | - |
| `username` | string | No | Username for basic authentication | - |
| `password` | string | No | Password for basic authentication | - |
| `apiKey` | string | No | API key for authentication (format: `id:api_key`) | - |
| `cloudID` | string | No | Elastic Cloud ID (alternative to addresses) | - |
| `indexPattern` | string | No | Index pattern for log queries | `logs-*` |

*Either `addresses` or `cloudID` is required

### Authentication Methods

The adapter supports three authentication methods:

#### 1. Basic Authentication

Use username and password:

```json
{
  "addresses": ["http://localhost:9200"],
  "username": "elastic",
  "password": "changeme",
  "indexPattern": "logs-*"
}
```

#### 2. API Key Authentication

Use an Elasticsearch API key:

```json
{
  "addresses": ["http://localhost:9200"],
  "apiKey": "id:api_key",
  "indexPattern": "logs-*"
}
```

To create an API key in Elasticsearch:

1. In Kibana, go to **Stack Management** → **API Keys**
2. Click **Create API key**
3. Give it a name and set appropriate permissions
4. Copy the API key in the format `id:api_key`

#### 3. Elastic Cloud

Use Elastic Cloud ID with credentials:

```json
{
  "cloudID": "deployment-name:base64-encoded-cloud-id",
  "username": "elastic",
  "password": "your-password",
  "indexPattern": "logs-*"
}
```

### Example Configuration

**Self-Hosted Elasticsearch - JSON format:**
```json
{
  "addresses": ["http://localhost:9200"],
  "username": "elastic",
  "password": "changeme",
  "indexPattern": "logs-*"
}
```

**Elastic Cloud - JSON format:**
```json
{
  "cloudID": "deployment:dXMtY2VudHJhbDEuZ2NwLmNsb3VkLmVzLmlvJGFiYzEyMw==",
  "username": "elastic",
  "password": "your-password",
  "indexPattern": "logs-*"
}
```

**Environment variables:**
```bash
export OPSORCH_LOG_PLUGIN=/path/to/bin/logplugin
export OPSORCH_LOG_CONFIG='{"addresses":["http://localhost:9200"],"username":"elastic","password":"changeme","indexPattern":"logs-*"}'
```

## Field Mapping

### Query Mapping

| OpsOrch Field | Elasticsearch Query | Notes |
|---------------|---------------------|-------|
| `Start`, `End` | `range` query on `@timestamp` | Time range filter |
| `expression.search` | `query_string` query | Full-text search across all fields |
| `expression.severityIn` | `terms` query on `severity` field | Matches the `severity` field in each document |
| `expression.filters` | `bool` query with `must`/`must_not` clauses | Field-level filters |
| `scope.service` | `term` query on `service` field | Service filtering |
| `scope.environment` | `term` query on `environment` field | Environment filtering |
| `scope.team` | `term` query on `team` field | Team filtering |

### Response Normalization

| Elasticsearch Field | OpsOrch Field | Transformation | Notes |
|--------------------|---------------|----------------|-------|
| `message` or `_source.message` | `Message` | Direct mapping | Log message text |
| `severity` (fallback to `level`) | `Severity` | Direct mapping | Copies `severity` if present, otherwise `level` |
| `service` | `Service` | Direct mapping | Service name |
| `@timestamp` | `Timestamp` | ISO 8601 timestamp | Log timestamp |
| `_index` | Stored in `Metadata["_index"]` | Direct mapping | Source index |
| `_id` | Stored in `Metadata["_id"]` | Direct mapping | Elasticsearch document ID |
| `_score` | Stored in `Metadata["_score"]` | Direct mapping | Search hit score |
| All other fields | `Fields` | Raw field values | Additional log fields |

### Severity Mapping

The adapter does not remap severity values; they are copied verbatim from the `severity` field (or the `level` field when `severity` is absent). Ensure your indices emit OpsOrch-compatible severity names if normalization is required.

## Usage

### In-Process Mode

Import the adapter for side effects to register it with OpsOrch Core:

```go
import (
    "context"
    "github.com/opsorch/opsorch-core/log"
    "github.com/opsorch/opsorch-core/schema"
    _ "github.com/opsorch/opsorch-elastic-adapter/log" // registers provider
)

constructor, _ := log.LookupProvider("elastic")
provider, _ := constructor(map[string]any{
    "addresses": []any{"http://localhost:9200"},
    "indexPattern": "logs-*",
})

entries, _ := provider.Query(context.Background(), schema.LogQuery{
    Start: time.Now().Add(-1 * time.Hour),
    End:   time.Now(),
    Expression: &schema.LogExpression{
        Search: "error",
        SeverityIn: []string{"error", "critical"},
    },
})
```

Configure via environment variables:

```bash
export OPSORCH_LOG_PROVIDER=elastic
export OPSORCH_LOG_CONFIG='{"addresses":["http://localhost:9200"],"username":"elastic","password":"changeme","indexPattern":"logs-*"}'
```

### Plugin Mode

Build the plugin binary:

```bash
make plugin
```

Configure OpsOrch Core to use the plugin:

```bash
export OPSORCH_LOG_PLUGIN=/path/to/bin/logplugin
export OPSORCH_LOG_CONFIG='{"addresses":["http://localhost:9200"],"username":"elastic","password":"changeme","indexPattern":"logs-*"}'
```

### Docker Deployment

Download pre-built plugin binaries from [GitHub Releases](https://github.com/opsorch/opsorch-elastic-adapter/releases):

```dockerfile
FROM ghcr.io/opsorch/opsorch-core:latest
WORKDIR /opt/opsorch

# Download plugin binary
ADD https://github.com/opsorch/opsorch-elastic-adapter/releases/download/v0.1.0/logplugin-linux-amd64 ./plugins/logplugin
RUN chmod +x ./plugins/logplugin

# Configure plugin
ENV OPSORCH_LOG_PLUGIN=/opt/opsorch/plugins/logplugin
```

## Development

### Prerequisites

- Go 1.22 or later
- Elasticsearch 7.x or 8.x instance
- Access credentials (username/password or API key)

### Building

```bash
# Download dependencies
go mod download

# Run unit tests
make test

# Build all packages
make build

# Build plugin binary
make plugin

# Run integration tests (requires Elasticsearch)
make integ
```

### Testing

**Unit Tests:**
```bash
make test
```

**Integration Tests:**

Integration tests run against a real Elasticsearch instance.

**Prerequisites:**
- Elasticsearch 7.x or 8.x running locally or remotely
- Access credentials
- Test index with log data (or empty index for basic connectivity tests)

**Setup:**
```bash
# Set required environment variables
export ELASTICSEARCH_ADDRESSES="http://localhost:9200"
export ELASTICSEARCH_USERNAME="elastic"
export ELASTICSEARCH_PASSWORD="changeme"
export ELASTICSEARCH_INDEX_PATTERN="logs-*"

# Run integration tests
make integ
```

**What the tests do:**
- Connect to Elasticsearch and verify authentication
- Query logs with various filters (time range, severity, full-text search)
- Test scope filtering (service, environment, team)
- Verify response normalization

**Expected behavior:**
- Tests verify connectivity to Elasticsearch
- Tests may return empty results if no matching logs exist
- Tests verify query structure is correct
- Tests verify response parsing works

**Note:** Integration tests require a running Elasticsearch instance. Use Docker for local testing:

```bash
# Start Elasticsearch with Docker
docker run -d -p 9200:9200 -e "discovery.type=single-node" -e "xpack.security.enabled=false" elasticsearch:8.11.0

# Run tests
export ELASTICSEARCH_ADDRESSES="http://localhost:9200"
make integ

# Clean up
docker stop $(docker ps -q --filter ancestor=elasticsearch:8.11.0)
```

### Project Structure

```
opsorch-elastic-adapter/
├── log/                        # Log provider implementation
│   ├── elastic_provider.go    # Core provider logic
│   └── elastic_provider_test.go
├── cmd/
│   └── logplugin/             # Plugin entrypoint
│       └── main.go
├── integ/                      # Integration tests
│   └── log.go
├── Makefile
└── README.md
```

**Key Components:**

- **log/elastic_provider.go**: Implements log.Provider interface, builds Elasticsearch queries and normalizes responses
- **cmd/logplugin**: JSON-RPC plugin wrapper for log provider
- **integ/log.go**: End-to-end integration tests against live Elasticsearch instance

## CI/CD & Pre-Built Binaries

The repository includes GitHub Actions workflows:

- **CI** (`ci.yml`): Runs tests and linting on every push/PR to main
  - Test Job: Runs unit tests on Go 1.21 and 1.22
  - Lint Job: Checks code formatting with gofmt, runs go vet, and staticcheck
- **Release** (`release.yml`): Manual workflow that:
  - Runs tests and linting
  - Creates version tags (patch/minor/major)
  - Builds multi-arch binaries (linux-amd64, linux-arm64, darwin-amd64, darwin-arm64)
  - Publishes binaries as GitHub release assets

### Downloading Pre-Built Binaries

Pre-built plugin binaries are available from [GitHub Releases](https://github.com/opsorch/opsorch-elastic-adapter/releases).

**Supported platforms:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)

## Plugin RPC Contract

OpsOrch Core communicates with the plugin over stdin/stdout using JSON-RPC.

### Message Format

**Request:**
```json
{
  "method": "log.query",
  "config": { /* decrypted configuration */ },
  "payload": { /* method-specific request body */ }
}
```

**Response:**
```json
{
  "result": { /* method-specific result */ },
  "error": "optional error message"
}
```

### Configuration Injection

The `config` field contains the decrypted configuration map from `OPSORCH_LOG_CONFIG`. The plugin receives this on every request, so it never stores secrets on disk.

### Supported Methods

#### log.query

Search logs with filters.

**Request:**
```json
{
  "method": "log.query",
  "config": {
    "addresses": ["http://localhost:9200"],
    "username": "elastic",
    "password": "changeme",
    "indexPattern": "logs-*"
  },
  "payload": {
    "start": "2024-01-01T00:00:00Z",
    "end": "2024-01-01T01:00:00Z",
    "expression": {
      "search": "error",
      "severityIn": ["error", "critical"]
    },
    "scope": {
      "service": "api",
      "environment": "prod"
    },
    "limit": 100
  }
}
```

**Response:**
```json
{
  "result": [
    {
      "message": "Database connection timeout",
      "severity": "error",
      "service": "api",
      "timestamp": "2024-01-01T00:15:30Z",
      "labels": ["error", "database"],
      "fields": {
        "error.type": "timeout",
        "duration_ms": 5000
      },
      "metadata": {
        "log_id": "abc123",
        "host": "api-server-01",
        "index": "logs-2024.01.01"
      }
    }
  ]
}
```

## Production Guidance

### Index Patterns

- Configure appropriate index patterns for your logging infrastructure
- Use date-based indices for better performance: `logs-2024.01.*`
- Consider using data streams for automatic index lifecycle management

### Query Performance

- Use appropriate time ranges to limit query scope
- Add field-level filters to reduce result sets
- Configure Elasticsearch query timeouts to prevent long-running queries
- Monitor query performance and adjust index settings as needed

### Security

1. **Never log credentials**: Avoid logging the config, username, password, or API key
2. **Use API keys**: Prefer API keys over username/password for better security
3. **Rotate credentials regularly**: Follow your organization's security policy
4. **Use TLS**: Always use HTTPS for production Elasticsearch clusters
5. **Restrict permissions**: Grant only necessary index read permissions to the API key/user
6. **Network security**: Ensure Elasticsearch is not publicly accessible

### Version Management

- Pin the adapter version in your `go.mod` for production deployments
- Test adapter updates in staging before production
- Monitor Elasticsearch client compatibility when upgrading Elasticsearch clusters
- The adapter uses Elasticsearch client v8.11.1, which supports both Elasticsearch 7.x and 8.x
- Verify OpsOrch Core compatibility before upgrading the adapter

## License

Apache 2.0

See LICENSE file for details.
