package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var releasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "GitHub release management operations",
	Long:  `Analyze and manage GitHub releases, including release notes categorization and label validation.`,
}

var analyzeReleaseCmd = NewOperationalCommand(
	"analyze [flags]",
	"Analyze PRs for release notes categorization",
	`Analyze merged PRs to ensure appropriate labels for release notes categorization.

This command helps identify:
- PRs missing classification labels (bug, enhancement, feature)
- PRs that should have 'ignore-for-release' label
- Inconsistent labeling patterns
- Release readiness status

Examples:
  # Analyze release candidates by milestone
  gh-helper releases analyze --milestone v0.19.0
  
  # Analyze by date range
  gh-helper releases analyze --since 2024-01-01 --until 2024-01-31
  
  # Output suggestions in markdown format
  gh-helper releases analyze --milestone v0.19.0 --format markdown
  
  # Analyze specific PR range
  gh-helper releases analyze --pr-range 250-300`,
	analyzeRelease,
)

func init() {
	// Configure flags for analyze command
	analyzeReleaseCmd.Flags().String("milestone", "", "Milestone title to analyze")
	analyzeReleaseCmd.Flags().String("since", "", "Start date (YYYY-MM-DD)")
	analyzeReleaseCmd.Flags().String("until", "", "End date (YYYY-MM-DD)")
	analyzeReleaseCmd.Flags().String("pr-range", "", "PR number range (e.g., 250-300)")
	analyzeReleaseCmd.Flags().Bool("include-drafts", false, "Include draft PRs in analysis")

	// Add subcommands
	releasesCmd.AddCommand(analyzeReleaseCmd)
}

// ReleaseAnalysis represents the complete analysis results
type ReleaseAnalysis struct {
	Milestone             string                    `json:"milestone,omitempty" yaml:"milestone,omitempty"`
	DateRange             *DateRange                `json:"dateRange,omitempty" yaml:"dateRange,omitempty"`
	TotalPRs              int                       `json:"totalPRs" yaml:"totalPRs"`
	MissingClassification []PRClassificationSuggestion `json:"missingClassification,omitempty" yaml:"missingClassification,omitempty"`
	ShouldIgnore          []PRIgnoreSuggestion         `json:"shouldIgnore,omitempty" yaml:"shouldIgnore,omitempty"`
	InconsistentLabeling  []PRInconsistency           `json:"inconsistentLabeling,omitempty" yaml:"inconsistentLabeling,omitempty"`
	Summary               ReleaseSummary              `json:"summary" yaml:"summary"`
}

// DateRange represents a date range for analysis
type DateRange struct {
	Since string `json:"since" yaml:"since"`
	Until string `json:"until" yaml:"until"`
}

// PRClassificationSuggestion represents a PR that needs a classification label
type PRClassificationSuggestion struct {
	Number         int    `json:"number" yaml:"number"`
	Title          string `json:"title" yaml:"title"`
	SuggestedLabel string `json:"suggestedLabel" yaml:"suggestedLabel"`
	Reasoning      string `json:"reasoning" yaml:"reasoning"`
}

// PRIgnoreSuggestion represents a PR that should be ignored in release notes
type PRIgnoreSuggestion struct {
	Number         int    `json:"number" yaml:"number"`
	Title          string `json:"title" yaml:"title"`
	SuggestedLabel string `json:"suggestedLabel" yaml:"suggestedLabel"`
	Reasoning      string `json:"reasoning" yaml:"reasoning"`
}

// PRInconsistency represents a PR with inconsistent labeling
type PRInconsistency struct {
	Number        int      `json:"number" yaml:"number"`
	Title         string   `json:"title" yaml:"title"`
	CurrentLabels []string `json:"currentLabels" yaml:"currentLabels"`
	Issue         string   `json:"issue" yaml:"issue"`
	Suggestion    string   `json:"suggestion" yaml:"suggestion"`
}

// ReleaseSummary provides summary statistics
type ReleaseSummary struct {
	WellLabeled      int  `json:"wellLabeled" yaml:"wellLabeled"`
	NeedsAttention   int  `json:"needsAttention" yaml:"needsAttention"`
	ReadyForRelease  bool `json:"readyForRelease" yaml:"readyForRelease"`
}

