package main

// GraphQL Operation Types
// This file defines all GraphQL queries, mutations, and their corresponding Go types
// for type-safe GraphQL operations across the dev-tools codebase.


// =============================================================================
// Query Types
// =============================================================================

// Basic Query Variables
type BasicRepoVariables struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type PRVariables struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	PRNumber int    `json:"prNumber"`
}

type NodeVariables struct {
	NodeID string `json:"nodeID"`
}

// =============================================================================
// Universal PR Query Types
// =============================================================================

type UniversalPRQueryVariables struct {
	Owner                   string `json:"owner"`
	Repo                    string `json:"repo"`
	PRNumber                int    `json:"prNumber"`
	IncludeReviews          bool   `json:"includeReviews"`
	IncludeThreads          bool   `json:"includeThreads"`
	IncludeStatus           bool   `json:"includeStatus"`
	IncludeMetadata         bool   `json:"includeMetadata"`
	IncludeReviewBodies     bool   `json:"includeReviewBodies"`
	IncludePagination       bool   `json:"includePagination"`
	IncludeThreadMetadata   bool   `json:"includeThreadMetadata"`
	IncludeCommentDetails   bool   `json:"includeCommentDetails"`
	ReviewLimit             int    `json:"reviewLimit"`
	ThreadLimit             int    `json:"threadLimit"`
}

// =============================================================================
// Node Query Types  
// =============================================================================

type NodeQueryVariables struct {
	NodeID                string `json:"nodeID"`
	IncludeThreadMetadata bool   `json:"includeThreadMetadata"`
	IncludeCommentDetails bool   `json:"includeCommentDetails"`
	CommentLimit          int    `json:"commentLimit"`
}

// =============================================================================
// Paginated Query Types
// =============================================================================

type PaginatedReviewQueryVariables struct {
	Owner               string `json:"owner"`
	Repo                string `json:"repo"`
	PRNumber            int    `json:"prNumber"`
	Limit               int    `json:"limit"`
	After               string `json:"after,omitempty"`
	Before              string `json:"before,omitempty"`
	IncludeReviewBodies bool   `json:"includeReviewBodies"`
}

// =============================================================================
// Mutation Types
// =============================================================================

// Thread Reply Mutation (for new comments in threads)
type AddPullRequestReviewCommentInput struct {
	ClientMutationID    *string `json:"clientMutationId,omitempty"`
	PullRequestID       *string `json:"pullRequestId,omitempty"`
	PullRequestReviewID *string `json:"pullRequestReviewId,omitempty"`
	CommitOID           *string `json:"commitOID,omitempty"`
	Body                string  `json:"body"`
	Path                *string `json:"path,omitempty"`
	Position            *int    `json:"position,omitempty"`
	InReplyTo           *string `json:"inReplyTo,omitempty"`
}

type AddPullRequestReviewCommentVariables struct {
	Input AddPullRequestReviewCommentInput `json:"input"`
}

type AddPullRequestReviewCommentResponse struct {
	Data struct {
		AddPullRequestReviewComment struct {
			Comment struct {
				ID  string `json:"id"`
				URL string `json:"url"`
			} `json:"comment"`
		} `json:"addPullRequestReviewComment"`
	} `json:"data"`
}

// Thread Reply Mutation (for replying to existing threads)
type AddPullRequestReviewThreadReplyInput struct {
	ClientMutationID              *string `json:"clientMutationId,omitempty"`
	PullRequestReviewID           *string `json:"pullRequestReviewId,omitempty"`
	PullRequestReviewThreadID     string  `json:"pullRequestReviewThreadId"`
	Body                          string  `json:"body"`
}

type AddPullRequestReviewThreadReplyVariables struct {
	Input AddPullRequestReviewThreadReplyInput `json:"input"`
}

type AddPullRequestReviewThreadReplyResponse struct {
	Data struct {
		AddPullRequestReviewThreadReply struct {
			Comment struct {
				ID   string `json:"id"`
				URL  string `json:"url"`
				Body string `json:"body"`
			} `json:"comment"`
		} `json:"addPullRequestReviewThreadReply"`
	} `json:"data"`
}

// Add Comment Mutation (for PR/Issue comments)
type AddCommentInput struct {
	ClientMutationID *string `json:"clientMutationId,omitempty"`
	SubjectID        string  `json:"subjectId"`
	Body             string  `json:"body"`
}

type AddCommentVariables struct {
	Input AddCommentInput `json:"input"`
}

type AddCommentResponse struct {
	Data struct {
		AddComment struct {
			CommentEdge struct {
				Node struct {
					ID  string `json:"id"`
					URL string `json:"url"`
				} `json:"node"`
			} `json:"commentEdge"`
		} `json:"addComment"`
	} `json:"data"`
}

