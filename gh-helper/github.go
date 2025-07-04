package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json" // Still needed for json.RawMessage type
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	
	"golang.org/x/net/http2"
)

// GitHubClient provides common GitHub operations with token caching
type GitHubClient struct {
	Owner        string
	Repo         string
	httpClient   *http.Client
	repositoryID string // Cached repository ID (immutable for repo lifetime)
}

// Global shared client for HTTP/2 connection reuse and keep-alive optimization
var (
	sharedHTTPClient *http.Client
	clientOnce       sync.Once
)


// Repository IDs are immutable for the lifetime of a repository
// - Renaming a repo preserves the ID
// - Only deletion+recreation generates a new ID (extremely rare in practice)
// - No TTL needed for single command execution lifecycle

// getOptimizedHTTPClient returns a shared HTTP client optimized for GitHub API
// Features: HTTP/2, connection pooling, keep-alive, proper timeouts
func getOptimizedHTTPClient() *http.Client {
	clientOnce.Do(func() {
		// Create transport with HTTP/2 support and connection pooling
		transport := &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				NextProtos: []string{"h2", "http/1.1"}, // Prefer HTTP/2
			},
		}

		// Configure HTTP/2 (ignore errors - falls back to HTTP/1.1 gracefully)
		_ = http2.ConfigureTransport(transport)

		sharedHTTPClient = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
	})
	return sharedHTTPClient
}

// PRInfo represents basic PR information  
type PRInfo struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

// NewGitHubClient creates a new GitHub client with default or custom owner/repo
// Uses shared HTTP client for optimal connection reuse and HTTP/2 benefits
func NewGitHubClient(owner, repo string) *GitHubClient {
	if owner == "" {
		owner = DefaultOwner
	}
	if repo == "" {
		repo = DefaultRepo
	}
	return &GitHubClient{
		Owner:      owner,
		Repo:       repo,
		httpClient: getOptimizedHTTPClient(), // Use shared optimized client
	}
}

// ValidateClient checks if the client has valid owner/repo configuration
// Returns error with actionable guidance if configuration is invalid
func (c *GitHubClient) ValidateClient() error {
	if c.Owner == "" && c.Repo == "" {
		return fmt.Errorf("GitHub owner and repository not configured. Solutions:\n" +
			"  1. Run from a git repository with configured remotes\n" +
			"  2. Use --owner and --repo flags\n" +
			"  3. Ensure git remote origin points to a GitHub repository")
	}
	if c.Owner == "" {
		return fmt.Errorf("GitHub owner not configured. Solutions:\n" +
			"  1. Run from a git repository with configured remotes\n" +
			"  2. Use --owner flag to specify repository owner")
	}
	if c.Repo == "" {
		return fmt.Errorf("GitHub repository not configured. Solutions:\n" +
			"  1. Run from a git repository with configured remotes\n" +
			"  2. Use --repo flag to specify repository name")
	}
	return nil
}

// getToken retrieves GitHub token from gh CLI  
// No caching needed - auth tokens don't invalidate during single command execution
func getToken() (string, error) {
	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get GitHub token: %w\n💡 Tip: Run 'gh auth login' to authenticate", err)
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("empty token returned from gh auth token")
	}

	return token, nil
}

// GraphQLRequest represents a GraphQL request payload
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// RunGraphQLQuery executes a GraphQL query using HTTP client (legacy compatibility)
func (c *GitHubClient) RunGraphQLQuery(query string) ([]byte, error) {
	return c.RunGraphQLQueryWithVariables(query, nil)
}

