package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var labelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "GitHub label operations for Issues and Pull Requests",
	Long:  `Manage labels on GitHub Issues and Pull Requests with bulk operations support.`,
}

var addLabelsCmd = NewOperationalCommand(
	"add <label>[,<label>...] [flags]",
	"Add labels to multiple items",
	`Add one or more labels to multiple PRs and/or Issues in a single operation.

Examples:
  # Add multiple labels to multiple items (auto-detect type)
  gh-helper labels add bug,high-priority --items 254,267,238,245
  
  # Explicit type specification
  gh-helper labels add enhancement --items issue/301,pull/302,340
  
  # Pattern-based labeling
  gh-helper labels add enhancement --title-pattern "^feat:"`,
	addLabels,
)

var removeLabelsCmd = NewOperationalCommand(
	"remove <label>[,<label>...] [flags]",
	"Remove labels from multiple items",
	`Remove one or more labels from multiple PRs and/or Issues in a single operation.

Examples:
  # Remove multiple labels from multiple items
  gh-helper labels remove needs-review,waiting-on-author --items 254,267,238,245
  
  # Remove with explicit type specification
  gh-helper labels remove wip --items issue/301,pull/302`,
	removeLabels,
)

var addFromIssuesCmd = NewOperationalCommand(
	"add-from-issues [flags]",
	"Add labels from linked issues to PR",
	`Inherit labels from issues that a PR closes or references.

Examples:
  # Add labels from linked issues to a PR
  gh-helper labels add-from-issues --pr 254
  
  # Dry-run to see what would be added
  gh-helper labels add-from-issues --pr 254 --dry-run`,
	addFromIssues,
)

func init() {
	// Add flags for add command
	addLabelsCmd.Flags().String("items", "", "Comma-separated list of items (e.g., 254,issue/238,pull/267)")
	addLabelsCmd.Flags().String("title-pattern", "", "Regex pattern to match titles")
	addLabelsCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	addLabelsCmd.Flags().Bool("confirm", false, "Interactive confirmation for bulk operations")
	addLabelsCmd.Flags().Bool("parallel", true, "Execute mutations concurrently")
	addLabelsCmd.Flags().Int("max-concurrent", 5, "Maximum concurrent requests")

	// Add flags for remove command
	removeLabelsCmd.Flags().String("items", "", "Comma-separated list of items (e.g., 254,issue/238,pull/267)")
	removeLabelsCmd.Flags().String("title-pattern", "", "Regex pattern to match titles")
	removeLabelsCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	removeLabelsCmd.Flags().Bool("confirm", false, "Interactive confirmation for bulk operations")
	removeLabelsCmd.Flags().Bool("parallel", true, "Execute mutations concurrently")
	removeLabelsCmd.Flags().Int("max-concurrent", 5, "Maximum concurrent requests")

	// Add flags for add-from-issues command
	addFromIssuesCmd.Flags().Int("pr", 0, "Pull request number")
	if err := addFromIssuesCmd.MarkFlagRequired("pr"); err != nil {
		panic(fmt.Sprintf("failed to mark pr flag as required: %v", err))
	}
	addFromIssuesCmd.Flags().Bool("dry-run", false, "Show what would be added without making changes")

	// Add subcommands
	labelsCmd.AddCommand(addLabelsCmd, removeLabelsCmd, addFromIssuesCmd)
}

// LabelOperationResult represents the result of a label operation
type LabelOperationResult struct {
	Type          string   `json:"type"`
	Number        int      `json:"number"`
	Operation     string   `json:"operation"`
	LabelsAdded   []string `json:"labelsAdded,omitempty"`
	LabelsRemoved []string `json:"labelsRemoved,omitempty"`
	CurrentLabels []string `json:"currentLabels"`
	Status        string   `json:"status"`
	Error         string   `json:"error,omitempty"`
}

