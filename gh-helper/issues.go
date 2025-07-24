package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "GitHub issue operations",
	Long:  `Manage GitHub issues with support for creation, sub-issues, and bulk operations.`,
}

var createIssueCmd = NewOperationalCommand(
	"create [flags]",
	"Create a new issue",
	`Create a new GitHub issue with optional labels, assignees, milestone, and parent relationship.

Examples:
  # Create a simple issue
  gh-helper issues create --title "Fix memory leak" --body "Description here"
  
  # Create issue with labels and assignees
  gh-helper issues create \
    --title "Implement caching layer" \
    --body "Add Redis caching to improve performance" \
    --label performance,enhancement \
    --assignee alice,bob
  
  # Create sub-issue under parent issue
  gh-helper issues create \
    --title "Write cache tests" \
    --body "Add unit tests for cache implementation" \
    --parent 123
  
  # Create from file with template
  gh-helper issues create --body-file issue-template.md --title "Release v2.0"`,
	createIssue,
)


var showIssueCmd = NewOperationalCommand(
	"show <issue> [flags]",
	"Show detailed issue information",
	`Show detailed information about an issue including optional sub-issue data.

Examples:
  # Show basic issue information
  gh-helper issues show 248
  
  # Include sub-issues list and completion statistics
  gh-helper issues show 248 --include-sub
  
  # Include detailed information for each sub-issue (requires additional queries)
  gh-helper issues show 248 --include-sub --detailed`,
	showIssue,
)

var editIssueCmd = NewOperationalCommand(
	"edit <issue> [flags]",
	"Edit issue properties and manage sub-issue relationships",
	`Edit various properties of an existing issue including parent relationships and sub-issue ordering.

Examples:
  # Add issue #456 as a sub-issue of #123
  gh-helper issues edit 456 --parent 123
  
  # Move issue #456 to a new parent #789
  gh-helper issues edit 456 --parent 789 --overwrite
  
  # Remove parent relationship
  gh-helper issues edit 456 --unlink-parent
  
  # Reorder sub-issue #456 after #789
  gh-helper issues edit 456 --after 789
  
  # Move sub-issue to beginning or end
  gh-helper issues edit 456 --position first
  gh-helper issues edit 456 --position last
  
  # Batch add multiple sub-issues to parent #123
  gh-helper issues edit 123 --add-subs 456,789,101
  
  # Batch remove multiple sub-issues from parent #123
  gh-helper issues edit 123 --remove-subs 456,789`,
	editIssue,
)

func init() {
	// Configure flags for create command
	createIssueCmd.Flags().StringP("title", "t", "", "Issue title (required)")
	createIssueCmd.Flags().StringP("body", "b", "", "Issue body content")
	createIssueCmd.Flags().StringP("body-file", "F", "", "Read body from file")
	createIssueCmd.Flags().StringSliceP("label", "l", []string{}, "Add labels (comma-separated)")
	createIssueCmd.Flags().StringSliceP("assignee", "a", []string{}, "Assign users (comma-separated)")
	createIssueCmd.Flags().StringP("milestone", "m", "", "Assign to milestone")
	createIssueCmd.Flags().StringP("project", "p", "", "Add to project")
	createIssueCmd.Flags().Int("parent", 0, "Parent issue number for sub-issue creation")

	// Mark title as required
	if err := createIssueCmd.MarkFlagRequired("title"); err != nil {
		panic(fmt.Sprintf("failed to mark title flag as required: %v", err))
	}


	// Configure flags for show command
	showIssueCmd.Flags().Bool("include-sub", false, "Include sub-issues list and statistics")
	showIssueCmd.Flags().Bool("detailed", false, "Include detailed information for each sub-issue (requires --include-sub)")

	// Configure flags for edit command
	editIssueCmd.Flags().Int("parent", 0, "Set parent issue number")
	editIssueCmd.Flags().Bool("overwrite", false, "Replace existing parent relationship")
	editIssueCmd.Flags().Bool("unlink-parent", false, "Remove parent relationship")
	editIssueCmd.Flags().Int("after", 0, "Place sub-issue after another sub-issue")
	editIssueCmd.Flags().Int("before", 0, "Place sub-issue before another sub-issue")
	editIssueCmd.Flags().String("position", "", "Move sub-issue to 'first' or 'last' position")
	editIssueCmd.Flags().IntSlice("add-subs", []int{}, "Add multiple sub-issues (comma-separated)")
	editIssueCmd.Flags().IntSlice("remove-subs", []int{}, "Remove multiple sub-issues (comma-separated)")

	// Add subcommands
	issuesCmd.AddCommand(createIssueCmd)
	issuesCmd.AddCommand(showIssueCmd)
	issuesCmd.AddCommand(editIssueCmd)
}

// IssueCreationResult represents the result of issue creation
type IssueCreationResult struct {
	Number    int                `json:"number"`
	Title     string             `json:"title"`
	URL       string             `json:"url"`
	State     string             `json:"state"`
	Labels    []string           `json:"labels,omitempty"`
	Assignees []string           `json:"assignees,omitempty"`
	Parent    *ParentIssueInfo   `json:"parent,omitempty"`
	CreatedAt string             `json:"createdAt"`
}

// ParentIssueInfo represents parent issue information
type ParentIssueInfo struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

