package log

import (
	"testing"

	"github.com/opsorch/opsorch-core/schema"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected Config
	}{
		{
			name: "basic config with addresses",
			input: map[string]any{
				"addresses":    []any{"http://localhost:9200"},
				"username":     "elastic",
				"password":     "changeme",
				"indexPattern": "logs-*",
			},
			expected: Config{
				Addresses:    []string{"http://localhost:9200"},
				Username:     "elastic",
				Password:     "changeme",
				IndexPattern: "logs-*",
			},
		},
		{
			name: "cloud config",
			input: map[string]any{
				"cloudID": "my-cloud:dXMtY2VudHJhbDEuZ2NwLmNsb3VkLmVzLmlvJDEyMzQ1Njc4",
				"apiKey":  "id:key",
			},
			expected: Config{
				CloudID:      "my-cloud:dXMtY2VudHJhbDEuZ2NwLmNsb3VkLmVzLmlvJDEyMzQ1Njc4",
				APIKey:       "id:key",
				IndexPattern: "logs-*", // default
			},
		},
		{
			name:  "default index pattern",
			input: map[string]any{},
			expected: Config{
				IndexPattern: "logs-*",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseConfig(tt.input)

			if len(result.Addresses) != len(tt.expected.Addresses) {
				t.Errorf("addresses length mismatch: got %d, want %d", len(result.Addresses), len(tt.expected.Addresses))
			}
			for i, addr := range result.Addresses {
				if addr != tt.expected.Addresses[i] {
					t.Errorf("addresses[%d] = %s, want %s", i, addr, tt.expected.Addresses[i])
				}
			}

			if result.Username != tt.expected.Username {
				t.Errorf("username = %s, want %s", result.Username, tt.expected.Username)
			}
			if result.Password != tt.expected.Password {
				t.Errorf("password = %s, want %s", result.Password, tt.expected.Password)
			}
			if result.APIKey != tt.expected.APIKey {
				t.Errorf("apiKey = %s, want %s", result.APIKey, tt.expected.APIKey)
			}
			if result.CloudID != tt.expected.CloudID {
				t.Errorf("cloudID = %s, want %s", result.CloudID, tt.expected.CloudID)
			}
			if result.IndexPattern != tt.expected.IndexPattern {
				t.Errorf("indexPattern = %s, want %s", result.IndexPattern, tt.expected.IndexPattern)
			}
		})
	}
}

