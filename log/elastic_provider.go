package log

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	corelog "github.com/opsorch/opsorch-core/log"
	"github.com/opsorch/opsorch-core/schema"
)

// ProviderName is the registry key for the Elasticsearch adapter.
const ProviderName = "elastic"

// AdapterVersion and RequiresCore express compatibility.
const (
	AdapterVersion = "0.1.0"
	RequiresCore   = ">=0.1.0"
)

// Config captures decrypted configuration from OpsOrch Core.
type Config struct {
	Addresses    []string
	Username     string
	Password     string
	APIKey       string
	CloudID      string
	IndexPattern string
}

// ElasticProvider implements the log.Provider interface for Elasticsearch.
type ElasticProvider struct {
	cfg    Config
	client *elasticsearch.Client
}

// New constructs the provider from decrypted config.
func New(cfg map[string]any) (corelog.Provider, error) {
	parsed := parseConfig(cfg)

	// Validate configuration
	if len(parsed.Addresses) == 0 && parsed.CloudID == "" {
		return nil, errors.New("either 'addresses' or 'cloudID' must be provided")
	}

	// Build Elasticsearch client configuration
	esCfg := elasticsearch.Config{}

	if parsed.CloudID != "" {
		esCfg.CloudID = parsed.CloudID
	} else {
		esCfg.Addresses = parsed.Addresses
	}

	// Configure authentication
	if parsed.APIKey != "" {
		esCfg.APIKey = parsed.APIKey
	} else if parsed.Username != "" || parsed.Password != "" {
		esCfg.Username = parsed.Username
		esCfg.Password = parsed.Password
	}

	// Create Elasticsearch client
	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	// Test connection with a ping
	_, err = client.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Elasticsearch: %w", err)
	}

	return &ElasticProvider{
		cfg:    parsed,
		client: client,
	}, nil
}

func init() {
	_ = corelog.RegisterProvider(ProviderName, New)
}

// Query executes a log query against Elasticsearch and returns normalized log entries.
func (p *ElasticProvider) Query(ctx context.Context, query schema.LogQuery) ([]schema.LogEntry, error) {
	// Build Elasticsearch query DSL
	esQuery := p.buildQuery(query)

	// Marshal to JSON
	queryBody, err := json.Marshal(esQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	// Execute search
	res, err := p.client.Search(
		p.client.Search.WithContext(ctx),
		p.client.Search.WithIndex(p.cfg.IndexPattern),
		p.client.Search.WithBody(strings.NewReader(string(queryBody))),
		p.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch query failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch returned error: %s", res.String())
	}

	// Parse response
	var result esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Normalize to schema.LogEntry
	entries := make([]schema.LogEntry, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		entry := p.normalizeHit(hit)
		entries = append(entries, entry)
	}

	return entries, nil
}

// buildQuery constructs an Elasticsearch query DSL from LogQuery.
func (p *ElasticProvider) buildQuery(query schema.LogQuery) map[string]any {
	mustClauses := []map[string]any{}

	// Time range filter
	if !query.Start.IsZero() || !query.End.IsZero() {
		rangeClause := map[string]any{
			"range": map[string]any{
				"@timestamp": map[string]any{},
			},
		}
		if !query.Start.IsZero() {
			rangeClause["range"].(map[string]any)["@timestamp"].(map[string]any)["gte"] = query.Start.Format(time.RFC3339)
		}
		if !query.End.IsZero() {
			rangeClause["range"].(map[string]any)["@timestamp"].(map[string]any)["lte"] = query.End.Format(time.RFC3339)
		}
		mustClauses = append(mustClauses, rangeClause)
	}

	// Expression filters
	if query.Expression != nil {
		// Full-text search
		if query.Expression.Search != "" {
			mustClauses = append(mustClauses, map[string]any{
				"query_string": map[string]any{
					"query": query.Expression.Search,
				},
			})
		}

		// Severity filter
		if len(query.Expression.SeverityIn) > 0 {
			mustClauses = append(mustClauses, map[string]any{
				"terms": map[string]any{
					"severity": query.Expression.SeverityIn,
				},
			})
		}

		// Structured filters
		for _, filter := range query.Expression.Filters {
			clause := p.buildFilterClause(filter)
			if clause != nil {
				mustClauses = append(mustClauses, clause)
			}
		}
	}

	// Scope filters
	if query.Scope.Service != "" {
		mustClauses = append(mustClauses, map[string]any{
			"term": map[string]any{
				"service": query.Scope.Service,
			},
		})
	}
	if query.Scope.Environment != "" {
		mustClauses = append(mustClauses, map[string]any{
			"term": map[string]any{
				"environment": query.Scope.Environment,
			},
		})
	}
	if query.Scope.Team != "" {
		mustClauses = append(mustClauses, map[string]any{
			"term": map[string]any{
				"team": query.Scope.Team,
			},
		})
	}

	// Metadata filters
	for key, value := range query.Metadata {
		mustClauses = append(mustClauses, map[string]any{
			"term": map[string]any{
				key: value,
			},
		})
	}

	// Build final query
	esQuery := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": mustClauses,
			},
		},
		"sort": []map[string]any{
			{"@timestamp": map[string]any{"order": "desc"}},
		},
	}

	// Apply limit
	if query.Limit > 0 {
		esQuery["size"] = query.Limit
	} else {
		esQuery["size"] = 1000 // Default limit
	}

	return esQuery
}

