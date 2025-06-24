package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	yamlformat "github.com/apstndb/go-yamlformat"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
)

// OutputFormat represents the output format for structured data
type OutputFormat string

const (
	FormatYAML     OutputFormat = "yaml"
	FormatJSON     OutputFormat = "json"
	FormatMarkdown OutputFormat = "markdown"
)

// ResolveFormat resolves the output format from command flags
// Handles mutually exclusive --format, --json, --yaml flags (enforced by cobra), defaults to YAML
func ResolveFormat(cmd *cobra.Command) OutputFormat {
	// Check aliases first (these take precedence since they're more specific)
	if jsonFlag, _ := cmd.Flags().GetBool("json"); jsonFlag {
		return FormatJSON
	}
	if yamlFlag, _ := cmd.Flags().GetBool("yaml"); yamlFlag {
		return FormatYAML
	}
	
	// Check main format flag
	formatStr, _ := cmd.Flags().GetString("format")
	format := OutputFormat(strings.ToLower(formatStr))
	switch format {
	case FormatJSON, FormatYAML, FormatMarkdown:
		return format
	default:
		return FormatYAML // Default
	}
}

// EncodeOutput encodes data to stdout using the given format
func EncodeOutput(w io.Writer, format OutputFormat, data interface{}) error {
	// Get jq query from command if available
	cmd := currentCommand
	if cmd != nil {
		jqQuery, _ := cmd.Root().Flags().GetString("jq")
		if jqQuery != "" {
			return EncodeOutputWithJQ(w, format, data, jqQuery)
		}
	}

	switch format {
	case FormatJSON:
		encoder := yamlformat.NewJSONEncoder(w)
		return encoder.Encode(data)
	default: // YAML and others
		encoder := yamlformat.NewEncoder(w)
		return encoder.Encode(data)
	}
}

// EncodeOutputWithCmd encodes data with optional jq query from command
func EncodeOutputWithCmd(cmd *cobra.Command, data interface{}) error {
	// Save current command for EncodeOutput to use
	oldCmd := currentCommand
	currentCommand = cmd
	defer func() { currentCommand = oldCmd }()

	format := ResolveFormat(cmd)
	return EncodeOutput(os.Stdout, format, data)
}

// currentCommand is used to pass command context to EncodeOutput
var currentCommand *cobra.Command

// EncodeOutputWithJQ encodes data with optional jq query filtering
func EncodeOutputWithJQ(w io.Writer, format OutputFormat, data interface{}, jqQuery string) error {
	// If no jq query provided, encode normally
	if jqQuery == "" {
		// Temporarily clear currentCommand to avoid recursion
		oldCmd := currentCommand
		currentCommand = nil
		defer func() { currentCommand = oldCmd }()
		return EncodeOutput(w, format, data)
	}

	// Parse the jq query
	query, err := gojq.Parse(jqQuery)
	if err != nil {
		return fmt.Errorf("invalid jq query: %w", err)
	}

	// Convert data to generic interface for gojq
	// This ensures gojq can process the data properly
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	
	var genericData interface{}
	if err := json.Unmarshal(jsonBytes, &genericData); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	// Compile the query for better performance
	code, err := gojq.Compile(query)
	if err != nil {
		return fmt.Errorf("failed to compile jq query: %w", err)
	}

	// Create context with timeout for query execution
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run the query
	iter := code.RunWithContext(ctx, genericData)

	// Collect all results first
	var results []interface{}
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if err == context.DeadlineExceeded {
				return fmt.Errorf("jq query timed out after 30 seconds")
			}
			return fmt.Errorf("jq query error: %w", err)
		}
		results = append(results, v)
	}

	// Determine what to encode
	var output interface{}
	switch len(results) {
	case 0:
		output = nil
	case 1:
		output = results[0]
	default:
		output = results
	}

	// Encode the output
	switch format {
	case FormatJSON:
		encoder := yamlformat.NewJSONEncoder(w)
		return encoder.Encode(output)
	default:
		encoder := yamlformat.NewEncoder(w)
		return encoder.Encode(output)
	}
}

// Unmarshal unmarshals data using yamlformat
func Unmarshal(data []byte, v interface{}) error {
	return yamlformat.Unmarshal(data, v)
}

// Marshal marshals data for the JSON format (used in github.go)
func (f OutputFormat) Marshal(data interface{}) ([]byte, error) {
	if f == FormatJSON {
		return yamlformat.MarshalJSON(data)
	}
	return yamlformat.Marshal(data)
}