func createIssue(cmd *cobra.Command, args []string) error {
	// Get flags
	title, err := cmd.Flags().GetString("title")
	if err != nil {
		return fmt.Errorf("failed to get 'title' flag: %w", err)
	}
	body, err := cmd.Flags().GetString("body")
	if err != nil {
		return fmt.Errorf("failed to get 'body' flag: %w", err)
	}
	bodyFile, err := cmd.Flags().GetString("body-file")
	if err != nil {
		return fmt.Errorf("failed to get 'body-file' flag: %w", err)
	}
	labels, err := cmd.Flags().GetStringSlice("label")
	if err != nil {
		return fmt.Errorf("failed to get 'label' flag: %w", err)
	}
	assignees, err := cmd.Flags().GetStringSlice("assignee")
	if err != nil {
		return fmt.Errorf("failed to get 'assignee' flag: %w", err)
	}
	milestone, err := cmd.Flags().GetString("milestone")
	if err != nil {
		return fmt.Errorf("failed to get 'milestone' flag: %w", err)
	}
	project, err := cmd.Flags().GetString("project")
	if err != nil {
		return fmt.Errorf("failed to get 'project' flag: %w", err)
	}
	parentNumber, err := cmd.Flags().GetInt("parent")
	if err != nil {
		return fmt.Errorf("failed to get 'parent' flag: %w", err)
	}

	// Handle body from file
	if bodyFile != "" {
		if body != "" {
			return fmt.Errorf("cannot specify both --body and --body-file")
		}
		content, err := os.ReadFile(bodyFile)
		if err != nil {
			return fmt.Errorf("failed to read body file: %w", err)
		}
		body = string(content)
	}

	// Create GitHub client
	client := NewGitHubClient(owner, repo)

	// Get repository ID
	repoID, err := client.GetRepositoryID()
	if err != nil {
		return fmt.Errorf("failed to get repository ID: %w", err)
	}

	// Get label IDs if labels are specified
	var labelIDs []string
	if len(labels) > 0 {
		labelMap, err := client.GetLabelIDs(labels)
		if err != nil {
			return fmt.Errorf("failed to get label IDs: %w", err)
		}
		// Extract the IDs from the map
		for _, labelName := range labels {
			if id, ok := labelMap[labelName]; ok {
				labelIDs = append(labelIDs, id)
			} else {
				return fmt.Errorf("label not found: %s", labelName)
			}
		}
	}

	// Get assignee IDs if assignees are specified
	var assigneeIDs []string
	if len(assignees) > 0 {
		assigneeIDs, err = client.GetUserIDs(assignees)
		if err != nil {
			return fmt.Errorf("failed to get assignee IDs: %w", err)
		}
	}

	// Get milestone ID if specified
	var milestoneID string
	if milestone != "" {
		milestoneID, err = client.GetMilestoneID(milestone)
		if err != nil {
			return fmt.Errorf("failed to get milestone ID: %w", err)
		}
	}

	// Get project ID if specified
	var projectID string
	if project != "" {
		projectID, err = client.GetProjectID(project)
		if err != nil {
			return fmt.Errorf("failed to get project ID: %w", err)
		}
	}

	// Create the issue
	mutation := `
	mutation CreateIssue($repositoryId: ID!, $title: String!, $body: String, $labelIds: [ID!], $assigneeIds: [ID!], $milestoneId: ID, $projectIds: [ID!]) {
		createIssue(input: {
			repositoryId: $repositoryId
			title: $title
			body: $body
			labelIds: $labelIds
			assigneeIds: $assigneeIds
			milestoneId: $milestoneId
			projectIds: $projectIds
		}) {
			issue {
				id
				number
				url
				title
				state
				labels(first: 10) {
					nodes {
						name
					}
				}
				assignees(first: 10) {
					nodes {
						login
					}
				}
				createdAt
			}
		}
	}`

	variables := map[string]interface{}{
		"repositoryId": repoID,
		"title":        title,
	}

	if body != "" {
		variables["body"] = body
	}
	if len(labelIDs) > 0 {
		variables["labelIds"] = labelIDs
	}
	if len(assigneeIDs) > 0 {
		variables["assigneeIds"] = assigneeIDs
	}
	if milestoneID != "" {
		variables["milestoneId"] = milestoneID
	}
	if projectID != "" {
		variables["projectIds"] = []string{projectID}
	}

	responseData, err := client.RunGraphQLQueryWithVariables(mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to create issue: %w", err)
	}

	var response CreateIssueResponse

	if err := json.Unmarshal(responseData, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	issue := response.Data.CreateIssue.Issue
	if issue.Number == 0 {
		return fmt.Errorf("issue creation failed: empty response")
	}

	// If parent is specified, create sub-issue relationship
	var parentInfo *ParentIssueInfo
	if parentNumber > 0 {
		parentInfo, err = client.AddSubIssue(issue.ID, parentNumber)
		if err != nil {
			// Don't fail the entire operation, just warn
			WarningMsg("Failed to create sub-issue relationship: %v", err).Print()
		}
	}

	// Build result
	result := IssueCreationResult{
		Number:    issue.Number,
		Title:     issue.Title,
		URL:       issue.URL,
		State:     issue.State,
		CreatedAt: issue.CreatedAt,
		Parent:    parentInfo,
	}

	// Extract labels
	for _, label := range issue.Labels.Nodes {
		result.Labels = append(result.Labels, label.Name)
	}

	// Extract assignees
	for _, assignee := range issue.Assignees.Nodes {
		result.Assignees = append(result.Assignees, assignee.Login)
	}

	// Output result
	output := map[string]interface{}{
		"issue": result,
	}

	return EncodeOutputWithCmd(cmd, output)
}

// Helper methods that need to be added to GitHubClient

// GetRepositoryID returns the repository's node ID
func (c *GitHubClient) GetRepositoryID() (string, error) {
	query := `
	query($owner: String!, $repo: String!) {
		repository(owner: $owner, name: $repo) {
			id
		}
	}`

	variables := map[string]interface{}{
		"owner": c.Owner,
		"repo":  c.Repo,
	}

	responseData, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return "", err
	}

	var response RepositoryIDResponse

	if err := json.Unmarshal(responseData, &response); err != nil {
		return "", err
	}

	return response.Data.Repository.ID, nil
}