// LabelOperationSummary represents the summary of bulk operations
type LabelOperationSummary struct {
	LabelsModified []LabelOperationResult `json:"labelsModified"`
	Summary        struct {
		TotalItems      int      `json:"totalItems"`
		LabelsModified  []string `json:"labelsModified"`
		Successful      int      `json:"successful"`
		Failed          int      `json:"failed"`
	} `json:"summary"`
}

// ParseItemSpec parses an item specification like "254", "issue/238", "pull/267"
func ParseItemSpec(spec string) (itemType string, number int, err error) {
	spec = strings.TrimSpace(spec)
	
	// Check for explicit type specification
	if strings.HasPrefix(spec, "issue/") {
		itemType = "Issue"
		_, err = fmt.Sscanf(spec, "issue/%d", &number)
		return
	}
	if strings.HasPrefix(spec, "pull/") || strings.HasPrefix(spec, "pr/") {
		itemType = "PullRequest"
		if strings.HasPrefix(spec, "pull/") {
			_, err = fmt.Sscanf(spec, "pull/%d", &number)
		} else {
			_, err = fmt.Sscanf(spec, "pr/%d", &number)
		}
		return
	}
	
	// Plain number - type will be auto-detected
	_, err = fmt.Sscanf(spec, "%d", &number)
	return "", number, err
}

func addLabels(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("requires at least one label argument")
	}

	labels := strings.Split(args[0], ",")
	for i := range labels {
		labels[i] = strings.TrimSpace(labels[i])
	}

	items, _ := cmd.Flags().GetString("items")
	titlePattern, _ := cmd.Flags().GetString("title-pattern")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	parallel, _ := cmd.Flags().GetBool("parallel")
	maxConcurrent, _ := cmd.Flags().GetInt("max-concurrent")

	if items == "" && titlePattern == "" {
		return fmt.Errorf("either --items or --title-pattern must be specified")
	}

	client := NewGitHubClient(owner, repo)

	// Get repository info
	repoID, err := client.getRepositoryID()
	if err != nil {
		return err
	}

	// Get label IDs
	labelMap, err := client.GetLabelIDs(labels)
	if err != nil {
		return err
	}

	var labelIDs []string
	for _, label := range labels {
		if id, ok := labelMap[label]; ok {
			labelIDs = append(labelIDs, id)
		} else {
			return fmt.Errorf("label '%s' not found in repository", label)
		}
	}

	var itemsToProcess []ItemToLabel

	// Process --items flag
	if items != "" {
		itemSpecs := strings.Split(items, ",")
		for _, spec := range itemSpecs {
			itemType, number, err := ParseItemSpec(spec)
			if err != nil {
				return fmt.Errorf("invalid item specification '%s': %v", spec, err)
			}
			
			// Get item info (including type detection if needed)
			item, err := client.GetLabelableInfo(repoID, itemType, number)
			if err != nil {
				return fmt.Errorf("failed to get item %s: %v", spec, err)
			}
			
			itemsToProcess = append(itemsToProcess, ItemToLabel{
				ID:     item.ID,
				Number: item.Number,
				Type:   item.TypeName,
				Title:  item.Title,
			})
		}
	}

	// Process --title-pattern flag
	if titlePattern != "" {
		re, err := regexp.Compile(titlePattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %v", err)
		}

		// Search for items matching the pattern
		matchingItems, err := client.SearchItemsByTitle(repoID, re)
		if err != nil {
			return err
		}

		itemsToProcess = append(itemsToProcess, matchingItems...)
	}

	if len(itemsToProcess) == 0 {
		fmt.Println("No items found to process")
		return nil
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var uniqueItems []ItemToLabel
	for _, item := range itemsToProcess {
		if !seen[item.ID] {
			seen[item.ID] = true
			uniqueItems = append(uniqueItems, item)
		}
	}
	itemsToProcess = uniqueItems

	if dryRun {
		fmt.Printf("Would add labels %v to %d items:\n", labels, len(itemsToProcess))
		for _, item := range itemsToProcess {
			fmt.Printf("  - %s #%d: %s\n", item.Type, item.Number, item.Title)
		}
		return nil
	}

	// Execute label additions
	results := ExecuteParallel(
		itemsToProcess,
		func(item ItemToLabel) (LabelOperationResult, error) {
			result := LabelOperationResult{
				Type:      item.Type,
				Number:    item.Number,
				Operation: "add",
			}

			updatedItem, err := client.AddLabelsToItem(item.ID, labelIDs)
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				return result, nil
			}

			result.Status = "success"
			result.LabelsAdded = labels
			result.CurrentLabels = extractLabelNames(updatedItem.Labels.Nodes)
			return result, nil
		},
		parallel,
		maxConcurrent,
	)

	// Prepare output
	summary := LabelOperationSummary{}
	summary.Summary.TotalItems = len(results)
	summary.Summary.LabelsModified = labels
	
	for _, result := range results {
		summary.LabelsModified = append(summary.LabelsModified, result)
		if result.Status == "success" {
			summary.Summary.Successful++
		} else {
			summary.Summary.Failed++
		}
	}

	return EncodeOutput(os.Stdout, ResolveFormat(cmd), summary)
}