// PRData represents a PR with its metadata for analysis
type PRData struct {
	Number       int      `json:"number"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	Labels       []string `json:"labels"`
	LinkedIssues []LinkedIssue `json:"linkedIssues"`
	MergedAt     string   `json:"mergedAt"`
	Author       string   `json:"author"`
}

// LinkedIssue represents an issue linked to a PR
type LinkedIssue struct {
	Number int      `json:"number"`
	Labels []string `json:"labels"`
}

func analyzeRelease(cmd *cobra.Command, args []string) error {
	// Get flags
	milestone, err := cmd.Flags().GetString("milestone")
	if err != nil {
		return fmt.Errorf("failed to get 'milestone' flag: %w", err)
	}
	since, err := cmd.Flags().GetString("since")
	if err != nil {
		return fmt.Errorf("failed to get 'since' flag: %w", err)
	}
	until, err := cmd.Flags().GetString("until")
	if err != nil {
		return fmt.Errorf("failed to get 'until' flag: %w", err)
	}
	prRange, err := cmd.Flags().GetString("pr-range")
	if err != nil {
		return fmt.Errorf("failed to get 'pr-range' flag: %w", err)
	}
	includeDrafts, err := cmd.Flags().GetBool("include-drafts")
	if err != nil {
		return fmt.Errorf("failed to get 'include-drafts' flag: %w", err)
	}

	// Validate input - must specify exactly one filter
	filters := 0
	if milestone != "" {
		filters++
	}
	if since != "" || until != "" {
		filters++
	}
	if prRange != "" {
		filters++
	}

	if filters == 0 {
		return fmt.Errorf("must specify one of --milestone, --since/--until, or --pr-range")
	}
	if filters > 1 {
		return fmt.Errorf("specify only one of --milestone, --since/--until, or --pr-range")
	}

	// Create GitHub client
	client := NewGitHubClient(owner, repo)

	// Fetch PRs based on filter
	var prs []PRData

	switch {
	case milestone != "":
		prs, err = client.GetPRsByMilestone(milestone, includeDrafts)
		if err != nil {
			return fmt.Errorf("failed to fetch PRs for milestone %s: %w", milestone, err)
		}

	case since != "" || until != "":
		// Parse dates
		var sinceTime, untilTime time.Time
		if since != "" {
			sinceTime, err = time.Parse("2006-01-02", since)
			if err != nil {
				return fmt.Errorf("invalid since date format: %w", err)
			}
		}
		if until != "" {
			untilTime, err = time.Parse("2006-01-02", until)
			if err != nil {
				return fmt.Errorf("invalid until date format: %w", err)
			}
			// Add 1 day to include the entire "until" day
			untilTime = untilTime.Add(24 * time.Hour)
		}

		prs, err = client.GetPRsByDateRange(sinceTime, untilTime, includeDrafts)
		if err != nil {
			return fmt.Errorf("failed to fetch PRs for date range: %w", err)
		}

	case prRange != "":
		// Parse range
		var start, end int
		_, err := fmt.Sscanf(prRange, "%d-%d", &start, &end)
		if err != nil {
			return fmt.Errorf("invalid PR range format (expected: 123-456): %w", err)
		}

		prs, err = client.GetPRsByRange(start, end, includeDrafts)
		if err != nil {
			return fmt.Errorf("failed to fetch PRs for range %s: %w", prRange, err)
		}
	}

	// Analyze PRs
	analysis := analyzePRs(prs)

	// Set metadata
	if milestone != "" {
		analysis.Milestone = milestone
	}
	if since != "" || until != "" {
		analysis.DateRange = &DateRange{
			Since: since,
			Until: until,
		}
	}

	// Output results
	// Special handling for markdown format
	if ResolveFormat(cmd) == FormatMarkdown {
		return outputMarkdownAnalysis(analysis)
	}

	output := map[string]interface{}{
		"releaseAnalysis": analysis,
	}

	return EncodeOutputWithCmd(cmd, output)
}

// analyzePRs performs the analysis on a set of PRs
func analyzePRs(prs []PRData) ReleaseAnalysis {
	analysis := ReleaseAnalysis{
		TotalPRs:              len(prs),
		MissingClassification: []PRClassificationSuggestion{},
		ShouldIgnore:          []PRIgnoreSuggestion{},
		InconsistentLabeling:  []PRInconsistency{},
	}

	classificationLabels := []string{"bug", "enhancement", "feature", "documentation", "chore", "refactor"}
	ignoreIndicators := []string{
		"CLAUDE.md",
		"dev-docs/",
		".github/workflows/",
		"Makefile",
		".gitignore",
		"go.mod",
		"go.sum",
	}

	for _, pr := range prs {
		hasClassification := false
		hasIgnore := hasLabel(pr.Labels, "ignore-for-release")
		isDocumentation := hasLabel(pr.Labels, "documentation")
		
		// Check if PR has classification label
		for _, label := range classificationLabels {
			if hasLabel(pr.Labels, label) {
				hasClassification = true
				break
			}
		}

		// Check if should be ignored first
		shouldIgnore := false
		ignoreReason := ""
		
		// Check title patterns
		lowerTitle := strings.ToLower(pr.Title)
		if strings.HasPrefix(lowerTitle, "docs:") && !isUserFacingDoc(pr.Title, pr.Body) {
			shouldIgnore = true
			ignoreReason = "Development documentation"
		}
		
		// Check file patterns in title or body
		for _, indicator := range ignoreIndicators {
			if strings.Contains(pr.Title, indicator) || strings.Contains(pr.Body, indicator) {
				shouldIgnore = true
				ignoreReason = fmt.Sprintf("Internal file changes (%s)", indicator)
				break
			}
		}

		if shouldIgnore && !hasIgnore {
			analysis.ShouldIgnore = append(analysis.ShouldIgnore, PRIgnoreSuggestion{
				Number:         pr.Number,
				Title:          pr.Title,
				SuggestedLabel: "ignore-for-release",
				Reasoning:      ignoreReason,
			})
		}

		// Analyze missing classification (skip if should be ignored or already has ignore label)
		if !hasClassification && !hasIgnore && !shouldIgnore {
			suggestion := suggestClassificationLabel(pr)
			if suggestion != nil {
				analysis.MissingClassification = append(analysis.MissingClassification, *suggestion)
			}
		}

		// Check for inconsistent labeling
		if isDocumentation && hasIgnore && isUserFacingDoc(pr.Title, pr.Body) {
			analysis.InconsistentLabeling = append(analysis.InconsistentLabeling, PRInconsistency{
				Number:        pr.Number,
				Title:         pr.Title,
				CurrentLabels: pr.Labels,
				Issue:         "User-facing documentation marked as ignore",
				Suggestion:    "Remove 'ignore-for-release' label",
			})
		}
	}

	// Calculate summary
	// Count unique PRs that need attention (some PRs might be in multiple categories)
	needsAttentionSet := make(map[int]bool)
	for _, pr := range analysis.MissingClassification {
		needsAttentionSet[pr.Number] = true
	}
	for _, pr := range analysis.ShouldIgnore {
		needsAttentionSet[pr.Number] = true
	}
	for _, pr := range analysis.InconsistentLabeling {
		needsAttentionSet[pr.Number] = true
	}
	
	needsAttention := len(needsAttentionSet)
	analysis.Summary = ReleaseSummary{
		WellLabeled:     analysis.TotalPRs - needsAttention,
		NeedsAttention:  needsAttention,
		ReadyForRelease: needsAttention == 0,
	}

	return analysis
}

// suggestClassificationLabel suggests a label based on PR content
func suggestClassificationLabel(pr PRData) *PRClassificationSuggestion {
	lowerTitle := strings.ToLower(pr.Title)
	
	// Check conventional commit prefixes
	if strings.HasPrefix(lowerTitle, "feat:") || strings.HasPrefix(lowerTitle, "feature:") {
		return &PRClassificationSuggestion{
			Number:         pr.Number,
			Title:          pr.Title,
			SuggestedLabel: "enhancement",
			Reasoning:      "Title starts with 'feat:' prefix",
		}
	}
	
	if strings.HasPrefix(lowerTitle, "fix:") || strings.HasPrefix(lowerTitle, "bugfix:") {
		return &PRClassificationSuggestion{
			Number:         pr.Number,
			Title:          pr.Title,
			SuggestedLabel: "bug",
			Reasoning:      "Title starts with 'fix:' prefix",
		}
	}
	
	if strings.HasPrefix(lowerTitle, "docs:") {
		return &PRClassificationSuggestion{
			Number:         pr.Number,
			Title:          pr.Title,
			SuggestedLabel: "documentation",
			Reasoning:      "Title starts with 'docs:' prefix",
		}
	}
	
	if strings.HasPrefix(lowerTitle, "chore:") || strings.HasPrefix(lowerTitle, "ci:") {
		return &PRClassificationSuggestion{
			Number:         pr.Number,
			Title:          pr.Title,
			SuggestedLabel: "chore",
			Reasoning:      "Title indicates maintenance work",
		}
	}
	
	// Check linked issues
	for _, issue := range pr.LinkedIssues {
		for _, label := range issue.Labels {
			if label == "bug" {
				return &PRClassificationSuggestion{
					Number:         pr.Number,
					Title:          pr.Title,
					SuggestedLabel: "bug",
					Reasoning:      fmt.Sprintf("Fixes issue #%d which has 'bug' label", issue.Number),
				}
			}
			if label == "enhancement" || label == "feature" {
				return &PRClassificationSuggestion{
					Number:         pr.Number,
					Title:          pr.Title,
					SuggestedLabel: "enhancement",
					Reasoning:      fmt.Sprintf("Implements issue #%d which has '%s' label", issue.Number, label),
				}
			}
		}
	}
	
	// Default suggestion based on keywords
	if strings.Contains(lowerTitle, "add") || strings.Contains(lowerTitle, "implement") {
		return &PRClassificationSuggestion{
			Number:         pr.Number,
			Title:          pr.Title,
			SuggestedLabel: "enhancement",
			Reasoning:      "Title suggests new functionality",
		}
	}
	
	return nil
}

// isUserFacingDoc checks if documentation is user-facing
func isUserFacingDoc(title, body string) bool {
	userFacingIndicators := []string{
		"README",
		"user guide",
		"tutorial",
		"example",
		"getting started",
		"installation",
		"usage",
	}
	
	combined := strings.ToLower(title + " " + body)
	for _, indicator := range userFacingIndicators {
		if strings.Contains(combined, strings.ToLower(indicator)) {
			return true
		}
	}
	
	return false
}

// hasLabel checks if a label exists in the list
func hasLabel(labels []string, target string) bool {
	targetLower := strings.ToLower(target)
	for _, label := range labels {
		if strings.ToLower(label) == targetLower {
			return true
		}
	}
	return false
}

// outputMarkdownAnalysis outputs the analysis in markdown format
func outputMarkdownAnalysis(analysis ReleaseAnalysis) error {
	fmt.Printf("# Release Notes Analysis")
	if analysis.Milestone != "" {
		fmt.Printf(" for %s", analysis.Milestone)
	}
	if analysis.DateRange != nil {
		fmt.Printf(" (%s to %s)", analysis.DateRange.Since, analysis.DateRange.Until)
	}
	fmt.Printf("\n\n")
	
	fmt.Printf("**Total PRs analyzed**: %d\n\n", analysis.TotalPRs)
	
	if len(analysis.MissingClassification) > 0 {
		fmt.Printf("## PRs Missing Primary Classification\n\n")
		for _, pr := range analysis.MissingClassification {
			fmt.Printf("- **#%d**: %s\n", pr.Number, pr.Title)
			fmt.Printf("  - **Suggested**: `%s`\n", pr.SuggestedLabel)
			fmt.Printf("  - **Reason**: %s\n\n", pr.Reasoning)
		}
	}
	
	if len(analysis.ShouldIgnore) > 0 {
		fmt.Printf("## PRs That Should Have 'ignore-for-release'\n\n")
		for _, pr := range analysis.ShouldIgnore {
			fmt.Printf("- **#%d**: %s\n", pr.Number, pr.Title)
			fmt.Printf("  - **Reason**: %s\n\n", pr.Reasoning)
		}
	}
	
	if len(analysis.InconsistentLabeling) > 0 {
		fmt.Printf("## PRs With Inconsistent Labeling\n\n")
		for _, pr := range analysis.InconsistentLabeling {
			fmt.Printf("- **#%d**: %s\n", pr.Number, pr.Title)
			fmt.Printf("  - **Current labels**: %s\n", strings.Join(pr.CurrentLabels, ", "))
			fmt.Printf("  - **Issue**: %s\n", pr.Issue)
			fmt.Printf("  - **Suggestion**: %s\n\n", pr.Suggestion)
		}
	}
	
	fmt.Printf("## Summary\n\n")
	fmt.Printf("- **Well-labeled PRs**: %d\n", analysis.Summary.WellLabeled)
	fmt.Printf("- **PRs needing attention**: %d\n", analysis.Summary.NeedsAttention)
	fmt.Printf("- **Ready for release**: %s\n", formatBool(analysis.Summary.ReadyForRelease))
	
	if !analysis.Summary.ReadyForRelease {
		fmt.Printf("\n⚠️ **Action Required**: Please review and apply the suggested labels before creating release notes.\n")
	}
	
	return nil
}

// formatBool returns a user-friendly boolean representation
func formatBool(b bool) string {
	if b {
		return "✅ Yes"
	}
	return "❌ No"
}

// GitHub API methods for fetching PRs

// GetPRsByMilestone fetches all PRs for a specific milestone
func (c *GitHubClient) GetPRsByMilestone(milestone string, includeDrafts bool) ([]PRData, error) {
	query := `
	query($owner: String!, $repo: String!, $milestone: String!) {
		repository(owner: $owner, name: $repo) {
			milestones(query: $milestone, first: 1) {
				nodes {
					pullRequests(first: 100, states: MERGED) {
						nodes {
							number
							title
							body
							mergedAt
							author {
								login
							}
							labels(first: 20) {
								nodes {
									name
								}
							}
							closingIssuesReferences(first: 10) {
								nodes {
									number
									labels(first: 20) {
										nodes {
											name
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner":     c.Owner,
		"repo":      c.Repo,
		"milestone": milestone,
	}

	responseData, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data struct {
			Repository struct {
				Milestones struct {
					Nodes []struct {
						PullRequests struct {
							Nodes []struct {
								Number    int    `json:"number"`
								Title     string `json:"title"`
								Body      string `json:"body"`
								MergedAt  string `json:"mergedAt"`
								Author    struct {
									Login string `json:"login"`
								} `json:"author"`
								Labels struct {
									Nodes []struct {
										Name string `json:"name"`
									} `json:"nodes"`
								} `json:"labels"`
								ClosingIssuesReferences struct {
									Nodes []struct {
										Number int `json:"number"`
										Labels struct {
											Nodes []struct {
												Name string `json:"name"`
											} `json:"nodes"`
										} `json:"labels"`
									} `json:"nodes"`
								} `json:"closingIssuesReferences"`
							} `json:"nodes"`
						} `json:"pullRequests"`
					} `json:"nodes"`
				} `json:"milestones"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, err
	}

	if len(response.Data.Repository.Milestones.Nodes) == 0 {
		return nil, fmt.Errorf("milestone not found: %s", milestone)
	}

	// Convert to PRData
	var prs []PRData
	for _, node := range response.Data.Repository.Milestones.Nodes[0].PullRequests.Nodes {
		pr := PRData{
			Number:   node.Number,
			Title:    node.Title,
			Body:     node.Body,
			MergedAt: node.MergedAt,
			Author:   node.Author.Login,
		}

		// Extract labels
		for _, label := range node.Labels.Nodes {
			if label.Name != "" {
				pr.Labels = append(pr.Labels, label.Name)
			}
		}

		// Extract linked issues
		for _, issue := range node.ClosingIssuesReferences.Nodes {
			linkedIssue := LinkedIssue{
				Number: issue.Number,
			}
			for _, label := range issue.Labels.Nodes {
				linkedIssue.Labels = append(linkedIssue.Labels, label.Name)
			}
			pr.LinkedIssues = append(pr.LinkedIssues, linkedIssue)
		}

		prs = append(prs, pr)
	}

	return prs, nil
}

// GetPRsByDateRange fetches PRs merged within a date range
func (c *GitHubClient) GetPRsByDateRange(since, until time.Time, includeDrafts bool) ([]PRData, error) {
	searchQuery := fmt.Sprintf("repo:%s/%s is:pr is:merged", c.Owner, c.Repo)
	
	if !since.IsZero() {
		searchQuery += fmt.Sprintf(" merged:>=%s", since.Format("2006-01-02"))
	}
	if !until.IsZero() {
		searchQuery += fmt.Sprintf(" merged:<%s", until.Format("2006-01-02"))
	}

	query := `
	query($searchQuery: String!) {
		search(query: $searchQuery, type: ISSUE, first: 100) {
			nodes {
				... on PullRequest {
					number
					title
					body
					mergedAt
					author {
						login
					}
					labels(first: 20) {
						nodes {
							name
						}
					}
					closingIssuesReferences(first: 10) {
						nodes {
							number
							labels(first: 20) {
								nodes {
									name
								}
							}
						}
					}
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"searchQuery": searchQuery,
	}

	responseData, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data struct {
			Search struct {
				Nodes []struct {
					Number    int    `json:"number"`
					Title     string `json:"title"`
					Body      string `json:"body"`
					MergedAt  string `json:"mergedAt"`
					Author    struct {
						Login string `json:"login"`
					} `json:"author"`
					Labels struct {
						Nodes []struct {
							Name string `json:"name"`
						} `json:"nodes"`
					} `json:"labels"`
					ClosingIssuesReferences struct {
						Nodes []struct {
							Number int `json:"number"`
							Labels struct {
								Nodes []struct {
									Name string `json:"name"`
								} `json:"nodes"`
							} `json:"labels"`
						} `json:"nodes"`
					} `json:"closingIssuesReferences"`
				} `json:"nodes"`
			} `json:"search"`
		} `json:"data"`
	}

	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, err
	}

	// Convert to PRData
	var prs []PRData
	for _, node := range response.Data.Search.Nodes {
		pr := PRData{
			Number:   node.Number,
			Title:    node.Title,
			Body:     node.Body,
			MergedAt: node.MergedAt,
			Author:   node.Author.Login,
		}

		// Extract labels
		for _, label := range node.Labels.Nodes {
			if label.Name != "" {
				pr.Labels = append(pr.Labels, label.Name)
			}
		}

		// Extract linked issues
		for _, issue := range node.ClosingIssuesReferences.Nodes {
			linkedIssue := LinkedIssue{
				Number: issue.Number,
			}
			for _, label := range issue.Labels.Nodes {
				linkedIssue.Labels = append(linkedIssue.Labels, label.Name)
			}
			pr.LinkedIssues = append(pr.LinkedIssues, linkedIssue)
		}

		prs = append(prs, pr)
	}

	// Sort by PR number
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].Number < prs[j].Number
	})

	return prs, nil
}

// GetPRsByRange fetches PRs within a number range
func (c *GitHubClient) GetPRsByRange(start, end int, includeDrafts bool) ([]PRData, error) {
	var allPRs []PRData
	
	// Fetch PRs in batches
	for i := start; i <= end; i++ {
		query := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				pullRequest(number: $number) {
					number
					title
					body
					state
					mergedAt
					author {
						login
					}
					labels(first: 20) {
						nodes {
							name
						}
					}
					closingIssuesReferences(first: 10) {
						nodes {
							number
							labels(first: 20) {
								nodes {
									name
								}
							}
						}
					}
				}
			}
		}`

		variables := map[string]interface{}{
			"owner":  c.Owner,
			"repo":   c.Repo,
			"number": i,
		}

		responseData, err := c.RunGraphQLQueryWithVariables(query, variables)
		if err != nil {
			// Log error and skip if PR doesn't exist
			fmt.Printf("Error fetching PR #%d: %v\n", i, err)
			continue
		}

		var response struct {
			Data struct {
				Repository struct {
					PullRequest *struct {
						Number    int    `json:"number"`
						Title     string `json:"title"`
						Body      string `json:"body"`
						State     string `json:"state"`
						MergedAt  string `json:"mergedAt"`
						Author    struct {
							Login string `json:"login"`
						} `json:"author"`
						Labels struct {
							Nodes []struct {
								Name string `json:"name"`
							} `json:"nodes"`
						} `json:"labels"`
						ClosingIssuesReferences struct {
							Nodes []struct {
								Number int `json:"number"`
								Labels struct {
									Nodes []struct {
										Name string `json:"name"`
									} `json:"nodes"`
								} `json:"labels"`
							} `json:"nodes"`
						} `json:"closingIssuesReferences"`
					} `json:"pullRequest"`
				} `json:"repository"`
			} `json:"data"`
		}

		if err := json.Unmarshal(responseData, &response); err != nil {
			continue
		}

		pr := response.Data.Repository.PullRequest
		
		// Skip if PR doesn't exist or not merged
		if pr == nil || pr.MergedAt == "" {
			continue
		}

		prData := PRData{
			Number:   pr.Number,
			Title:    pr.Title,
			Body:     pr.Body,
			MergedAt: pr.MergedAt,
			Author:   pr.Author.Login,
		}

		// Extract labels
		for _, label := range pr.Labels.Nodes {
			prData.Labels = append(prData.Labels, label.Name)
		}

		// Extract linked issues
		for _, issue := range pr.ClosingIssuesReferences.Nodes {
			linkedIssue := LinkedIssue{
				Number: issue.Number,
			}
			for _, label := range issue.Labels.Nodes {
				linkedIssue.Labels = append(linkedIssue.Labels, label.Name)
			}
			prData.LinkedIssues = append(prData.LinkedIssues, linkedIssue)
		}

		allPRs = append(allPRs, prData)
	}

	return allPRs, nil
}