// Note: GetLabelIDs is already implemented in github.go and returns map[string]string

// GetUserIDs returns the node IDs for the given usernames
func (c *GitHubClient) GetUserIDs(usernames []string) ([]string, error) {
	// For simplicity, we'll query each user individually
	// In a production system, you might want to batch these
	var ids []string
	
	for _, username := range usernames {
		query := `
		query($login: String!) {
			user(login: $login) {
				id
			}
		}`

		variables := map[string]interface{}{
			"login": username,
		}

		responseData, err := c.RunGraphQLQueryWithVariables(query, variables)
		if err != nil {
			return nil, fmt.Errorf("failed to get user %s: %w", username, err)
		}

		var response UserQueryResponse

		if err := json.Unmarshal(responseData, &response); err != nil {
			return nil, err
		}

		if response.Data.User.ID == "" {
			return nil, fmt.Errorf("user not found: %s", username)
		}

		ids = append(ids, response.Data.User.ID)
	}

	return ids, nil
}

// GetMilestoneID returns the node ID for the given milestone title
func (c *GitHubClient) GetMilestoneID(title string) (string, error) {
	query := `
	query($owner: String!, $repo: String!) {
		repository(owner: $owner, name: $repo) {
			milestones(first: 100, states: OPEN) {
				nodes {
					id
					title
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner": c.Owner,
		"repo":  c.Repo,
	}

	responseData, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return "", err
	}

	var response MilestoneQueryResponse

	if err := json.Unmarshal(responseData, &response); err != nil {
		return "", err
	}

	// Find milestone by title
	for _, milestone := range response.Data.Repository.Milestones.Nodes {
		if milestone.Title == title {
			return milestone.ID, nil
		}
	}

	return "", fmt.Errorf("milestone not found: %s", title)
}

// GetProjectID returns the node ID for the given project name
func (c *GitHubClient) GetProjectID(name string) (string, error) {
	// Note: This is a simplified implementation
	// GitHub Projects V2 has a more complex structure
	query := `
	query($owner: String!, $repo: String!) {
		repository(owner: $owner, name: $repo) {
			projectsV2(first: 20) {
				nodes {
					id
					title
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner": c.Owner,
		"repo":  c.Repo,
	}

	responseData, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return "", err
	}

	var response ProjectQueryResponse

	if err := json.Unmarshal(responseData, &response); err != nil {
		return "", err
	}

	// Find project by name
	for _, project := range response.Data.Repository.ProjectsV2.Nodes {
		if project.Title == name {
			return project.ID, nil
		}
	}

	return "", fmt.Errorf("project not found: %s", name)
}

// GetIssueWithSubIssues fetches issue information with optional sub-issues
func (c *GitHubClient) GetIssueWithSubIssues(number int, includeSub bool, detailed bool) (*IssueShowResult, error) {
	// Build query with fragments
	query := AllIssueFragments + `
	query($owner: String!, $repo: String!, $number: Int!, $includeSub: Boolean!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				...IssueFields
				subIssues(first: 100) @include(if: $includeSub) {
					totalCount
					nodes {
						...SubIssueFields
					}
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner":      c.Owner,
		"repo":       c.Repo,
		"number":     number,
		"includeSub": includeSub,
	}

	responseData, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issue: %w", err)
	}

	var response IssueWithSubIssuesResponse

	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Data.Repository.Issue == nil {
		return nil, fmt.Errorf("issue not found: #%d", number)
	}

	issue := response.Data.Repository.Issue

	// Build result
	result := &IssueShowResult{
		Issue: DetailedIssueInfo{
			Number:    issue.Number,
			Title:     issue.Title,
			State:     issue.State,
			Body:      issue.Body,
			URL:       issue.URL,
			CreatedAt: issue.CreatedAt,
			UpdatedAt: issue.UpdatedAt,
		},
	}

	// Extract labels
	for _, label := range issue.Labels.Nodes {
		result.Issue.Labels = append(result.Issue.Labels, label.Name)
	}

	// Extract assignees
	for _, assignee := range issue.Assignees.Nodes {
		result.Issue.Assignees = append(result.Issue.Assignees, assignee.Login)
	}

	// Process sub-issues if included
	if includeSub && issue.SubIssues != nil {
		subIssuesInfo := &SubIssuesInfo{
			TotalCount: issue.SubIssues.TotalCount,
			Items:      []SubIssueItem{},
		}

		completedCount := 0
		for _, subIssue := range issue.SubIssues.Nodes {
			item := SubIssueItem{
				Number: subIssue.Number,
				Title:  subIssue.Title,
				State:  subIssue.State,
				Closed: subIssue.Closed,
			}
			if subIssue.Closed {
				completedCount++
			}
			subIssuesInfo.Items = append(subIssuesInfo.Items, item)
		}

		subIssuesInfo.CompletedCount = completedCount
		if subIssuesInfo.TotalCount > 0 {
			subIssuesInfo.CompletionPercentage = float64(completedCount) / float64(subIssuesInfo.TotalCount) * 100
		}

		// Fetch detailed information if requested
		if detailed && len(subIssuesInfo.Items) > 0 {
			// Batch fetch sub-issue details (could be optimized with GraphQL aliases)
			for i := range subIssuesInfo.Items {
				detailResult, err := c.GetIssueWithSubIssues(subIssuesInfo.Items[i].Number, false, false)
				if err != nil {
					// Log error but continue
					WarningMsg("Failed to fetch details for sub-issue #%d: %v", subIssuesInfo.Items[i].Number, err).Print()
					continue
				}
				subIssuesInfo.Items[i].Details = &detailResult.Issue
			}
		}

		result.SubIssues = subIssuesInfo
	}

	return result, nil
}

// RemoveSubIssue removes a sub-issue relationship
func (c *GitHubClient) RemoveSubIssue(childNumber int) (*BasicIssueInfo, error) {
	// First, get the child issue ID
	childQuery := `
	query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				id
				title
				url
				state
			}
		}
	}`

	childVariables := map[string]interface{}{
		"owner":  c.Owner,
		"repo":   c.Repo,
		"number": childNumber,
	}

	childData, err := c.RunGraphQLQueryWithVariables(childQuery, childVariables)
	if err != nil {
		return nil, fmt.Errorf("failed to get child issue: %w", err)
	}

	var childResponse GetRepositoryIssueResponse

	if err := json.Unmarshal(childData, &childResponse); err != nil {
		return nil, err
	}

	if childResponse.Data.Repository.Issue == nil {
		return nil, fmt.Errorf("child issue not found: #%d", childNumber)
	}

	childID := childResponse.Data.Repository.Issue.ID

	// Get the parent issue ID first - we need both IDs for removeSubIssue
	// We need to find which issue is the parent
	parentQuery := `
	query($childId: ID!) {
		node(id: $childId) {
			... on Issue {
				parent {
					id
				}
			}
		}
	}`

	parentVariables := map[string]interface{}{
		"childId": childID,
	}

	parentData, err := c.RunGraphQLQueryWithVariables(parentQuery, parentVariables)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent issue: %w", err)
	}

	var parentResponse NodeQueryParentResponse
	if err := json.Unmarshal(parentData, &parentResponse); err != nil {
		return nil, err
	}

	if parentResponse.Data.Node.Parent == nil {
		return nil, fmt.Errorf("issue #%d has no parent", childNumber)
	}

	parentID := parentResponse.Data.Node.Parent.ID

	// Remove the sub-issue relationship
	mutation := `
	mutation($issueId: ID!, $subIssueId: ID!) {
		removeSubIssue(input: {
			issueId: $issueId
			subIssueId: $subIssueId
		}) {
			issue {
				id
				number
				title
				url
				state
			}
		}
	}`

	variables := map[string]interface{}{
		"issueId":    parentID,
		"subIssueId": childID,
	}

	responseData, err := c.RunGraphQLQueryWithVariables(mutation, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to remove sub-issue relationship: %w", err)
	}

	var response RemoveSubIssueMutationResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, err
	}

	return &BasicIssueInfo{
		Number: childNumber,
		Title:  childResponse.Data.Repository.Issue.Title,
		URL:    childResponse.Data.Repository.Issue.URL,
		State:  childResponse.Data.Repository.Issue.State,
	}, nil
}

// SetIssueParent sets or updates the parent of an issue
func (c *GitHubClient) SetIssueParent(childNumber int, parentNumber int, overwrite bool) (*EditIssueResult, error) {
	// Get both issue IDs
	issueQuery := `
	query($owner: String!, $repo: String!, $childNumber: Int!, $parentNumber: Int!) {
		repository(owner: $owner, name: $repo) {
			child: issue(number: $childNumber) {
				id
				number
				title
				url
				state
			}
			parent: issue(number: $parentNumber) {
				id
				number
				title
				url
				state
			}
		}
	}`

	variables := map[string]interface{}{
		"owner":        c.Owner,
		"repo":         c.Repo,
		"childNumber":  childNumber,
		"parentNumber": parentNumber,
	}

	responseData, err := c.RunGraphQLQueryWithVariables(issueQuery, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issues: %w", err)
	}

	var response GetRepositoryIssuesResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if response.Data.Repository.Child == nil {
		return nil, fmt.Errorf("child issue not found: #%d", childNumber)
	}
	if response.Data.Repository.Parent == nil {
		return nil, fmt.Errorf("parent issue not found: #%d", parentNumber)
	}

	child := response.Data.Repository.Child
	parent := response.Data.Repository.Parent

	// Create the sub-issue relationship
	mutation := `
	mutation($parentId: ID!, $subIssueId: ID!, $replaceParent: Boolean!) {
		addSubIssue(input: {
			issueId: $parentId
			subIssueId: $subIssueId
			replaceParent: $replaceParent
		}) {
			issue {
				id
				number
				title
				url
				state
			}
			subIssue {
				id
				number
				title
				url
				state
			}
		}
	}`

	linkVariables := map[string]interface{}{
		"parentId":      parent.ID,
		"subIssueId":    child.ID,
		"replaceParent": overwrite,
	}

	linkResponseData, err := c.RunGraphQLQueryWithVariables(mutation, linkVariables)
	if err != nil {
		return nil, fmt.Errorf("failed to set parent relationship: %w", err)
	}

	var linkResponse AddSubIssueMutationResponse
	if err := json.Unmarshal(linkResponseData, &linkResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Build result
	result := &EditIssueResult{
		Issue: child.toBasicIssueInfo(),
		Changes: []ChangeInfo{
			{
				Field:    "parent",
				NewValue: fmt.Sprintf("#%d", parentNumber),
				Action:   "set",
			},
		},
		ParentChange: &ParentChangeInfo{
			NewParent: &BasicIssueInfo{
				Number: parent.Number,
				Title:  parent.Title,
				URL:    parent.URL,
				State:  parent.State,
			},
			Action: "add",
		},
	}

	if overwrite {
		result.ParentChange.Action = "replace"
	}

	return result, nil
}

// UnlinkIssueParent removes the parent relationship from an issue
func (c *GitHubClient) UnlinkIssueParent(childNumber int) (*EditIssueResult, error) {
	child, err := c.RemoveSubIssue(childNumber)
	if err != nil {
		return nil, err
	}

	return &EditIssueResult{
		Issue: *child,
		Changes: []ChangeInfo{
			{
				Field:    "parent",
				OldValue: "linked",
				NewValue: "none",
				Action:   "unlink",
			},
		},
		ParentChange: &ParentChangeInfo{
			Action: "remove",
		},
	}, nil
}

// AddSubIssue creates a parent-child relationship between issues
func (c *GitHubClient) AddSubIssue(childID string, parentNumber int) (*ParentIssueInfo, error) {
	// First, get the parent issue ID
	parentQuery := `
	query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				id
				title
			}
		}
	}`

	parentVariables := map[string]interface{}{
		"owner":  c.Owner,
		"repo":   c.Repo,
		"number": parentNumber,
	}

	parentData, err := c.RunGraphQLQueryWithVariables(parentQuery, parentVariables)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent issue: %w", err)
	}

	var parentResponse GetRepositoryIssueResponse

	if err := json.Unmarshal(parentData, &parentResponse); err != nil {
		return nil, err
	}

	parentID := parentResponse.Data.Repository.Issue.ID
	if parentID == "" {
		return nil, fmt.Errorf("parent issue not found: #%d", parentNumber)
	}

	// Create the sub-issue relationship
	mutation := `
	mutation($parentId: ID!, $subIssueId: ID!) {
		addSubIssue(input: {
			issueId: $parentId
			subIssueId: $subIssueId
		}) {
			issue {
				id
			}
		}
	}`

	variables := map[string]interface{}{
		"parentId":   parentID,
		"subIssueId": childID,
	}

	_, err = c.RunGraphQLQueryWithVariables(mutation, variables)
	if err != nil {
		return nil, err
	}

	return &ParentIssueInfo{
		Number: parentNumber,
		Title:  parentResponse.Data.Repository.Issue.Title,
	}, nil
}

// BasicIssueInfo represents basic issue information matching GitHub GraphQL API Issue type
type BasicIssueInfo struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

// IssueShowResult represents the result of showing issue details
type IssueShowResult struct {
	Issue     DetailedIssueInfo `json:"issue"`
	SubIssues *SubIssuesInfo    `json:"subIssues,omitempty"`
}

// DetailedIssueInfo represents detailed issue information
type DetailedIssueInfo struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Body      string   `json:"body"`
	Labels    []string `json:"labels,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
	URL       string   `json:"url"`
}