// buildFilterClause converts a LogFilter to an Elasticsearch clause.
func (p *ElasticProvider) buildFilterClause(filter schema.LogFilter) map[string]any {
	switch filter.Operator {
	case "=":
		return map[string]any{
			"term": map[string]any{
				filter.Field: filter.Value,
			},
		}
	case "!=":
		return map[string]any{
			"bool": map[string]any{
				"must_not": map[string]any{
					"term": map[string]any{
						filter.Field: filter.Value,
					},
				},
			},
		}
	case "contains":
		return map[string]any{
			"wildcard": map[string]any{
				filter.Field: map[string]any{
					"value": "*" + filter.Value + "*",
				},
			},
		}
	case "regex":
		return map[string]any{
			"regexp": map[string]any{
				filter.Field: map[string]any{
					"value": filter.Value,
				},
			},
		}
	default:
		return nil
	}
}

// normalizeHit converts an Elasticsearch hit to a schema.LogEntry.
func (p *ElasticProvider) normalizeHit(hit esHit) schema.LogEntry {
	source := hit.Source

	entry := schema.LogEntry{
		Metadata: map[string]any{
			"_index": hit.Index,
			"_id":    hit.ID,
			"_score": hit.Score,
		},
	}

	// Extract timestamp
	if ts, ok := source["@timestamp"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			entry.Timestamp = parsed
		}
	}

	// Extract message
	if msg, ok := source["message"].(string); ok {
		entry.Message = msg
	}

	// Extract severity
	if sev, ok := source["severity"].(string); ok {
		entry.Severity = sev
	} else if level, ok := source["level"].(string); ok {
		entry.Severity = level
	}

	// Extract service
	if svc, ok := source["service"].(string); ok {
		entry.Service = svc
	}

	// Extract labels (string-valued fields)
	entry.Labels = make(map[string]string)
	for key, value := range source {
		if key == "@timestamp" || key == "message" || key == "severity" || key == "level" || key == "service" {
			continue
		}
		if strVal, ok := value.(string); ok {
			entry.Labels[key] = strVal
		}
	}

	// Extract fields (all structured data)
	entry.Fields = make(map[string]any)
	for key, value := range source {
		if key == "@timestamp" || key == "message" || key == "severity" || key == "level" || key == "service" {
			continue
		}
		entry.Fields[key] = value
	}

	return entry
}

// parseConfig extracts and validates configuration.
func parseConfig(cfg map[string]any) Config {
	out := Config{
		IndexPattern: "logs-*", // Default index pattern
	}

	// Parse addresses
	if addrs, ok := cfg["addresses"].([]any); ok {
		for _, addr := range addrs {
			if strAddr, ok := addr.(string); ok {
				out.Addresses = append(out.Addresses, strAddr)
			}
		}
	}

	// Parse string fields
	if v, ok := cfg["username"].(string); ok {
		out.Username = v
	}
	if v, ok := cfg["password"].(string); ok {
		out.Password = v
	}
	if v, ok := cfg["apiKey"].(string); ok {
		out.APIKey = v
	}
	if v, ok := cfg["cloudID"].(string); ok {
		out.CloudID = v
	}
	if v, ok := cfg["indexPattern"].(string); ok && v != "" {
		out.IndexPattern = v
	}

	return out
}

// Elasticsearch response types
type esSearchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []esHit `json:"hits"`
	} `json:"hits"`
}

type esHit struct {
	Index  string                 `json:"_index"`
	ID     string                 `json:"_id"`
	Score  float64                `json:"_score"`
	Source map[string]interface{} `json:"_source"`
}