// Create PR Mutation
type CreatePullRequestInput struct {
	ClientMutationID     *string `json:"clientMutationId,omitempty"`
	RepositoryID         string  `json:"repositoryId"`
	BaseRefName          string  `json:"baseRefName"`
	HeadRefName          string  `json:"headRefName"`
	HeadRepositoryID     *string `json:"headRepositoryId,omitempty"`
	Title                string  `json:"title"`
	Body                 *string `json:"body,omitempty"`
	MaintainerCanModify  *bool   `json:"maintainerCanModify,omitempty"`
	Draft                *bool   `json:"draft,omitempty"`
}

type CreatePullRequestVariables struct {
	Input CreatePullRequestInput `json:"input"`
}

type CreatePullRequestResponse struct {
	Data struct {
		CreatePullRequest struct {
			PullRequest PRInfo `json:"pullRequest"`
		} `json:"createPullRequest"`
	} `json:"data"`
}

// =============================================================================
// Repository Query Types
// =============================================================================

type RepositoryIDQueryVariables struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type RepositoryIDResponse struct {
	Data struct {
		Repository struct {
			ID string `json:"id"`
		} `json:"repository"`
	} `json:"data"`
}

type PRIDQueryVariables struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	PRNumber int    `json:"prNumber"`
}

type PRIDResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				ID string `json:"id"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// =============================================================================
// Issue/PR Resolution Types
// =============================================================================

type ResolveNumberVariables struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

type ResolveNumberResponse struct {
	Data struct {
		Repository struct {
			Issue       *IssueNode       `json:"issue"`
			PullRequest *PullRequestNode `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

type IssueNode struct {
	ID       string `json:"id"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
	NodeType string `json:"__typename"`
}

type PullRequestNode struct {
	ID       string `json:"id"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
	NodeType string `json:"__typename"`
}

// =============================================================================
// Current Branch PR Query Types
// =============================================================================

type CurrentBranchPRVariables struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
}

type CurrentBranchPRResponse struct {
	Data struct {
		Repository struct {
			PullRequests struct {
				Nodes []PRInfo `json:"nodes"`
			} `json:"pullRequests"`
		} `json:"repository"`
	} `json:"data"`
}

// =============================================================================
// Associated PRs Query Types
// =============================================================================

type AssociatedPRsVariables struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	IssueNumber int    `json:"issueNumber"`
}

type AssociatedPRsResponse struct {
	Data struct {
		Repository struct {
			Issue struct {
				TimelineItems struct {
					Nodes []TimelineItem `json:"nodes"`
				} `json:"timelineItems"`
			} `json:"issue"`
		} `json:"repository"`
	} `json:"data"`
}

type TimelineItem struct {
	Type        string   `json:"__typename"`
	PullRequest *PRInfo  `json:"pullRequest,omitempty"`
}

// =============================================================================
// Domain Types (used across multiple operations)
// =============================================================================

// PRInfo and PRCreateOptions are defined in github.go

// =============================================================================
// Review Monitor Types
// =============================================================================

type ReviewMonitorVariables struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	PRNumber int    `json:"prNumber"`
}

type ReviewMonitorResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				Reviews struct {
					Nodes []ReviewFields `json:"nodes"`
				} `json:"reviews"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// =============================================================================
// Label Operation Types
// =============================================================================

// Label represents a GitHub label
type Label struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LabelableInfo represents common fields for Issues and PRs
type LabelableInfo struct {
	ID       string `json:"id"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
	TypeName string `json:"__typename"`
	Labels   struct {
		Nodes []Label `json:"nodes"`
	} `json:"labels"`
}

// GetLabelIDsVariables for fetching label IDs by name
type GetLabelIDsVariables struct {
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	Labels []string `json:"labels"`
}

// GetLabelIDsResponse contains label ID mapping
type GetLabelIDsResponse struct {
	Data struct {
		Repository struct {
			Labels struct {
				Nodes []Label `json:"nodes"`
			} `json:"labels"`
		} `json:"repository"`
	} `json:"data"`
}

// AddLabelsToLabelableInput for adding labels mutation
type AddLabelsToLabelableInput struct {
	ClientMutationID *string  `json:"clientMutationId,omitempty"`
	LabelableID      string   `json:"labelableId"`
	LabelIDs         []string `json:"labelIds"`
}

// AddLabelsToLabelableVariables for the mutation
type AddLabelsToLabelableVariables struct {
	Input AddLabelsToLabelableInput `json:"input"`
}

