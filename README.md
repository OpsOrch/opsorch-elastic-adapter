# OpsOrch Elasticsearch Log Adapter

Elasticsearch adapter for OpsOrch log queries. This adapter integrates with Elasticsearch to provide normalized log search capabilities.

## Version Compatibility

- **Adapter Version**: 0.1.0
- **Requires OpsOrch Core**: >=0.1.0
- **Elasticsearch Client**: v8.11.1 (go-elasticsearch/v8)
- **Go Version**: 1.22+
- **Supported Elasticsearch**: 7.x, 8.x

## Quick start

1) Configure the adapter with Elasticsearch connection details
2) Use in-process via the registry or as a JSON-RPC plugin
3) Query logs using the normalized `schema.LogQuery` interface

## Configuration

The Elasticsearch log provider expects a config like:

```json
{
  "addresses": ["http://localhost:9200"],
  "username": "elastic",
  "password": "changeme",
  "indexPattern": "logs-*"
}
```

### Configuration Options

- `addresses` ([]string): Elasticsearch cluster URLs (required)
- `username` (string): Username for basic authentication (optional)
- `password` (string): Password for basic authentication (optional)
- `apiKey` (string): API key for authentication (optional, alternative to username/password)
- `cloudID` (string): Elastic Cloud ID (optional, alternative to addresses)
- `indexPattern` (string): Index pattern for log queries (default: "logs-*")

### Authentication Methods

The adapter supports multiple authentication methods:

1. **Basic Authentication**: Provide `username` and `password`
2. **API Key**: Provide `apiKey` in the format `id:api_key`
3. **Elastic Cloud**: Provide `cloudID` along with credentials

## Building

```bash
make test    # run unit tests
make build   # build all packages
make plugin  # builds ./bin/logplugin
make integ   # run integration tests
```

## CI/CD

The project includes GitHub Actions workflows for continuous integration:

- **Test Job**: Runs unit tests on Go 1.21 and 1.22
- **Lint Job**: Checks code formatting with gofmt, runs go vet, and staticcheck

See [.github/workflows/ci.yml](.github/workflows/ci.yml) for the full configuration.

## Usage

### In-Process

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

### As a Plugin

Build the plugin binary:
```bash
make plugin
```

Point OpsOrch Core at the plugin:
```bash
export OPSORCH_LOG_PLUGIN=/path/to/bin/logplugin
```

The plugin communicates via JSON-RPC over stdin/stdout.

## Query Features

The adapter supports:
- **Time range filtering**: Query logs within specific time windows
- **Full-text search**: Search across log messages and indexed fields
- **Structured filters**: Field-level filters with operators (=, !=, contains, regex)
- **Severity filtering**: Filter by log severity levels
- **Scope filtering**: Filter by service, environment, team metadata
- **Result normalization**: Returns standardized `schema.LogEntry` objects

## Production guidance

- Keep adapters stateless; never persist credentials
- Use secure credential management (API keys, environment variables)
- Normalize responses to the current `opsorch-core/schema` types
- Use `metadata` field for Elasticsearch-specific information
- Configure appropriate index patterns for your logging infrastructure
- Use appropriate Elasticsearch query timeouts to prevent long-running queries
- For plugins, build static binaries and checksum them

### Version Management

- Pin the adapter version in your `go.mod` for production deployments
- Test adapter updates in staging before production
- Monitor Elasticsearch client compatibility when upgrading Elasticsearch clusters
- The adapter uses Elasticsearch client v8.11.1, which supports both Elasticsearch 7.x and 8.x
- Verify OpsOrch Core compatibility before upgrading the adapter

## License

See LICENSE file for details.