// SubIssuesInfo represents sub-issues information and statistics
type SubIssuesInfo struct {
	TotalCount           int            `json:"totalCount"`
	CompletedCount       int            `json:"completedCount"`
	CompletionPercentage float64        `json:"completionPercentage"`
	Items                []SubIssueItem `json:"items"`
}

// SubIssueItem represents a single sub-issue
type SubIssueItem struct {
	Number  int                `json:"number"`
	Title   string             `json:"title"`
	State   string             `json:"state"`
	Closed  bool               `json:"closed"`
	Details *DetailedIssueInfo `json:"details,omitempty"`
}

// EditIssueResult represents the result of issue editing
type EditIssueResult struct {
	Issue        BasicIssueInfo    `json:"issue"`
	Changes      []ChangeInfo      `json:"changes"`
	ParentChange *ParentChangeInfo `json:"parentChange,omitempty"`
}

// ChangeInfo represents a single change made to an issue
type ChangeInfo struct {
	Field    string `json:"field"`
	OldValue string `json:"oldValue,omitempty"`
	NewValue string `json:"newValue"`
	Action   string `json:"action"`
}

// ParentChangeInfo represents parent relationship changes
type ParentChangeInfo struct {
	OldParent *BasicIssueInfo `json:"oldParent,omitempty"`
	NewParent *BasicIssueInfo `json:"newParent,omitempty"`
	Action    string          `json:"action"`
}