// AddLabelsToLabelableResponse from the mutation
type AddLabelsToLabelableResponse struct {
	Data struct {
		AddLabelsToLabelable struct {
			Labelable LabelableInfo `json:"labelable"`
		} `json:"addLabelsToLabelable"`
	} `json:"data"`
}

// RemoveLabelsFromLabelableInput for removing labels mutation
type RemoveLabelsFromLabelableInput struct {
	ClientMutationID *string  `json:"clientMutationId,omitempty"`
	LabelableID      string   `json:"labelableId"`
	LabelIDs         []string `json:"labelIds"`
}

// RemoveLabelsFromLabelableVariables for the mutation
type RemoveLabelsFromLabelableVariables struct {
	Input RemoveLabelsFromLabelableInput `json:"input"`
}

// RemoveLabelsFromLabelableResponse from the mutation
type RemoveLabelsFromLabelableResponse struct {
	Data struct {
		RemoveLabelsFromLabelable struct {
			Labelable LabelableInfo `json:"labelable"`
		} `json:"removeLabelsFromLabelable"`
	} `json:"data"`
}

// GetLabelableInfoVariables for fetching item details
type GetLabelableInfoVariables struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

// GetLabelableInfoResponse contains item details
type GetLabelableInfoResponse struct {
	Data struct {
		Repository struct {
			Issue       *LabelableInfo `json:"issue"`
			PullRequest *LabelableInfo `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// SearchItemsByTitleVariables for searching by pattern
type SearchItemsByTitleVariables struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// SearchItemsByTitleResponse contains search results
type SearchItemsByTitleResponse struct {
	Data struct {
		Search struct {
			Nodes []LabelableInfo `json:"nodes"`
		} `json:"search"`
	} `json:"data"`
}

// PRWithLinkedIssues for add-from-issues command
type PRWithLinkedIssues struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Labels struct {
		Nodes []Label `json:"nodes"`
	} `json:"labels"`
	ClosingIssuesReferences struct {
		Nodes []struct {
			Number int `json:"number"`
			Labels struct {
				Nodes []Label `json:"nodes"`
			} `json:"labels"`
		} `json:"nodes"`
	} `json:"closingIssuesReferences"`
}

// GetPRWithLinkedIssuesVariables for fetching PR with issues
type GetPRWithLinkedIssuesVariables struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	PRNumber int    `json:"prNumber"`
}

// GetPRWithLinkedIssuesResponse contains PR and linked issues
type GetPRWithLinkedIssuesResponse struct {
	Data struct {
		Repository struct {
			PullRequest PRWithLinkedIssues `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// =============================================================================
// Issue Types (GitHub GraphQL API types and custom types)
// =============================================================================

// IssueFields represents the common fields for Issue in GitHub GraphQL API
type IssueFields struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url,omitempty"`   // Optional: not always queried
	State  string `json:"state,omitempty"` // Optional: not always queried
}

// CreateIssueInput for GitHub GraphQL API createIssue mutation
type CreateIssueInput struct {
	ClientMutationID *string  `json:"clientMutationId,omitempty"`
	RepositoryID     string   `json:"repositoryId"`
	Title            string   `json:"title"`
	Body             *string  `json:"body,omitempty"`
	LabelIDs         []string `json:"labelIds,omitempty"`
	AssigneeIDs      []string `json:"assigneeIds,omitempty"`
	MilestoneID      *string  `json:"milestoneId,omitempty"`
	ProjectIDs       []string `json:"projectIds,omitempty"`
}

// CreateIssueResponse from GitHub GraphQL API
type CreateIssueResponse struct {
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

// AddSubIssueInput for GitHub GraphQL API addSubIssue mutation
type AddSubIssueInput struct {
	ClientMutationID *string `json:"clientMutationId,omitempty"`
	IssueID          string  `json:"issueId"`
	SubIssueID       string  `json:"subIssueId"`
}

// AddSubIssueResponse from GitHub GraphQL API
type AddSubIssueResponse struct {
	Data struct {
		AddSubIssue struct {
			Issue IssueFields `json:"issue"` // GitHub API Issue type
		} `json:"addSubIssue"`
	} `json:"data"`
}

// RemoveSubIssueInput for GitHub GraphQL API removeSubIssue mutation
type RemoveSubIssueInput struct {
	ClientMutationID *string `json:"clientMutationId,omitempty"`
	IssueID          string  `json:"issueId"`
	SubIssueID       string  `json:"subIssueId"`
}

// RemoveSubIssueMutationResponse from GitHub GraphQL API
type RemoveSubIssueMutationResponse struct {
	Data struct {
		RemoveSubIssue struct {
			Issue IssueFields `json:"issue"` // GitHub API Issue type
		} `json:"removeSubIssue"`
	} `json:"data"`
}

// ReprioritizeSubIssueInput for GitHub GraphQL API reprioritizeSubIssue mutation
type ReprioritizeSubIssueInput struct {
	ClientMutationID *string `json:"clientMutationId,omitempty"`
	IssueID          string  `json:"issueId"`
	SubIssueID       string  `json:"subIssueId"`
	AfterID          *string `json:"afterId,omitempty"`
	BeforeID         *string `json:"beforeId,omitempty"`
}

// ReprioritizeSubIssueResponse from GitHub GraphQL API
type ReprioritizeSubIssueResponse struct {
	Data struct {
		ReprioritizeSubIssue struct {
			Issue IssueFields `json:"issue"` // GitHub API Issue type
		} `json:"reprioritizeSubIssue"`
	} `json:"data"`
}

// NodeQueryParentResponse for queries fetching parent issue info via node
type NodeQueryParentResponse struct {
	Data struct {
		Node struct {
			Parent *IssueFields `json:"parent"` // GitHub API Issue type (can be nil)
		} `json:"node"`
	} `json:"data"`
}

// GetRepositoryIssuesResponse for queries fetching two issues
type GetRepositoryIssuesResponse struct {
	Data struct {
		Repository struct {
			Child  *IssueFields `json:"child"`  // GitHub API Issue type
			Parent *IssueFields `json:"parent"` // GitHub API Issue type
		} `json:"repository"`
	} `json:"data"`
}

// UserQueryResponse for fetching user information
type UserQueryResponse struct {
	Data struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	} `json:"data"`
}

// MilestoneQueryResponse for fetching milestone information
type MilestoneQueryResponse struct {
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

// ProjectQueryResponse for fetching project information
type ProjectQueryResponse struct {
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

// IssueWithSubIssuesResponse for queries fetching issue with sub-issues
type IssueWithSubIssuesResponse struct {
	Data struct {
		Repository struct {
			Issue *struct {
				Number    int    `json:"number"`
				Title     string `json:"title"`
				State     string `json:"state"`
				Body      string `json:"body"`
				URL       string `json:"url"`
				CreatedAt string `json:"createdAt"`
				UpdatedAt string `json:"updatedAt"`
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
				SubIssues *struct {
					TotalCount int `json:"totalCount"`
					Nodes      []struct {
						ID     string `json:"id"`
						Number int    `json:"number"`
						Title  string `json:"title"`
						State  string `json:"state"`
						Closed bool   `json:"closed"`
					} `json:"nodes"`
				} `json:"subIssues,omitempty"`
			} `json:"issue"`
		} `json:"repository"`
	} `json:"data"`
}

// BatchSubIssueQueryResponse for batch operations on sub-issues
type BatchSubIssueQueryResponse struct {
	Data struct {
		Repository map[string]*IssueFields `json:"repository"` // GitHub API Issue type
	} `json:"data"`
}

// GetRepositoryIssueResponse for queries fetching a single issue
type GetRepositoryIssueResponse struct {
	Data struct {
		Repository struct {
			Issue *IssueFields `json:"issue"` // GitHub API Issue type
		} `json:"repository"`
	} `json:"data"`
}

// NodeQuerySubIssuesResponse for queries fetching sub-issues via node(id:)
type NodeQuerySubIssuesResponse struct {
	Data struct {
		Node struct {
			SubIssues struct {
				Nodes []struct {
					ID string `json:"id"`
				} `json:"nodes"`
			} `json:"subIssues"`
		} `json:"node"`
	} `json:"data"`
}

// IssueQueryResponse for simple issue queries with parent info
type IssueQueryResponse struct {
	Data struct {
		Repository struct {
			Issue *struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				URL   string `json:"url"`
				State string `json:"state"`
				Parent *struct {
					ID     string `json:"id"`
					Number int    `json:"number"`
					Title  string `json:"title"`
				} `json:"parent"`
			} `json:"issue"`
		} `json:"repository"`
	} `json:"data"`
}

// AddSubIssueMutationResponse for addSubIssue mutation with two issues returned
type AddSubIssueMutationResponse struct {
	Data struct {
		AddSubIssue struct {
			Issue    IssueFields `json:"issue"`    // GitHub API Issue type (parent)
			SubIssue IssueFields `json:"subIssue"` // GitHub API Issue type (child)
		} `json:"addSubIssue"`
	} `json:"data"`
}

// =============================================================================
// Common Types
// =============================================================================

type ErrorResponse struct {
	Errors []struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"errors"`
}

// =============================================================================
// Utility Types for Operation Configuration
// =============================================================================

type QueryConfig interface {
	ToGraphQLVariables() map[string]interface{}
}

type MutationConfig interface {
	ToGraphQLVariables() map[string]interface{}
}