// RunGraphQLQueryWithVariables executes a GraphQL query with variables using optimized HTTP client
// Optimization details documented in dev-docs/lessons-learned/shell-to-go-migration.md
func (c *GitHubClient) RunGraphQLQueryWithVariables(query string, variables map[string]interface{}) ([]byte, error) {
	// Validate client configuration before making API calls
	if err := c.ValidateClient(); err != nil {
		return nil, err
	}

	token, err := getToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub token: %w", err)
	}

	// Prepare GraphQL request
	reqPayload := GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

	// Use unified JSON marshaling for GitHub API
	jsonData, err := FormatJSON.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "spanner-mycli-dev-tools/1.0")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GraphQL request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("failed to close response body", "url", resp.Request.URL.String(), "error", err)
		}
	}()

	// Read response
	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GraphQL request failed with status %d: %s", resp.StatusCode, buf.String())
	}

	// Parse GraphQL response for errors (with JSON marshaler support for json.RawMessage)
	var graphqlResp GraphQLResponse
	if err := Unmarshal(buf.Bytes(), &graphqlResp); err == nil {
		if len(graphqlResp.Errors) > 0 {
			return nil, fmt.Errorf("GraphQL error: %s", graphqlResp.Errors[0].Message)
		}
	}

	return buf.Bytes(), nil
}

// CreatePRComment creates a comment on a pull request using GraphQL mutation
// 
// NOTE: Attempted single-request optimization, but addComment is a root-level mutation
// that requires a node ID, not accessible via nested repository context.
// Keeping the 2-step approach: query PR ID → mutation addComment
func (c *GitHubClient) CreatePRComment(prNumber, body string) error {
	prNumberInt, err := strconv.Atoi(prNumber)
	if err != nil {
		return fmt.Errorf("invalid PR number format: %w", err)
	}

	// First get the PR ID
	prIDQuery := `
	query($owner: String!, $repo: String!, $prNumber: Int!) {
	  repository(owner: $owner, name: $repo) {
	    pullRequest(number: $prNumber) {
	      id
	    }
	  }
	}`

	variables := map[string]interface{}{
		"owner":    c.Owner,
		"repo":     c.Repo,
		"prNumber": prNumberInt,
	}

	result, err := c.RunGraphQLQueryWithVariables(prIDQuery, variables)
	if err != nil {
		return fmt.Errorf("failed to get PR ID: %w", err)
	}

	var prResponse struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ID string `json:"id"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := Unmarshal(result, &prResponse); err != nil {
		return fmt.Errorf("failed to parse PR ID response: %w", err)
	}

	prID := prResponse.Data.Repository.PullRequest.ID

	// Now create the comment
	commentMutation := `
	mutation($subjectId: ID!, $body: String!) {
	  addComment(input: {
	    subjectId: $subjectId
	    body: $body
	  }) {
	    commentEdge {
	      node {
	        id
	        url
	      }
	    }
	  }
	}`

	commentVariables := map[string]interface{}{
		"subjectId": prID,
		"body":      body,
	}

	_, err = c.RunGraphQLQueryWithVariables(commentMutation, commentVariables)
	if err != nil {
		return fmt.Errorf("failed to create PR comment: %w", err)
	}

	return nil
}

// PRCreateOptions represents options for creating a pull request
type PRCreateOptions struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Draft bool   `json:"draft,omitempty"`
}

// getRepositoryID gets repository ID with simple instance-level caching
//
// DESIGN RATIONALE: Instance-level vs Global Caching
// - Typical usage: single repo (apstndb/spanner-mycli) per GitHubClient instance
// - Simple struct fields avoid sync.Map complexity and type assertions  
// - Repository IDs are immutable for repo lifetime (no TTL needed)
// - Follows YAGNI: complex global cache unnecessary for current usage patterns
func (c *GitHubClient) getRepositoryID() (string, error) {
	// Check instance cache first (repository IDs are immutable)
	if c.repositoryID != "" {
		return c.repositoryID, nil
	}
	
	// Cache miss - fetch from API
	repoQuery := `
	query($owner: String!, $repo: String!) {
	  repository(owner: $owner, name: $repo) {
	    id
	  }
	}`

	variables := map[string]interface{}{
		"owner": c.Owner,
		"repo":  c.Repo,
	}

	result, err := c.RunGraphQLQueryWithVariables(repoQuery, variables)
	if err != nil {
		return "", fmt.Errorf("failed to get repository ID: %w", err)
	}

	var repoResponse struct {
		Data struct {
			Repository struct {
				ID string `json:"id"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := Unmarshal(result, &repoResponse); err != nil {
		return "", fmt.Errorf("failed to parse repository ID response: %w", err)
	}

	// Cache in instance fields (immutable for repository lifetime)
	c.repositoryID = repoResponse.Data.Repository.ID
	
	return c.repositoryID, nil
}