// toBasicIssueInfo converts GitHub API IssueFields to our custom BasicIssueInfo type
func (n *IssueFields) toBasicIssueInfo() BasicIssueInfo {
	return BasicIssueInfo{
		Number: n.Number,
		Title:  n.Title,
		URL:    n.URL,
		State:  n.State,
	}
}

func showIssue(cmd *cobra.Command, args []string) error {
	// Validate required arguments
	if len(args) < 1 {
		return fmt.Errorf("issue number is required")
	}
	
	// Parse issue number from args
	issueNumber, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid issue number: %s", args[0])
	}
	
	// Get flags
	includeSub, err := cmd.Flags().GetBool("include-sub")
	if err != nil {
		return fmt.Errorf("failed to get 'include-sub' flag: %w", err)
	}
	detailed, err := cmd.Flags().GetBool("detailed")
	if err != nil {
		return fmt.Errorf("failed to get 'detailed' flag: %w", err)
	}
	
	// Validate flag combination
	if detailed && !includeSub {
		return fmt.Errorf("--detailed requires --include-sub")
	}
	
	// Create GitHub client
	client := NewGitHubClient(owner, repo)
	
	// Fetch issue information
	result, err := client.GetIssueWithSubIssues(issueNumber, includeSub, detailed)
	if err != nil {
		return fmt.Errorf("failed to fetch issue: %w", err)
	}
	
	// Output result
	output := map[string]interface{}{
		"issueShow": result,
	}
	
	return EncodeOutputWithCmd(cmd, output)
}

