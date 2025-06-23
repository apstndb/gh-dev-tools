package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var nodeIDCmd = &cobra.Command{
	Use:   "node-id",
	Short: "Get GitHub node IDs for GraphQL operations",
	Long:  `Retrieve node IDs for issues and pull requests to use in GraphQL mutations.`,
}

var nodeIDIssueCmd = NewOperationalCommand(
	"issue <number>",
	"Get node ID for an issue",
	`Get the GraphQL node ID for a specific issue.

Example:
  gh-helper node-id issue 248`,
	nodeIDIssue,
)

var nodeIDPRCmd = NewOperationalCommand(
	"pr <number>",
	"Get node ID for a pull request",
	`Get the GraphQL node ID for a specific pull request.

Example:
  gh-helper node-id pr 312`,
	nodeIDPR,
)

func init() {
	// Add batch flag to root node-id command
	nodeIDCmd.Flags().String("batch", "", "Batch resolve multiple items (e.g., issue:123,pr:456,issue:789)")
	
	// Set custom run function for batch processing
	nodeIDCmd.Run = func(cmd *cobra.Command, args []string) {
		batch, _ := cmd.Flags().GetString("batch")
		if batch != "" {
			if err := nodeIDBatch(cmd, batch); err != nil {
				ErrorMsg("Error: %v", err).Print()
				os.Exit(1)
			}
		} else {
			_ = cmd.Help()
		}
	}
	
	// Add subcommands
	nodeIDCmd.AddCommand(nodeIDIssueCmd)
	nodeIDCmd.AddCommand(nodeIDPRCmd)
}

// NodeIDResult represents a single node ID result
type NodeIDResult struct {
	Type   string `json:"type"`
	Number int    `json:"number"`
	ID     string `json:"id"`
}

// NodeIDResponse represents the response for node ID queries
type NodeIDResponse struct {
	NodeID  *NodeIDResult   `json:"nodeId,omitempty"`  // For single queries
	NodeIDs []NodeIDResult  `json:"nodeIds,omitempty"`  // For batch queries
}

func nodeIDIssue(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("issue number is required")
	}
	
	number, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid issue number: %s", args[0])
	}
	
	client := NewGitHubClient(owner, repo)
	
	query := `
	query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				id
			}
		}
	}`
	
	variables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"number": number,
	}
	
	data, err := client.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return fmt.Errorf("failed to fetch issue: %w", err)
	}
	
	var response struct {
		Data struct {
			Repository struct {
				Issue *struct {
					ID string `json:"id"`
				} `json:"issue"`
			} `json:"repository"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	if response.Data.Repository.Issue == nil {
		return fmt.Errorf("issue not found: #%d", number)
	}
	
	result := NodeIDResponse{
		NodeID: &NodeIDResult{
			Type:   "Issue",
			Number: number,
			ID:     response.Data.Repository.Issue.ID,
		},
	}
	
	format := ResolveFormat(cmd)
	return EncodeOutput(os.Stdout, format, result)
}

func nodeIDPR(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("pull request number is required")
	}
	
	number, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid pull request number: %s", args[0])
	}
	
	client := NewGitHubClient(owner, repo)
	
	query := `
	query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			pullRequest(number: $number) {
				id
			}
		}
	}`
	
	variables := map[string]interface{}{
		"owner":  owner,
		"repo":   repo,
		"number": number,
	}
	
	data, err := client.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return fmt.Errorf("failed to fetch pull request: %w", err)
	}
	
	var response struct {
		Data struct {
			Repository struct {
				PullRequest *struct {
					ID string `json:"id"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	if response.Data.Repository.PullRequest == nil {
		return fmt.Errorf("pull request not found: #%d", number)
	}
	
	result := NodeIDResponse{
		NodeID: &NodeIDResult{
			Type:   "PullRequest",
			Number: number,
			ID:     response.Data.Repository.PullRequest.ID,
		},
	}
	
	format := ResolveFormat(cmd)
	return EncodeOutput(os.Stdout, format, result)
}

func nodeIDBatch(cmd *cobra.Command, batch string) error {
	// Parse batch string
	items := strings.Split(batch, ",")
	if len(items) == 0 {
		return fmt.Errorf("no items specified in batch")
	}
	
	type batchItem struct {
		Type   string
		Number int
		Alias  string
	}
	
	var batchItems []batchItem
	for i, item := range items {
		parts := strings.Split(strings.TrimSpace(item), ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid batch item format: %s (expected type:number)", item)
		}
		
		itemType := strings.ToLower(parts[0])
		if itemType != "issue" && itemType != "pr" {
			return fmt.Errorf("invalid type: %s (expected 'issue' or 'pr')", parts[0])
		}
		
		number, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid number in batch item %s: %w", item, err)
		}
		
		batchItems = append(batchItems, batchItem{
			Type:   itemType,
			Number: number,
			Alias:  fmt.Sprintf("item%d", i),
		})
	}
	
	// Build batch query using aliases
	var queryBuilder strings.Builder
	queryBuilder.WriteString("query($owner: String!, $repo: String!) {\n")
	queryBuilder.WriteString("  repository(owner: $owner, name: $repo) {\n")
	
	for _, item := range batchItems {
		if item.Type == "issue" {
			fmt.Fprintf(&queryBuilder, "    %s: issue(number: %d) { id }\n", item.Alias, item.Number)
		} else {
			fmt.Fprintf(&queryBuilder, "    %s: pullRequest(number: %d) { id }\n", item.Alias, item.Number)
		}
	}
	
	queryBuilder.WriteString("  }\n")
	queryBuilder.WriteString("}")
	
	client := NewGitHubClient(owner, repo)
	
	variables := map[string]interface{}{
		"owner": owner,
		"repo":  repo,
	}
	
	data, err := client.RunGraphQLQueryWithVariables(queryBuilder.String(), variables)
	if err != nil {
		return fmt.Errorf("failed to fetch items: %w", err)
	}
	
	// Parse response
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(data, &rawResponse); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	dataMap, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected response format")
	}
	
	repoMap, ok := dataMap["repository"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("repository not found in response")
	}
	
	// Build results
	var results []NodeIDResult
	for _, item := range batchItems {
		if itemData, exists := repoMap[item.Alias]; exists && itemData != nil {
			if itemMap, ok := itemData.(map[string]interface{}); ok {
				if id, ok := itemMap["id"].(string); ok {
					resultType := "Issue"
					if item.Type == "pr" {
						resultType = "PullRequest"
					}
					results = append(results, NodeIDResult{
						Type:   resultType,
						Number: item.Number,
						ID:     id,
					})
				}
			}
		}
	}
	
	response := NodeIDResponse{
		NodeIDs: results,
	}
	
	format := ResolveFormat(cmd)
	return EncodeOutput(os.Stdout, format, response)
}