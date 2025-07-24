package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEncodeOutputWithJQ(t *testing.T) {
	tests := []struct {
		name        string
		format      OutputFormat
		data        interface{}
		jqQuery     string
		wantContain string
		wantEmpty   bool
		wantErr     bool
	}{
		{
			name:   "no jq query - YAML output",
			format: FormatYAML,
			data: map[string]interface{}{
				"name": "test",
				"value": 42,
			},
			jqQuery:     "",
			wantContain: "name: test",
		},
		{
			name:   "no jq query - JSON output",
			format: FormatJSON,
			data: map[string]interface{}{
				"name": "test",
				"value": 42,
			},
			jqQuery:     "",
			wantContain: `"name": "test"`,
		},
		{
			name:   "extract single field",
			format: FormatYAML,
			data: map[string]interface{}{
				"name": "test",
				"value": 42,
			},
			jqQuery:     ".name",
			wantContain: "test",
		},
		{
			name:   "filter array",
			format: FormatJSON,
			data: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"id": 1, "active": true},
					map[string]interface{}{"id": 2, "active": false},
					map[string]interface{}{"id": 3, "active": true},
				},
			},
			jqQuery:     ".items[] | select(.active) | .id",
			wantContain: "1",
		},
		{
			name:   "construct object",
			format: FormatYAML,
			data: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "Alice",
					"age":  30,
					"email": "alice@example.com",
				},
			},
			jqQuery:     ".user | {name, age}",
			wantContain: "name: Alice",
		},
		{
			name:   "invalid jq query",
			format: FormatYAML,
			data: map[string]interface{}{
				"test": "data",
			},
			jqQuery: ".invalid syntax[",
			wantErr: true,
		},
		{
			name:   "empty result",
			format: FormatJSON,
			data: map[string]interface{}{
				"items": []interface{}{},
			},
			jqQuery:   ".items[]",
			wantEmpty: true,
		},
		{
			name:   "multiple results as YAML",
			format: FormatYAML,
			data: map[string]interface{}{
				"numbers": []interface{}{1, 2, 3},
			},
			jqQuery:     ".numbers[]",
			wantContain: "1\n---\n2\n---\n3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := EncodeOutputWithJQ(&buf, tt.format, tt.data, tt.jqQuery)

			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeOutputWithJQ() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Skip output check if error was expected
			}

			output := buf.String()
			
			// Check if we expect empty output
			if tt.wantEmpty {
				if output != "" {
					t.Errorf("EncodeOutputWithJQ() output = %v, want empty", output)
				}
				return
			}
			
			// Check for expected content
			if tt.wantContain != "" && !strings.Contains(output, tt.wantContain) {
				t.Errorf("EncodeOutputWithJQ() output = %v, want to contain %v", output, tt.wantContain)
			}
		})
	}
}

func TestEncodeOutputWithJQEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		jqQuery string
		wantErr bool
	}{
		{
			name:    "complex nested query",
			data: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{
						"name": "Alice",
						"posts": []interface{}{
							map[string]interface{}{"id": 1, "title": "First"},
							map[string]interface{}{"id": 2, "title": "Second"},
						},
					},
				},
			},
			jqQuery: ".users[].posts[] | select(.id == 1) | .title",
			wantErr: false,
		},
		{
			name:    "error in jq expression",
			data:    map[string]interface{}{"test": "value"},
			jqQuery: ".test | tonumber",
			wantErr: true, // "value" cannot be converted to number
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := EncodeOutputWithJQ(&buf, FormatJSON, tt.data, tt.jqQuery)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeOutputWithJQ() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncodeOutputWithJQTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}
	
	// This test verifies that long-running jq queries are terminated by the timeout
	// We create a large dataset and use a complex query that would take too long
	
	// Create a large dataset with nested structures
	largeData := make([]interface{}, 10000)
	for i := range largeData {
		largeData[i] = map[string]interface{}{
			"id": i,
			"nested": map[string]interface{}{
				"value": i * 2,
				"deep": map[string]interface{}{
					"data": i * 3,
				},
			},
		}
	}
	
	// Use a pathologically slow jq query - cartesian product that would take forever
	slowQuery := `. as $all | .[] | . as $item | $all[] | select(.id > $item.id) | {a: $item.id, b: .id}`
	
	var buf bytes.Buffer
	err := EncodeOutputWithJQ(&buf, FormatJSON, largeData, slowQuery)
	
	// We expect this to timeout (context deadline exceeded)
	if err == nil {
		t.Error("Expected timeout error, but got none")
	}
	
	// Check for context deadline exceeded error
	// Note: Ideally we would only check errors.Is(err, context.DeadlineExceeded),
	// but go-jq-yamlformat currently returns "execution timeout after Xs" without
	// properly wrapping the context.DeadlineExceeded error. This is a known issue
	// that should be fixed in the library.
	if !errors.Is(err, context.DeadlineExceeded) {
		// Fallback check for the specific error message from go-jq-yamlformat
		if !strings.Contains(err.Error(), "execution timeout after") {
			t.Errorf("Expected context.DeadlineExceeded or 'execution timeout after', got: %v", err)
		}
	}
}