// ReorderSubIssue reorders a sub-issue within its parent's sub-issue list
func (c *GitHubClient) ReorderSubIssue(subIssueNumber int, afterNumber int, beforeNumber int, position string) (*EditIssueResult, error) {
	// First, get the sub-issue ID and its parent
	query := `
	query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				id
				title
				url
				state
				parent {
					id
					number
					title
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner":  c.Owner,
		"repo":   c.Repo,
		"number": subIssueNumber,
	}

	responseData, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issue: %w", err)
	}

	var response IssueQueryResponse

	if err := json.Unmarshal(responseData, &response); err != nil {
		return nil, err
	}

	if response.Data.Repository.Issue == nil {
		return nil, fmt.Errorf("issue not found: #%d", subIssueNumber)
	}

	issue := response.Data.Repository.Issue
	if issue.Parent == nil {
		return nil, fmt.Errorf("issue #%d is not a sub-issue", subIssueNumber)
	}

	// Determine the position details
	var afterID, beforeID *string
	
	if afterNumber != 0 {
		afterQuery := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) { id }
			}
		}`
		afterVars := map[string]interface{}{"owner": c.Owner, "repo": c.Repo, "number": afterNumber}
		afterData, err := c.RunGraphQLQueryWithVariables(afterQuery, afterVars)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch 'after' issue: %w", err)
		}
		var afterResp GetRepositoryIssueResponse
		if err := json.Unmarshal(afterData, &afterResp); err != nil {
			return nil, err
		}
		if afterResp.Data.Repository.Issue == nil {
			return nil, fmt.Errorf("'after' issue not found: #%d", afterNumber)
		}
		afterID = &afterResp.Data.Repository.Issue.ID
	}
	
	if beforeNumber != 0 {
		beforeQuery := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) { id }
			}
		}`
		beforeVars := map[string]interface{}{"owner": c.Owner, "repo": c.Repo, "number": beforeNumber}
		beforeData, err := c.RunGraphQLQueryWithVariables(beforeQuery, beforeVars)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch 'before' issue: %w", err)
		}
		var beforeResp GetRepositoryIssueResponse
		if err := json.Unmarshal(beforeData, &beforeResp); err != nil {
			return nil, err
		}
		if beforeResp.Data.Repository.Issue == nil {
			return nil, fmt.Errorf("'before' issue not found: #%d", beforeNumber)
		}
		beforeID = &beforeResp.Data.Repository.Issue.ID
	}

	// Execute the reprioritizeSubIssue mutation
	mutation := `
	mutation($parentId: ID!, $subIssueId: ID!, $afterId: ID, $beforeId: ID) {
		reprioritizeSubIssue(input: {
			issueId: $parentId
			subIssueId: $subIssueId
			afterId: $afterId
			beforeId: $beforeId
		}) {
			issue {
				id
				number
				title
			}
		}
	}`

	mutationVars := map[string]interface{}{
		"parentId":   issue.Parent.ID,
		"subIssueId": issue.ID,
	}
	
	// Handle position-based ordering
	switch position {
	case "first":
		// For first position, we need to get the first sub-issue and use beforeId
		firstQuery := `
		query($parentId: ID!) {
			node(id: $parentId) {
				... on Issue {
					subIssues(first: 1) {
						nodes { id }
					}
				}
			}
		}`
		firstVars := map[string]interface{}{"parentId": issue.Parent.ID}
		firstData, err := c.RunGraphQLQueryWithVariables(firstQuery, firstVars)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch first sub-issue: %w", err)
		}
		var firstResp NodeQuerySubIssuesResponse
		if err := json.Unmarshal(firstData, &firstResp); err != nil {
			return nil, err
		}
		if len(firstResp.Data.Node.SubIssues.Nodes) > 0 {
			firstID := firstResp.Data.Node.SubIssues.Nodes[0].ID
			if firstID != issue.ID {
				mutationVars["beforeId"] = firstID
			}
		}
	case "last":
		// For last position, we need to get the last sub-issue
		lastQuery := `
		query($parentId: ID!) {
			node(id: $parentId) {
				... on Issue {
					subIssues(last: 1) {
						nodes { id }
					}
				}
			}
		}`
		lastVars := map[string]interface{}{"parentId": issue.Parent.ID}
		lastData, err := c.RunGraphQLQueryWithVariables(lastQuery, lastVars)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch last sub-issue: %w", err)
		}
		var lastResp NodeQuerySubIssuesResponse
		if err := json.Unmarshal(lastData, &lastResp); err != nil {
			return nil, err
		}
		if len(lastResp.Data.Node.SubIssues.Nodes) > 0 {
			lastID := lastResp.Data.Node.SubIssues.Nodes[0].ID
			if lastID != issue.ID {
				mutationVars["afterId"] = lastID
			}
		}
	default:
		// Use provided afterId or beforeId
		if afterID != nil {
			mutationVars["afterId"] = *afterID
		}
		if beforeID != nil {
			mutationVars["beforeId"] = *beforeID
		}
	}

	mutationData, err := c.RunGraphQLQueryWithVariables(mutation, mutationVars)
	if err != nil {
		return nil, fmt.Errorf("failed to reorder sub-issue: %w", err)
	}

	var mutationResp ReprioritizeSubIssueResponse

	if err := json.Unmarshal(mutationData, &mutationResp); err != nil {
		return nil, err
	}

	// Build result
	changeDescription := ""
	if position != "" {
		changeDescription = fmt.Sprintf("moved to %s position", position)
	} else if afterNumber != 0 {
		changeDescription = fmt.Sprintf("moved after #%d", afterNumber)
	} else if beforeNumber != 0 {
		changeDescription = fmt.Sprintf("moved before #%d", beforeNumber)
	}

	return &EditIssueResult{
		Issue: BasicIssueInfo{
			Number: subIssueNumber,
			Title:  issue.Title,
			URL:    issue.URL,
			State:  issue.State,
		},
		Changes: []ChangeInfo{
			{
				Field:    "position",
				NewValue: changeDescription,
				Action:   "reorder",
			},
		},
	}, nil
}

// BatchAddSubIssues adds multiple sub-issues to a parent issue
func (c *GitHubClient) BatchAddSubIssues(parentNumber int, subIssueNumbers []int) (*EditIssueResult, error) {
	// Get parent issue ID
	parentQuery := `
	query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				id
				title
				url
				state
			}
		}
	}`

	parentVars := map[string]interface{}{
		"owner":  c.Owner,
		"repo":   c.Repo,
		"number": parentNumber,
	}

	parentData, err := c.RunGraphQLQueryWithVariables(parentQuery, parentVars)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch parent issue: %w", err)
	}

	var parentResp GetRepositoryIssueResponse

	if err := json.Unmarshal(parentData, &parentResp); err != nil {
		return nil, err
	}

	if parentResp.Data.Repository.Issue == nil {
		return nil, fmt.Errorf("parent issue not found: #%d", parentNumber)
	}

	parentIssue := parentResp.Data.Repository.Issue

	// Get sub-issue IDs
	subIssueIDs := make([]string, 0, len(subIssueNumbers))
	for _, num := range subIssueNumbers {
		subQuery := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) { id }
			}
		}`
		subVars := map[string]interface{}{"owner": c.Owner, "repo": c.Repo, "number": num}
		subData, err := c.RunGraphQLQueryWithVariables(subQuery, subVars)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch sub-issue #%d: %w", num, err)
		}
		
		var subResp GetRepositoryIssueResponse
		
		if err := json.Unmarshal(subData, &subResp); err != nil {
			return nil, err
		}
		
		if subResp.Data.Repository.Issue == nil {
			return nil, fmt.Errorf("sub-issue not found: #%d", num)
		}
		
		subIssueIDs = append(subIssueIDs, subResp.Data.Repository.Issue.ID)
	}

	// Use GraphQL aliases to batch the operations
	var mutationBuilder strings.Builder
	mutationBuilder.WriteString("mutation {")
	
	for i, subID := range subIssueIDs {
		fmt.Fprintf(&mutationBuilder, `
		add%d: addSubIssue(input: {
			issueId: "%s"
			subIssueId: "%s"
		}) {
			issue { id }
		}`, i, parentIssue.ID, subID)
	}
	
	mutationBuilder.WriteString("\n}")

	mutationData, err := c.RunGraphQLQuery(mutationBuilder.String())
	if err != nil {
		return nil, fmt.Errorf("failed to add sub-issues: %w", err)
	}

	// We don't need to parse the full response, just check for errors
	var errorCheck struct {
		Errors []interface{} `json:"errors"`
	}
	if err := json.Unmarshal(mutationData, &errorCheck); err == nil && len(errorCheck.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors occurred during batch operation")
	}

	return &EditIssueResult{
		Issue: BasicIssueInfo{
			Number: parentNumber,
			Title:  parentIssue.Title,
			URL:    parentIssue.URL,
			State:  parentIssue.State,
		},
		Changes: []ChangeInfo{
			{
				Field:    "sub-issues",
				NewValue: fmt.Sprintf("added %d sub-issues: %v", len(subIssueNumbers), subIssueNumbers),
				Action:   "add",
			},
		},
	}, nil
}