func removeLabels(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("requires at least one label argument")
	}

	labels := strings.Split(args[0], ",")
	for i := range labels {
		labels[i] = strings.TrimSpace(labels[i])
	}

	items, _ := cmd.Flags().GetString("items")
	titlePattern, _ := cmd.Flags().GetString("title-pattern")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	parallel, _ := cmd.Flags().GetBool("parallel")
	maxConcurrent, _ := cmd.Flags().GetInt("max-concurrent")

	if items == "" && titlePattern == "" {
		return fmt.Errorf("either --items or --title-pattern must be specified")
	}

	client := NewGitHubClient(owner, repo)

	// Get repository info
	repoID, err := client.getRepositoryID()
	if err != nil {
		return err
	}

	// Get label IDs
	labelMap, err := client.GetLabelIDs(labels)
	if err != nil {
		return err
	}

	var labelIDs []string
	for _, label := range labels {
		if id, ok := labelMap[label]; ok {
			labelIDs = append(labelIDs, id)
		} else {
			// Skip non-existent labels for remove operation
			continue
		}
	}

	if len(labelIDs) == 0 {
		fmt.Println("No valid labels to remove")
		return nil
	}

	var itemsToProcess []ItemToLabel

	// Process --items flag
	if items != "" {
		itemSpecs := strings.Split(items, ",")
		for _, spec := range itemSpecs {
			itemType, number, err := ParseItemSpec(spec)
			if err != nil {
				return fmt.Errorf("invalid item specification '%s': %v", spec, err)
			}
			
			// Get item info (including type detection if needed)
			item, err := client.GetLabelableInfo(repoID, itemType, number)
			if err != nil {
				return fmt.Errorf("failed to get item %s: %v", spec, err)
			}
			
			itemsToProcess = append(itemsToProcess, ItemToLabel{
				ID:     item.ID,
				Number: item.Number,
				Type:   item.TypeName,
				Title:  item.Title,
			})
		}
	}

	// Process --title-pattern flag
	if titlePattern != "" {
		re, err := regexp.Compile(titlePattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %v", err)
		}

		// Search for items matching the pattern
		matchingItems, err := client.SearchItemsByTitle(repoID, re)
		if err != nil {
			return err
		}

		itemsToProcess = append(itemsToProcess, matchingItems...)
	}

	if len(itemsToProcess) == 0 {
		fmt.Println("No items found to process")
		return nil
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var uniqueItems []ItemToLabel
	for _, item := range itemsToProcess {
		if !seen[item.ID] {
			seen[item.ID] = true
			uniqueItems = append(uniqueItems, item)
		}
	}
	itemsToProcess = uniqueItems

	if dryRun {
		fmt.Printf("Would remove labels %v from %d items:\n", labels, len(itemsToProcess))
		for _, item := range itemsToProcess {
			fmt.Printf("  - %s #%d: %s\n", item.Type, item.Number, item.Title)
		}
		return nil
	}

	// Execute label removals
	results := ExecuteParallel(
		itemsToProcess,
		func(item ItemToLabel) (LabelOperationResult, error) {
			result := LabelOperationResult{
				Type:      item.Type,
				Number:    item.Number,
				Operation: "remove",
			}

			updatedItem, err := client.RemoveLabelsFromItem(item.ID, labelIDs)
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				return result, nil
			}

			result.Status = "success"
			result.LabelsRemoved = labels
			result.CurrentLabels = extractLabelNames(updatedItem.Labels.Nodes)
			return result, nil
		},
		parallel,
		maxConcurrent,
	)

	// Prepare output
	summary := LabelOperationSummary{}
	summary.Summary.TotalItems = len(results)
	summary.Summary.LabelsModified = labels
	
	for _, result := range results {
		summary.LabelsModified = append(summary.LabelsModified, result)
		if result.Status == "success" {
			summary.Summary.Successful++
		} else {
			summary.Summary.Failed++
		}
	}

	return EncodeOutput(os.Stdout, ResolveFormat(cmd), summary)
}

