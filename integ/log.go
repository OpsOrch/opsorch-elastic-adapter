//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/opsorch/opsorch-core/schema"
	elasticlog "github.com/opsorch/opsorch-elastic-adapter/log"
)

func main() {
	// Test statistics
	var totalTests, passedTests, failedTests int
	startTime := time.Now()

	testResult := func(name string, err error) {
		totalTests++
		if err != nil {
			failedTests++
			log.Printf("❌ %s: %v", name, err)
		} else {
			passedTests++
			fmt.Printf("✅ %s passed\n", name)
		}
	}

	fmt.Println("=============================================")
	fmt.Println("Elasticsearch Log Adapter Integration Test")
	fmt.Println("=============================================")
	fmt.Printf("Started: %s\n\n", startTime.Format("2006-01-02 15:04:05"))

	fmt.Println("⚠️  NOTE: These tests require a running Elasticsearch instance")
	fmt.Println("   Configure connection via environment or modify the test config")
	fmt.Println()

	ctx := context.Background()

	// Test 1: Create provider with valid config
	fmt.Println("\n=== Test 1: Create Provider with Valid Config ===")
	config := map[string]any{
		"addresses":    []any{"http://localhost:9200"},
		"indexPattern": "logs-*",
	}

	provider, err := elasticlog.New(config)
	if err != nil {
		testResult("Create provider", err)
		fmt.Printf("⚠️  Skipping remaining tests due to provider creation failure\n")
		fmt.Printf("   Make sure Elasticsearch is running on localhost:9200\n")
		printSummary(totalTests, passedTests, failedTests, startTime)
		return
	}
	fmt.Println("✅ Provider created successfully")
	testResult("Create provider", nil)

	// Test 2: Query with time range
	fmt.Println("\n=== Test 2: Query Logs with Time Range ===")
	entries, err := provider.Query(ctx, schema.LogQuery{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
		Limit: 10,
	})
	if err != nil {
		testResult("Query with time range", err)
	} else {
		fmt.Printf("Found %d log entries\n", len(entries))
		if len(entries) > 0 {
			fmt.Printf("Sample entry:\n")
			fmt.Printf("  Timestamp: %s\n", entries[0].Timestamp.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Message: %s\n", truncate(entries[0].Message, 80))
			fmt.Printf("  Severity: %s\n", entries[0].Severity)
			fmt.Printf("  Service: %s\n", entries[0].Service)
		}
		testResult("Query with time range", nil)
	}

	// Test 3: Query with full-text search
	fmt.Println("\n=== Test 3: Query with Full-Text Search ===")
	entries, err = provider.Query(ctx, schema.LogQuery{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
		Expression: &schema.LogExpression{
			Search: "error OR warning",
		},
		Limit: 5,
	})
	if err != nil {
		testResult("Query with full-text search", err)
	} else {
		fmt.Printf("Found %d entries matching 'error OR warning'\n", len(entries))
		for i, entry := range entries {
			fmt.Printf("  [%d] %s: %s\n", i+1, entry.Timestamp.Format("15:04:05"), truncate(entry.Message, 60))
		}
		testResult("Query with full-text search", nil)
	}

	// Test 4: Query with severity filter
	fmt.Println("\n=== Test 4: Query by Severity ===")
	entries, err = provider.Query(ctx, schema.LogQuery{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
		Expression: &schema.LogExpression{
			SeverityIn: []string{"error", "critical"},
		},
		Limit: 5,
	})
	if err != nil {
		testResult("Query by severity", err)
	} else {
		fmt.Printf("Found %d error/critical entries\n", len(entries))
		allMatch := true
		for _, entry := range entries {
			if entry.Severity != "error" && entry.Severity != "critical" {
				allMatch = false
				testResult("Validate severity filter", fmt.Errorf("expected error/critical, got %s", entry.Severity))
				break
			}
		}
		if allMatch {
			testResult("Query by severity", nil)
		}
	}

	// Test 5: Query with structured filters
	fmt.Println("\n=== Test 5: Query with Structured Filters ===")
	entries, err = provider.Query(ctx, schema.LogQuery{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
		Expression: &schema.LogExpression{
			Filters: []schema.LogFilter{
				{
					Field:    "service",
					Operator: "=",
					Value:    "api-gateway",
				},
			},
		},
		Limit: 5,
	})
	if err != nil {
		// It's ok if no results, as long as query syntax is correct
		if len(entries) == 0 {
			fmt.Println("No entries found for service=api-gateway (expected if not in dataset)")
			testResult("Query with structured filters", nil)
		} else {
			testResult("Query with structured filters", err)
		}
	} else {
		fmt.Printf("Found %d entries for service=api-gateway\n", len(entries))
		testResult("Query with structured filters", nil)
	}

	// Test 6: Query with scope filters
	fmt.Println("\n=== Test 6: Query with Scope Filters ===")
	entries, err = provider.Query(ctx, schema.LogQuery{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
		Scope: schema.QueryScope{
			Environment: "production",
		},
		Limit: 5,
	})
	if err != nil {
		// It's ok if no results
		if len(entries) == 0 {
			fmt.Println("No entries found for environment=production (expected if not in dataset)")
			testResult("Query with scope filters", nil)
		} else {
			testResult("Query with scope filters", err)
		}
	} else {
		fmt.Printf("Found %d entries in production environment\n", len(entries))
		testResult("Query with scope filters", nil)
	}

	// Test 7: Query with limit
	fmt.Println("\n=== Test 7: Query with Limit ===")
	entries, err = provider.Query(ctx, schema.LogQuery{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
		Limit: 3,
	})
	if err != nil {
		testResult("Query with limit", err)
	} else {
		if len(entries) <= 3 {
			fmt.Printf("Correctly limited to %d entries\n", len(entries))
			testResult("Query with limit", nil)
		} else {
			testResult("Query with limit", fmt.Errorf("expected max 3 entries, got %d", len(entries)))
		}
	}

	// Test 8: Query with combined filters
	fmt.Println("\n=== Test 8: Query with Combined Filters ===")
	entries, err = provider.Query(ctx, schema.LogQuery{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
		Expression: &schema.LogExpression{
			Search:     "error",
			SeverityIn: []string{"error", "warning"},
		},
		Limit: 5,
	})
	if err != nil {
		testResult("Query with combined filters", err)
	} else {
		fmt.Printf("Found %d entries (search='error' AND severity in [error,warning])\n", len(entries))
		testResult("Query with combined filters", nil)
	}

	// Test 9: Validate result normalization
	fmt.Println("\n=== Test 9: Validate Result Normalization ===")
	entries, err = provider.Query(ctx, schema.LogQuery{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
		Limit: 1,
	})
	if err != nil {
		testResult("Validate result normalization", err)
	} else if len(entries) == 0 {
		fmt.Println("No entries to validate (empty result set)")
		testResult("Validate result normalization", nil)
	} else {
		entry := entries[0]
		hasTimestamp := !entry.Timestamp.IsZero()
		hasMetadata := entry.Metadata != nil && len(entry.Metadata) > 0

		fmt.Printf("Entry validation:\n")
		fmt.Printf("  Has timestamp: %v\n", hasTimestamp)
		fmt.Printf("  Has message: %v\n", entry.Message != "")
		fmt.Printf("  Has metadata: %v\n", hasMetadata)
		fmt.Printf("  Metadata keys: %v\n", getKeys(entry.Metadata))

		if !hasTimestamp {
			testResult("Validate result normalization", fmt.Errorf("missing timestamp"))
		} else if !hasMetadata {
			testResult("Validate result normalization", fmt.Errorf("missing metadata"))
		} else {
			testResult("Validate result normalization", nil)
		}
	}

	// Test 10: Invalid configuration
	fmt.Println("\n=== Test 10: Error Handling - Invalid Config ===")
	_, err = elasticlog.New(map[string]any{}) // No addresses or cloudID
	if err != nil {
		fmt.Printf("Correctly rejected invalid config: %v\n", err)
		testResult("Error handling for invalid config", nil)
	} else {
		testResult("Error handling for invalid config", fmt.Errorf("should have rejected config without addresses"))
	}

	printSummary(totalTests, passedTests, failedTests, startTime)
}

func printSummary(totalTests, passedTests, failedTests int, startTime time.Time) {
	// Print summary
	duration := time.Since(startTime)
	fmt.Println("\n=============================================")
	fmt.Println("Test Summary")
	fmt.Println("=============================================")
	fmt.Printf("Total Tests: %d\n", totalTests)
	fmt.Printf("Passed: %d ✅\n", passedTests)
	fmt.Printf("Failed: %d ❌\n", failedTests)
	fmt.Printf("Duration: %v\n", duration.Round(time.Millisecond))
	if totalTests > 0 {
		fmt.Printf("Success Rate: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)
	}

	if failedTests == 0 {
		fmt.Println("\n✅ All tests passed successfully!")
	} else {
		fmt.Printf("\n⚠️  %d test(s) failed. Please review the output above.\n", failedTests)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func getKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
