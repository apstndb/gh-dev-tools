package main

import (
	"encoding/json"
	"fmt"
	"os"

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

	// Add subcommands
	issuesCmd.AddCommand(createIssueCmd)
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
	title, _ := cmd.Flags().GetString("title")
	body, _ := cmd.Flags().GetString("body")
	bodyFile, _ := cmd.Flags().GetString("body-file")
	labels, _ := cmd.Flags().GetStringSlice("label")
	assignees, _ := cmd.Flags().GetStringSlice("assignee")
	milestone, _ := cmd.Flags().GetString("milestone")
	project, _ := cmd.Flags().GetString("project")
	parentNumber, _ := cmd.Flags().GetInt("parent")

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

	var response struct {
		Data struct {
			CreateIssue struct {
				Issue struct {
					ID        string `json:"id"`
					Number    int    `json:"number"`
					URL       string `json:"url"`
					Title     string `json:"title"`
					State     string `json:"state"`
					Labels    struct {
						Nodes []struct {
							Name string `json:"name"`
						} `json:"nodes"`
					} `json:"labels"`
					Assignees struct {
						Nodes []struct {
							Login string `json:"login"`
						} `json:"nodes"`
					} `json:"assignees"`
					CreatedAt string `json:"createdAt"`
				} `json:"issue"`
			} `json:"createIssue"`
		} `json:"data"`
	}

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
	format := ResolveFormat(cmd)
	output := map[string]interface{}{
		"issue": result,
	}

	return EncodeOutput(os.Stdout, format, output)
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

	var response struct {
		Data struct {
			Repository struct {
				ID string `json:"id"`
			} `json:"repository"`
		} `json:"data"`
	}

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

		var response struct {
			Data struct {
				User struct {
					ID string `json:"id"`
				} `json:"user"`
			} `json:"data"`
		}

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

	var response struct {
		Data struct {
			Repository struct {
				Milestones struct {
					Nodes []struct {
						ID    string `json:"id"`
						Title string `json:"title"`
					} `json:"nodes"`
				} `json:"milestones"`
			} `json:"repository"`
		} `json:"data"`
	}

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

	var response struct {
		Data struct {
			Repository struct {
				ProjectsV2 struct {
					Nodes []struct {
						ID    string `json:"id"`
						Title string `json:"title"`
					} `json:"nodes"`
				} `json:"projectsV2"`
			} `json:"repository"`
		} `json:"data"`
	}

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

	var parentResponse struct {
		Data struct {
			Repository struct {
				Issue struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"issue"`
			} `json:"repository"`
		} `json:"data"`
	}

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