// CreatePR creates a pull request using GraphQL mutation with repository ID caching
func (c *GitHubClient) CreatePR(opts PRCreateOptions) (*PRInfo, error) {
	// Get repository ID with caching optimization
	repositoryID, err := c.getRepositoryID()
	if err != nil {
		return nil, err
	}

	// Create PR using GraphQL mutation
	mutation := `
	mutation($repositoryId: ID!, $baseRefName: String!, $headRefName: String!, $title: String!, $body: String, $draft: Boolean) {
	  createPullRequest(input: {
	    repositoryId: $repositoryId
	    baseRefName: $baseRefName
	    headRefName: $headRefName
	    title: $title
	    body: $body
	    draft: $draft
	  }) {
	    pullRequest {
	      number
	      title
	      state
	    }
	  }
	}`

	mutationVariables := map[string]interface{}{
		"repositoryId":  repositoryID,
		"baseRefName":   opts.Base,
		"headRefName":   opts.Head,
		"title":         opts.Title,
		"body":          opts.Body,
		"draft":         opts.Draft,
	}

	data, err := c.RunGraphQLQueryWithVariables(mutation, mutationVariables)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	var response struct {
		Data struct {
			CreatePullRequest struct {
				PullRequest PRInfo `json:"pullRequest"`
			} `json:"createPullRequest"`
		} `json:"data"`
	}

	if err := Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse PR response: %w", err)
	}

	return &response.Data.CreatePullRequest.PullRequest, nil
}

// GetCurrentUser returns the current authenticated GitHub username
func (c *GitHubClient) GetCurrentUser() (string, error) {
	query := `
	{
	  viewer {
	    login
	  }
	}`
	
	result, err := c.RunGraphQLQuery(query)
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	
	var response struct {
		Data struct {
			Viewer struct {
				Login string `json:"login"`
			} `json:"viewer"`
		} `json:"data"`
	}
	
	if err := Unmarshal(result, &response); err != nil {
		return "", fmt.Errorf("failed to parse user response: %w", err)
	}
	
	return response.Data.Viewer.Login, nil
}