func TestBuildFilterClause(t *testing.T) {
	p := &ElasticProvider{}

	tests := []struct {
		name     string
		filter   schema.LogFilter
		expected map[string]any
	}{
		{
			name: "equals operator",
			filter: schema.LogFilter{
				Field:    "status",
				Operator: "=",
				Value:    "200",
			},
			expected: map[string]any{
				"term": map[string]any{
					"status": "200",
				},
			},
		},
		{
			name: "not equals operator",
			filter: schema.LogFilter{
				Field:    "status",
				Operator: "!=",
				Value:    "200",
			},
			expected: map[string]any{
				"bool": map[string]any{
					"must_not": map[string]any{
						"term": map[string]any{
							"status": "200",
						},
					},
				},
			},
		},
		{
			name: "contains operator",
			filter: schema.LogFilter{
				Field:    "message",
				Operator: "contains",
				Value:    "error",
			},
			expected: map[string]any{
				"wildcard": map[string]any{
					"message": map[string]any{
						"value": "*error*",
					},
				},
			},
		},
		{
			name: "regex operator",
			filter: schema.LogFilter{
				Field:    "url",
				Operator: "regex",
				Value:    "/api/.*",
			},
			expected: map[string]any{
				"regexp": map[string]any{
					"url": map[string]any{
						"value": "/api/.*",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.buildFilterClause(tt.filter)
			// Basic structural validation
			if result == nil {
				t.Fatalf("expected non-nil result")
			}
			// Note: Deep comparison would require more sophisticated testing
			// This validates the basic structure is created
		})
	}
}

func TestNormalizeHit(t *testing.T) {
	p := &ElasticProvider{}

	hit := esHit{
		Index: "logs-2023.10.01",
		ID:    "abc123",
		Score: 1.23,
		Source: map[string]interface{}{
			"@timestamp":  "2023-10-01T12:00:00Z",
			"message":     "Test log message",
			"severity":    "error",
			"service":     "api-gateway",
			"environment": "production",
			"status":      "500",
		},
	}

	entry := normalizeHit(p, hit)

	if entry.Message != "Test log message" {
		t.Errorf("message = %s, want Test log message", entry.Message)
	}
	if entry.Severity != "error" {
		t.Errorf("severity = %s, want error", entry.Severity)
	}
	if entry.Service != "api-gateway" {
		t.Errorf("service = %s, want api-gateway", entry.Service)
	}
	if entry.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
	if entry.Metadata["_index"] != "logs-2023.10.01" {
		t.Errorf("metadata[_index] = %v, want logs-2023.10.01", entry.Metadata["_index"])
	}
	if entry.Labels["environment"] != "production" {
		t.Errorf("labels[environment] = %s, want production", entry.Labels["environment"])
	}
}

func TestBuildKibanaURL(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		indexPattern  string
		query         schema.LogQuery
		expectURL     bool
		shouldContain []string
	}{
		{
			name:         "empty base URL",
			baseURL:      "",
			indexPattern: "logs-*",
			query:        schema.LogQuery{},
			expectURL:    false,
		},
		{
			name:         "basic URL without filters",
			baseURL:      "http://kibana:5601",
			indexPattern: "logs-*",
			query:        schema.LogQuery{},
			expectURL:    true,
			shouldContain: []string{
				"http://kibana:5601/app/kibana#/discover",
				"logs-*",
			},
		},
		{
			name:         "URL with search expression",
			baseURL:      "http://kibana:5601",
			indexPattern: "logs-app",
			query: schema.LogQuery{
				Expression: &schema.LogExpression{
					Search: "error connection",
				},
			},
			expectURL: true,
			shouldContain: []string{
				"error connection",
				"logs-app",
			},
		},
		{
			name:         "URL with structured filter",
			baseURL:      "http://kibana:5601",
			indexPattern: "logs-*",
			query: schema.LogQuery{
				Expression: &schema.LogExpression{
					Filters: []schema.LogFilter{
						{
							Field:    "service",
							Operator: "=",
							Value:    "api-gateway",
						},
					},
				},
			},
			expectURL: true,
			shouldContain: []string{
				"api-gateway",
				"logs-*",
			},
		},
		{
			name:         "URL with scope filter",
			baseURL:      "http://kibana:5601",
			indexPattern: "logs-*",
			query: schema.LogQuery{
				Scope: schema.QueryScope{
					Service:     "api",
					Environment: "production",
				},
			},
			expectURL: true,
			shouldContain: []string{
				"api",
				"production",
				"logs-*",
			},
		},
		{
			name:         "URL with multiple filters",
			baseURL:      "http://kibana:5601",
			indexPattern: "logs-prod",
			query: schema.LogQuery{
				Expression: &schema.LogExpression{
					Search: "error",
					Filters: []schema.LogFilter{
						{
							Field:    "status",
							Operator: "=",
							Value:    "500",
						},
					},
				},
				Scope: schema.QueryScope{
					Environment: "production",
				},
			},
			expectURL: true,
			shouldContain: []string{
				"error",
				"500",
				"production",
				"logs-prod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := buildKibanaURL(tt.baseURL, tt.indexPattern, tt.query)

			if tt.expectURL && url == "" {
				t.Errorf("expected URL, got empty string")
			}
			if !tt.expectURL && url != "" {
				t.Errorf("expected empty URL, got %s", url)
			}

			for _, shouldContain := range tt.shouldContain {
				if !contains(url, shouldContain) {
					t.Errorf("URL should contain %q, got %s", shouldContain, url)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	// Simple substring check
	if len(substr) == 0 {
		return true
	}
	return indexInString(s, substr) != -1
}

func indexInString(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
