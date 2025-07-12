package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	jqyaml "github.com/apstndb/go-jq-yamlformat"
	yamlformat "github.com/apstndb/go-yamlformat"
	"github.com/spf13/cobra"
)

// OutputFormat represents the output format for structured data
type OutputFormat string

const (
	FormatYAML     OutputFormat = "yaml"
	FormatJSON     OutputFormat = "json"
	FormatMarkdown OutputFormat = "markdown"

	// jqQueryTimeout is the maximum time allowed for jq query execution
	jqQueryTimeout = 30 * time.Second
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

	// Create pipeline with jq query
	pipeline, err := jqyaml.New(jqyaml.WithQuery(jqQuery))
	if err != nil {
		return fmt.Errorf("failed to create jq pipeline: %w", err)
	}

	// Convert OutputFormat to yamlformat.Format
	var yf yamlformat.Format
	switch format {
	case FormatJSON:
		yf = yamlformat.FormatJSON
	default:
		yf = yamlformat.FormatYAML
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), jqQueryTimeout)
	defer cancel()

	// Execute pipeline with writer option
	return pipeline.Execute(ctx, data, jqyaml.WithWriter(w, yf))
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