// GetCurrentBranchPR gets the PR associated with the current branch using GraphQL
func (c *GitHubClient) GetCurrentBranchPR() (*PRInfo, error) {
	// First get current branch name
	branch, err := getCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Search for PR by head branch using GraphQL
	query := `
	query($owner: String!, $repo: String!, $headRefName: String!) {
	  repository(owner: $owner, name: $repo) {
	    pullRequests(first: 1, states: OPEN, headRefName: $headRefName) {
	      nodes {
	        number
	        title
	        state
	      }
	    }
	  }
	}`

	variables := map[string]interface{}{
		"owner":       c.Owner,
		"repo":        c.Repo,
		"headRefName": branch,
	}

	data, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to search for PR: %w", err)
	}

	var response struct {
		Data struct {
			Repository struct {
				PullRequests struct {
					Nodes []PRInfo `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse PR search results: %w", err)
	}

	prs := response.Data.Repository.PullRequests.Nodes
	if len(prs) == 0 {
		return nil, fmt.Errorf("no PR found for current branch '%s'", branch)
	}

	return &prs[0], nil
}

// getCurrentBranch gets the current git branch name
func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// NodeInfo represents a GitHub issue or PR
type NodeInfo struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	State    string `json:"state"`
	NodeType string // "Issue" or "PullRequest"
}

// ResolveNumber determines if a number is an issue or PR and resolves accordingly
func (c *GitHubClient) ResolveNumber(number int) (*NodeInfo, error) {
	query := `
	query($owner: String!, $repo: String!, $number: Int!) {
	  repository(owner: $owner, name: $repo) {
	    issueOrPullRequest(number: $number) {
	      __typename
	      ... on Issue {
	        number
	        title
	        state
	      }
	      ... on PullRequest {
	        number
	        title
	        state
	      }
	    }
	  }
	}`

	variables := map[string]interface{}{
		"owner":  c.Owner,
		"repo":   c.Repo,
		"number": number,
	}

	result, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve number: %w", err)
	}

	var response struct {
		Data struct {
			Repository struct {
				IssueOrPullRequest struct {
					TypeName string `json:"__typename"`
					Number   int    `json:"number"`
					Title    string `json:"title"`
					State    string `json:"state"`
				} `json:"issueOrPullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	node := response.Data.Repository.IssueOrPullRequest
	if node.Number == 0 {
		return nil, fmt.Errorf("number %d not found", number)
	}

	return &NodeInfo{
		Number:   node.Number,
		Title:    node.Title,
		State:    node.State,
		NodeType: node.TypeName,
	}, nil
}

// ParseInputFormat parses various input formats for issues/PRs
type InputFormat struct {
	Type   string // "issue", "pr", or "auto" 
	Number int
}

// ParseInput parses input in various formats:
// - "123" -> auto-detect
// - "issues/123" or "issue/123" -> force issue
// - "pull/123" or "pr/123" -> force PR
// - "" -> current branch PR
func ParseInput(input string) (*InputFormat, error) {
	if input == "" {
		return &InputFormat{Type: "current"}, nil
	}

	// Check for explicit path formats using Cut
	prefix, numberStr, found := strings.Cut(input, "/")
	if found {
		switch prefix {
		case "issues", "issue":
			number, err := strconv.Atoi(numberStr)
			if err != nil {
				return nil, fmt.Errorf("invalid issue number in '%s': %w", input, err)
			}
			return &InputFormat{Type: "issue", Number: number}, nil
			
		case "pull", "pr":
			number, err := strconv.Atoi(numberStr)
			if err != nil {
				return nil, fmt.Errorf("invalid PR number in '%s': %w", input, err)
			}
			return &InputFormat{Type: "pr", Number: number}, nil
			
		default:
			// Has "/" but unknown prefix - treat as invalid
			return nil, fmt.Errorf("unknown format '%s': use 'issues/N', 'pull/N', 'pr/N', or plain number", input)
		}
	}

	// Plain number - auto-detect
	number, err := strconv.Atoi(input)
	if err != nil {
		return nil, fmt.Errorf("invalid number format: %s", input)
	}
	return &InputFormat{Type: "auto", Number: number}, nil
}

// ResolvePRNumber resolves PR number using multiple strategies:
// 1. If empty input, find PR associated with current git branch (via GraphQL search by branch name)
// 2. If explicit format (issues/N, pull/N), skip auto-detection
// 3. If plain number, auto-detect issue vs PR
func (c *GitHubClient) ResolvePRNumber(input string) (int, string, error) {
	format, err := ParseInput(input)
	if err != nil {
		return 0, "", err
	}

	switch format.Type {
	case "current":
		// Strategy 1: Use current branch PR
		pr, err := c.GetCurrentBranchPR()
		if err != nil {
			return 0, "", fmt.Errorf("no PR number provided and current branch has no associated PR: %w", err)
		}
		return pr.Number, fmt.Sprintf("Using current branch PR #%d: %s", pr.Number, pr.Title), nil

	case "pr":
		// Strategy 2a: Explicit PR format - skip auto-detection
		return format.Number, fmt.Sprintf("Using explicit PR #%d", format.Number), nil

	case "issue":
		// Strategy 2b: Explicit issue format - directly find associated PRs
		prs, err := c.FindPRsForIssue(format.Number)
		if err != nil {
			return 0, "", fmt.Errorf("failed to find PRs for explicit issue #%d: %w", format.Number, err)
		}
		
		// Look for open PR
		for _, pr := range prs {
			if pr.State == "OPEN" {
				return pr.Number, fmt.Sprintf("Resolved explicit issue #%d to open PR #%d: %s", format.Number, pr.Number, pr.Title), nil
			}
		}
		
		if len(prs) > 0 {
			return 0, "", fmt.Errorf("explicit issue #%d has %d associated PR(s) but none are open", format.Number, len(prs))
		}
		return 0, "", fmt.Errorf("explicit issue #%d has no associated PRs", format.Number)

	case "auto":
		// Strategy 3: Auto-detect for plain numbers
		node, err := c.ResolveNumber(format.Number)
		if err != nil {
			return 0, "", err
		}

		switch node.NodeType {
		case "PullRequest":
			return node.Number, fmt.Sprintf("Auto-detected PR #%d: %s", node.Number, node.Title), nil
		case "Issue":
			// Find associated open PR for this issue
			prs, err := c.FindPRsForIssue(node.Number)
			if err != nil {
				return 0, "", fmt.Errorf("failed to find PRs for auto-detected issue #%d: %w", node.Number, err)
			}
			
			// Look for open PR
			for _, pr := range prs {
				if pr.State == "OPEN" {
					return pr.Number, fmt.Sprintf("Auto-detected issue #%d → open PR #%d: %s", node.Number, pr.Number, pr.Title), nil
				}
			}
			
			if len(prs) > 0 {
				return 0, "", fmt.Errorf("auto-detected issue #%d has %d associated PR(s) but none are open", node.Number, len(prs))
			}
			return 0, "", fmt.Errorf("auto-detected issue #%d has no associated PRs", node.Number)
		default:
			return 0, "", fmt.Errorf("unknown node type: %s", node.NodeType)
		}

	default:
		return 0, "", fmt.Errorf("unknown input format type: %s", format.Type)
	}
}

// GetLabelIDs fetches label IDs by names
func (c *GitHubClient) GetLabelIDs(labelNames []string) (map[string]string, error) {
	query := LabelFragment + `
	query($owner: String!, $repo: String!) {
		repository(owner: $owner, name: $repo) {
			labels(first: 100) {
				nodes {
					...LabelFields
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner": c.Owner,
		"repo":  c.Repo,
	}

	var response GetLabelIDsResponse
	responseBytes, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch labels: %w", err)
	}
	if err := Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse labels response: %w", err)
	}

	labelMap := make(map[string]string, len(labelNames))
	for _, label := range response.Data.Repository.Labels.Nodes {
		labelMap[label.Name] = label.ID
	}

	return labelMap, nil
}

// GetLabelableInfo fetches information about an issue or PR
func (c *GitHubClient) GetLabelableInfo(repoID string, itemType string, number int) (*LabelableInfo, error) {
	// If itemType is empty, we need to auto-detect
	if itemType == "" {
		node, err := c.ResolveNumber(number)
		if err != nil {
			return nil, err
		}
		itemType = node.NodeType
	}

	// Use $isIssue and $isPR to conditionally include issue or pullRequest fields
	// based on the item type to avoid unnecessary queries
	query := LabelFragment + `
	query GetLabelableInfo(
		$owner: String!    # repository owner
		$repo: String!     # repository name
		$number: Int!      # issue or PR number
		$isIssue: Boolean! # flag to include issue fields
		$isPR: Boolean!    # flag to include PR fields
	) {
		repository(owner: $owner, name: $repo) {
			issue(number: $number) @include(if: $isIssue) {
				id
				number
				title
				__typename
				labels(first: 100) {
					nodes {
						...LabelFields
					}
				}
			}
			pullRequest(number: $number) @include(if: $isPR) {
				id
				number
				title
				__typename
				labels(first: 100) {
					nodes {
						...LabelFields
					}
				}
			}
		}
	}`

	isIssue := itemType == "Issue"
	isPR := itemType == "PullRequest"

	variables := map[string]interface{}{
		"owner":   c.Owner,
		"repo":    c.Repo,
		"number":  number,
		"isIssue": isIssue,
		"isPR":    isPR,
	}

	var response GetLabelableInfoResponse
	responseBytes, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch item info: %w", err)
	}
	if err := Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse item info response: %w", err)
	}

	if isIssue && response.Data.Repository.Issue != nil {
		return response.Data.Repository.Issue, nil
	}
	if isPR && response.Data.Repository.PullRequest != nil {
		return response.Data.Repository.PullRequest, nil
	}

	return nil, fmt.Errorf("item #%d not found", number)
}

// SearchItemsByTitle searches for issues and PRs by title pattern
func (c *GitHubClient) SearchItemsByTitle(repoID string, re *regexp.Regexp) ([]ItemToLabel, error) {
	// GitHub search doesn't support regex, so we'll fetch all and filter
	query := `
	query($owner: String!, $repo: String!) {
		repository(owner: $owner, name: $repo) {
			issues(first: 100, states: OPEN) {
				nodes {
					id
					number
					title
					__typename
				}
			}
			pullRequests(first: 100, states: OPEN) {
				nodes {
					id
					number
					title
					__typename
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner": c.Owner,
		"repo":  c.Repo,
	}

	var response struct {
		Data struct {
			Repository struct {
				Issues struct {
					Nodes []LabelableInfo `json:"nodes"`
				} `json:"issues"`
				PullRequests struct {
					Nodes []LabelableInfo `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
	}

	responseBytes, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to search items: %w", err)
	}
	if err := Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	var items []ItemToLabel
	
	// Filter issues by regex
	for _, issue := range response.Data.Repository.Issues.Nodes {
		if re.MatchString(issue.Title) {
			items = append(items, ItemToLabel{
				ID:     issue.ID,
				Number: issue.Number,
				Type:   issue.TypeName,
				Title:  issue.Title,
			})
		}
	}

	// Filter PRs by regex
	for _, pr := range response.Data.Repository.PullRequests.Nodes {
		if re.MatchString(pr.Title) {
			items = append(items, ItemToLabel{
				ID:     pr.ID,
				Number: pr.Number,
				Type:   pr.TypeName,
				Title:  pr.Title,
			})
		}
	}

	return items, nil
}

// AddLabelsToItem adds labels to a single item
func (c *GitHubClient) AddLabelsToItem(itemID string, labelIDs []string) (*LabelableInfo, error) {
	mutation := AllLabelFragments + `
	mutation($input: AddLabelsToLabelableInput!) {
		addLabelsToLabelable(input: $input) {
			labelable {
				...LabelableFields
			}
		}
	}`

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"labelableId": itemID,
			"labelIds":    labelIDs,
		},
	}

	var response AddLabelsToLabelableResponse
	responseBytes, err := c.RunGraphQLQueryWithVariables(mutation, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to add labels: %w", err)
	}
	if err := Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse add labels response: %w", err)
	}

	return &response.Data.AddLabelsToLabelable.Labelable, nil
}

// RemoveLabelsFromItem removes labels from a single item
func (c *GitHubClient) RemoveLabelsFromItem(itemID string, labelIDs []string) (*LabelableInfo, error) {
	mutation := AllLabelFragments + `
	mutation($input: RemoveLabelsFromLabelableInput!) {
		removeLabelsFromLabelable(input: $input) {
			labelable {
				...LabelableFields
			}
		}
	}`

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"labelableId": itemID,
			"labelIds":    labelIDs,
		},
	}

	var response RemoveLabelsFromLabelableResponse
	responseBytes, err := c.RunGraphQLQueryWithVariables(mutation, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to remove labels: %w", err)
	}
	if err := Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse remove labels response: %w", err)
	}

	return &response.Data.RemoveLabelsFromLabelable.Labelable, nil
}

// GetPRWithLinkedIssues fetches a PR with its linked issues
func (c *GitHubClient) GetPRWithLinkedIssues(prNumber int) (*PRWithLinkedIssues, error) {
	query := LabelFragment + `
	query($owner: String!, $repo: String!, $prNumber: Int!) {
		repository(owner: $owner, name: $repo) {
			pullRequest(number: $prNumber) {
				id
				number
				title
				labels(first: 100) {
					nodes {
						...LabelFields
					}
				}
				closingIssuesReferences(first: 10) {
					nodes {
						number
						labels(first: 100) {
							nodes {
								...LabelFields
							}
						}
					}
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"owner":    c.Owner,
		"repo":     c.Repo,
		"prNumber": prNumber,
	}

	var response GetPRWithLinkedIssuesResponse
	responseBytes, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR with linked issues: %w", err)
	}
	if err := Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse PR with linked issues response: %w", err)
	}

	return &response.Data.Repository.PullRequest, nil
}