// BatchRemoveSubIssues removes multiple sub-issues from a parent issue
func (c *GitHubClient) BatchRemoveSubIssues(parentNumber int, subIssueNumbers []int) (*EditIssueResult, error) {
	// Get parent issue ID
	parentQuery := `
	query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) {
				id
				title
				url
				state
			}
		}
	}`

	parentVars := map[string]interface{}{
		"owner":  c.Owner,
		"repo":   c.Repo,
		"number": parentNumber,
	}

	parentData, err := c.RunGraphQLQueryWithVariables(parentQuery, parentVars)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch parent issue: %w", err)
	}

	var parentResp GetRepositoryIssueResponse

	if err := json.Unmarshal(parentData, &parentResp); err != nil {
		return nil, err
	}

	if parentResp.Data.Repository.Issue == nil {
		return nil, fmt.Errorf("parent issue not found: #%d", parentNumber)
	}

	parentIssue := parentResp.Data.Repository.Issue

	// Get sub-issue IDs
	subIssueIDs := make([]string, 0, len(subIssueNumbers))
	for _, num := range subIssueNumbers {
		subQuery := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) { id }
			}
		}`
		subVars := map[string]interface{}{"owner": c.Owner, "repo": c.Repo, "number": num}
		subData, err := c.RunGraphQLQueryWithVariables(subQuery, subVars)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch sub-issue #%d: %w", num, err)
		}
		
		var subResp GetRepositoryIssueResponse
		
		if err := json.Unmarshal(subData, &subResp); err != nil {
			return nil, err
		}
		
		if subResp.Data.Repository.Issue == nil {
			return nil, fmt.Errorf("sub-issue not found: #%d", num)
		}
		
		subIssueIDs = append(subIssueIDs, subResp.Data.Repository.Issue.ID)
	}

	// Use GraphQL aliases to batch the operations
	var mutationBuilder strings.Builder
	mutationBuilder.WriteString("mutation {")
	
	for i, subID := range subIssueIDs {
		fmt.Fprintf(&mutationBuilder, `
		remove%d: removeSubIssue(input: {
			issueId: "%s"
			subIssueId: "%s"
		}) {
			issue { id }
		}`, i, parentIssue.ID, subID)
	}
	
	mutationBuilder.WriteString("\n}")

	mutationData, err := c.RunGraphQLQuery(mutationBuilder.String())
	if err != nil {
		return nil, fmt.Errorf("failed to remove sub-issues: %w", err)
	}

	// We don't need to parse the full response, just check for errors
	var errorCheck struct {
		Errors []interface{} `json:"errors"`
	}
	if err := json.Unmarshal(mutationData, &errorCheck); err == nil && len(errorCheck.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors occurred during batch operation")
	}

	return &EditIssueResult{
		Issue: BasicIssueInfo{
			Number: parentNumber,
			Title:  parentIssue.Title,
			URL:    parentIssue.URL,
			State:  parentIssue.State,
		},
		Changes: []ChangeInfo{
			{
				Field:    "sub-issues",
				NewValue: fmt.Sprintf("removed %d sub-issues: %v", len(subIssueNumbers), subIssueNumbers),
				Action:   "remove",
			},
		},
	}, nil
}

func editIssue(cmd *cobra.Command, args []string) error {
	// Validate required arguments
	if len(args) < 1 {
		return fmt.Errorf("issue number is required")
	}
	
	// Parse issue number from args
	issueNumber, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid issue number: %s", args[0])
	}
	
	// Get all flags
	parentNumber, err := cmd.Flags().GetInt("parent")
	if err != nil {
		return fmt.Errorf("failed to get 'parent' flag: %w", err)
	}
	overwrite, err := cmd.Flags().GetBool("overwrite")
	if err != nil {
		return fmt.Errorf("failed to get 'overwrite' flag: %w", err)
	}
	unlinkParent, err := cmd.Flags().GetBool("unlink-parent")
	if err != nil {
		return fmt.Errorf("failed to get 'unlink-parent' flag: %w", err)
	}
	afterNumber, err := cmd.Flags().GetInt("after")
	if err != nil {
		return fmt.Errorf("failed to get 'after' flag: %w", err)
	}
	beforeNumber, err := cmd.Flags().GetInt("before")
	if err != nil {
		return fmt.Errorf("failed to get 'before' flag: %w", err)
	}
	position, err := cmd.Flags().GetString("position")
	if err != nil {
		return fmt.Errorf("failed to get 'position' flag: %w", err)
	}
	addSubs, err := cmd.Flags().GetIntSlice("add-subs")
	if err != nil {
		return fmt.Errorf("failed to get 'add-subs' flag: %w", err)
	}
	removeSubs, err := cmd.Flags().GetIntSlice("remove-subs")
	if err != nil {
		return fmt.Errorf("failed to get 'remove-subs' flag: %w", err)
	}
	
	// Count how many operations are requested
	operationCount := 0
	if parentNumber != 0 || unlinkParent {
		operationCount++
	}
	if afterNumber != 0 || beforeNumber != 0 || position != "" {
		operationCount++
	}
	if len(addSubs) > 0 {
		operationCount++
	}
	if len(removeSubs) > 0 {
		operationCount++
	}
	
	if operationCount == 0 {
		return fmt.Errorf("must specify at least one operation (--parent, --unlink-parent, --after, --before, --position, --add-subs, or --remove-subs)")
	}
	if operationCount > 1 {
		return fmt.Errorf("cannot combine multiple operations in a single command")
	}
	
	// Validate specific flag combinations
	if unlinkParent && parentNumber != 0 {
		return fmt.Errorf("cannot use --unlink-parent with --parent")
	}
	if unlinkParent && overwrite {
		return fmt.Errorf("cannot use --unlink-parent with --overwrite")
	}
	if overwrite && parentNumber == 0 {
		return fmt.Errorf("--overwrite requires --parent")
	}
	if afterNumber != 0 && beforeNumber != 0 {
		return fmt.Errorf("cannot use --after with --before")
	}
	if (afterNumber != 0 || beforeNumber != 0) && position != "" {
		return fmt.Errorf("cannot use --after/--before with --position")
	}
	if position != "" && position != "first" && position != "last" {
		return fmt.Errorf("--position must be 'first' or 'last'")
	}
	
	// Create GitHub client
	client := NewGitHubClient(owner, repo)
	
	// Execute the appropriate operation
	var result *EditIssueResult
	
	switch {
	case unlinkParent:
		result, err = client.UnlinkIssueParent(issueNumber)
	case parentNumber != 0:
		result, err = client.SetIssueParent(issueNumber, parentNumber, overwrite)
	case afterNumber != 0 || beforeNumber != 0 || position != "":
		result, err = client.ReorderSubIssue(issueNumber, afterNumber, beforeNumber, position)
	case len(addSubs) > 0:
		result, err = client.BatchAddSubIssues(issueNumber, addSubs)
	case len(removeSubs) > 0:
		result, err = client.BatchRemoveSubIssues(issueNumber, removeSubs)
	}
	
	if err != nil {
		return fmt.Errorf("failed to edit issue: %w", err)
	}
	
	// Output result
	output := map[string]interface{}{
		"issueEdit": result,
	}
	
	return EncodeOutputWithCmd(cmd, output)
}