func addFromIssues(cmd *cobra.Command, args []string) error {
	prNumber, _ := cmd.Flags().GetInt("pr")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	client := NewGitHubClient(owner, repo)

	// Get PR info with linked issues
	pr, err := client.GetPRWithLinkedIssues(prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR %d: %v", prNumber, err)
	}

	// Collect all unique labels from linked issues
	labelSet := make(map[string]bool)
	existingLabels := make(map[string]bool)
	
	// Track existing labels on PR
	for _, label := range pr.Labels.Nodes {
		existingLabels[label.Name] = true
	}

	// Collect labels from linked issues
	for _, issue := range pr.ClosingIssuesReferences.Nodes {
		for _, label := range issue.Labels.Nodes {
			if !existingLabels[label.Name] {
				labelSet[label.Name] = true
			}
		}
	}

	// Convert to slice
	var labelsToAdd []string
	for label := range labelSet {
		labelsToAdd = append(labelsToAdd, label)
	}

	if len(labelsToAdd) == 0 {
		fmt.Printf("PR #%d already has all labels from its linked issues\n", prNumber)
		return nil
	}

	if dryRun {
		fmt.Printf("Would add the following labels to PR #%d:\n", prNumber)
		for _, label := range labelsToAdd {
			fmt.Printf("  - %s\n", label)
		}
		fmt.Printf("\nLabels inherited from %d linked issue(s)\n", len(pr.ClosingIssuesReferences.Nodes))
		return nil
	}

	// Get label IDs
	labelMap, err := client.GetLabelIDs(labelsToAdd)
	if err != nil {
		return err
	}

	var labelIDs []string
	for _, label := range labelsToAdd {
		if id, ok := labelMap[label]; ok {
			labelIDs = append(labelIDs, id)
		}
	}

	// Add labels to PR
	updatedPR, err := client.AddLabelsToItem(pr.ID, labelIDs)
	if err != nil {
		return fmt.Errorf("failed to add labels: %v", err)
	}

	// Prepare output
	result := LabelOperationResult{
		Type:          "PullRequest",
		Number:        prNumber,
		Operation:     "add-from-issues",
		LabelsAdded:   labelsToAdd,
		CurrentLabels: extractLabelNames(updatedPR.Labels.Nodes),
		Status:        "success",
	}

	return EncodeOutput(os.Stdout, ResolveFormat(cmd), result)
}

// Helper function to extract label names from nodes
func extractLabelNames(nodes []Label) []string {
	names := make([]string, len(nodes))
	for i, node := range nodes {
		names[i] = node.Name
	}
	return names
}

// ItemToLabel represents an item to be labeled
type ItemToLabel struct {
	ID     string
	Number int
	Type   string
	Title  string
}