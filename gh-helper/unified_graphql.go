package main

import (
	"encoding/json"
	"fmt"
)

// PRQueryConfig provides unified configuration for all PR data queries
type PRQueryConfig struct {
	// Basic identification
	Owner    string
	Repo     string
	PRNumber int

	// Feature flags for conditional data fetching
	IncludeReviews        bool
	IncludeThreads        bool
	IncludeStatus         bool
	IncludeMetadata       bool
	IncludeReviewBodies   bool
	IncludePagination     bool
	IncludeThreadMetadata bool // For isOutdated, subjectType, pullRequest
	IncludeCommentDetails bool // For diffHunk in comments
	IncludeComments       bool // For PR comments (issue comments)

	// Limits for data fetching
	ReviewLimit  int
	ThreadLimit  int
	CommentLimit int
}

// NewPRQueryConfig creates a basic configuration
func NewPRQueryConfig(owner, repo string, prNumber int) *PRQueryConfig {
	return &PRQueryConfig{
		Owner:        owner,
		Repo:         repo,
		PRNumber:     prNumber,
		ReviewLimit:  15,
		ThreadLimit:  50,
		CommentLimit: 50,
	}
}

// Preset configurations for common use cases

// ForReviewsOnly configures for basic review checking
func (c *PRQueryConfig) ForReviewsOnly() *PRQueryConfig {
	c.IncludeReviews = true
	c.IncludeReviewBodies = true
	c.ReviewLimit = 15
	return c
}

// ForReviewsAndStatus configures for review + CI status monitoring
func (c *PRQueryConfig) ForReviewsAndStatus() *PRQueryConfig {
	c.IncludeReviews = true
	c.IncludeStatus = true
	c.IncludeMetadata = true
	c.IncludeReviewBodies = true
	c.ReviewLimit = 15
	return c
}

// ForCompleteAnalysis configures for comprehensive review analysis
func (c *PRQueryConfig) ForCompleteAnalysis() *PRQueryConfig {
	c.IncludeReviews = true
	c.IncludeThreads = true
	c.IncludeStatus = true
	c.IncludeMetadata = true
	c.IncludeReviewBodies = true
	c.IncludePagination = true
	c.ReviewLimit = 50
	c.ThreadLimit = 100
	return c
}

// ForThreadsOnly configures for thread-focused operations
func (c *PRQueryConfig) ForThreadsOnly() *PRQueryConfig {
	c.IncludeThreads = true
	c.IncludeReviewBodies = true
	c.ThreadLimit = 100
	return c
}

// ForSingleThread configures for detailed single thread display
func (c *PRQueryConfig) ForSingleThread() *PRQueryConfig {
	c.IncludeThreads = true
	c.IncludeThreadMetadata = true
	c.IncludeCommentDetails = true
	c.ThreadLimit = 1
	return c
}

// WithThreads adds thread data to the query
func (c *PRQueryConfig) WithThreads() *PRQueryConfig {
	c.IncludeThreads = true
	return c
}

// WithComments adds PR comments to the query
func (c *PRQueryConfig) WithComments() *PRQueryConfig {
	c.IncludeComments = true
	return c
}

// ToGraphQLVariables converts config to GraphQL variables
func (c *PRQueryConfig) ToGraphQLVariables() map[string]interface{} {
	return map[string]interface{}{
		"owner":                 c.Owner,
		"repo":                  c.Repo,
		"prNumber":              c.PRNumber,
		"includeReviews":        c.IncludeReviews,
		"includeThreads":        c.IncludeThreads,
		"includeStatus":         c.IncludeStatus,
		"includeMetadata":       c.IncludeMetadata,
		"includeReviewBodies":   c.IncludeReviewBodies,
		"includePagination":     c.IncludePagination,
		"includeThreadMetadata": c.IncludeThreadMetadata,
		"includeCommentDetails": c.IncludeCommentDetails,
		"includeComments":       c.IncludeComments,
		"reviewLimit":           c.ReviewLimit,
		"threadLimit":           c.ThreadLimit,
		"commentLimit":          c.CommentLimit,
	}
}

// UniversalPRQuery - single GraphQL query for all PR data needs
const UniversalPRQuery = `
fragment PageInfoFields on PageInfo {
  hasNextPage
  hasPreviousPage
  startCursor
  endCursor
}

fragment ReviewCommentFields on PullRequestReviewComment {
  id
  body
  path
  line
  createdAt
}

fragment ReviewFields on PullRequestReview {
  id
  author { login }
  createdAt
  state
  body @include(if: $includeReviewBodies)
  comments(first: 50) @include(if: $includeReviewBodies) {
    nodes {
      ...ReviewCommentFields
    }
  }
}

fragment ThreadCommentFields on PullRequestReviewComment {
  id
  body
  author { login }
  createdAt
  diffHunk @include(if: $includeCommentDetails)
}

fragment ThreadFields on PullRequestReviewThread {
  id
  path
  line
  isResolved
  isOutdated @include(if: $includeThreadMetadata)
  subjectType @include(if: $includeThreadMetadata)
  pullRequest @include(if: $includeThreadMetadata) {
    number
    title
  }
  comments(first: 20) {
    nodes {
      ...ThreadCommentFields
    }
  }
}

fragment StatusCheckRollupFields on StatusCheckRollup {
  state
  contexts(first: 100) {
    nodes {
      __typename
      ... on CheckRun {
        name
        status
        conclusion
        isRequired(pullRequestNumber: $prNumber)
      }
      ... on StatusContext {
        context
        state
        isRequired(pullRequestNumber: $prNumber)
      }
    }
  }
}

fragment CommitWithStatusFields on Commit {
  statusCheckRollup {
    ...StatusCheckRollupFields
  }
}

query UniversalPRQuery(
  $owner: String!
  $repo: String!
  $prNumber: Int!
  
  # Feature flags with sensible defaults
  $includeReviews: Boolean! = true
  $includeThreads: Boolean! = false
  $includeStatus: Boolean! = false
  $includeMetadata: Boolean! = false
  $includeReviewBodies: Boolean! = false
  $includePagination: Boolean! = false
  $includeThreadMetadata: Boolean! = false
  $includeCommentDetails: Boolean! = false
  $includeComments: Boolean! = false
  
  # Limits with defaults
  $reviewLimit: Int = 15
  $threadLimit: Int = 50
  $commentLimit: Int = 50
) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $prNumber) {
      # Always included basic info
      number
      title
      
      # Metadata (conditional)
      mergeable @include(if: $includeMetadata)
      mergeStateStatus @include(if: $includeMetadata)
      createdAt @include(if: $includeMetadata)
      
      # Last push date
      timelineItems(last: 1, itemTypes: [HEAD_REF_FORCE_PUSHED_EVENT, PULL_REQUEST_COMMIT]) @include(if: $includeMetadata) {
        nodes {
          __typename
          ... on HeadRefForcePushedEvent {
            createdAt
          }
          ... on PullRequestCommit {
            commit {
              committedDate
            }
          }
        }
      }
      
      # Reviews (conditional)
      reviews(last: $reviewLimit) @include(if: $includeReviews) {
        nodes {
          ...ReviewFields
        }
        pageInfo @include(if: $includePagination) {
          ...PageInfoFields
        }
        totalCount @include(if: $includePagination)
      }
      
      # Review threads (conditional)
      reviewThreads(first: $threadLimit) @include(if: $includeThreads) {
        nodes {
          ...ThreadFields
        }
        pageInfo @include(if: $includePagination) {
          ...PageInfoFields
        }
        totalCount @include(if: $includePagination)
      }
      
      # Status checks (conditional)
      commits(last: 1) @include(if: $includeStatus) {
        nodes {
          commit {
            ...CommitWithStatusFields
          }
        }
      }
      
      # PR Comments (conditional)
      comments(last: $commentLimit) @include(if: $includeComments) {
        nodes {
          id
          author { login }
          body
          createdAt
        }
        pageInfo @include(if: $includePagination) {
          ...PageInfoFields
        }
        totalCount @include(if: $includePagination)
      }
    }
  }
}`

// UniversalPRResponse - unified response type for all PR queries
type UniversalPRResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				Number           int     `json:"number"`
				Title            string  `json:"title"`
				Mergeable        *string `json:"mergeable,omitempty"`
				MergeStateStatus *string `json:"mergeStateStatus,omitempty"`
				CreatedAt        *string `json:"createdAt,omitempty"`
				
				TimelineItems *struct {
					Nodes []interface{} `json:"nodes,omitempty"`
				} `json:"timelineItems,omitempty"`

				Reviews *struct {
					Nodes      []ReviewFields  `json:"nodes,omitempty"`
					PageInfo   *PageInfoFields `json:"pageInfo,omitempty"`
					TotalCount *int            `json:"totalCount,omitempty"`
				} `json:"reviews,omitempty"`

				ReviewThreads *struct {
					Nodes      []ThreadFields  `json:"nodes,omitempty"`
					PageInfo   *PageInfoFields `json:"pageInfo,omitempty"`
					TotalCount *int            `json:"totalCount,omitempty"`
				} `json:"reviewThreads,omitempty"`

				Commits *struct {
					Nodes []struct {
						Commit CommitWithStatusFields `json:"commit"`
					} `json:"nodes,omitempty"`
				} `json:"commits,omitempty"`
				
				Comments *struct {
					Nodes      []CommentFields `json:"nodes,omitempty"`
					PageInfo   *PageInfoFields `json:"pageInfo,omitempty"`
					TotalCount *int            `json:"totalCount,omitempty"`
				} `json:"comments,omitempty"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// Type-safe accessors for UniversalPRResponse

// GetReviews returns reviews if included, nil otherwise
func (r *UniversalPRResponse) GetReviews() []ReviewFields {
	if r.Data.Repository.PullRequest.Reviews == nil {
		return nil
	}
	return r.Data.Repository.PullRequest.Reviews.Nodes
}

// GetThreads returns review threads if included, nil otherwise
func (r *UniversalPRResponse) GetThreads() []ThreadFields {
	if r.Data.Repository.PullRequest.ReviewThreads == nil {
		return nil
	}
	return r.Data.Repository.PullRequest.ReviewThreads.Nodes
}

// GetStatus returns commit status if included, nil otherwise
func (r *UniversalPRResponse) GetStatus() *CommitWithStatusFields {
	if r.Data.Repository.PullRequest.Commits == nil ||
		len(r.Data.Repository.PullRequest.Commits.Nodes) == 0 {
		return nil
	}
	return &r.Data.Repository.PullRequest.Commits.Nodes[0].Commit
}

// GetStatusCheckRollup returns status check rollup if available
func (r *UniversalPRResponse) GetStatusCheckRollup() *StatusCheckRollupFields {
	status := r.GetStatus()
	if status == nil {
		return nil
	}
	return status.StatusCheckRollup
}

// GetMergeStatus returns merge status information if included
func (r *UniversalPRResponse) GetMergeStatus() (mergeable, mergeState string) {
	pr := r.Data.Repository.PullRequest
	if pr.Mergeable != nil {
		mergeable = *pr.Mergeable
	}
	if pr.MergeStateStatus != nil {
		mergeState = *pr.MergeStateStatus
	}
	return
}

// GetPRMetadata returns basic PR metadata
func (r *UniversalPRResponse) GetPRMetadata() PRMetadata {
	pr := r.Data.Repository.PullRequest
	return PRMetadata{
		Number: pr.Number,
		Title:  pr.Title,
	}
}

// HasReviews checks if reviews data was requested and available
func (r *UniversalPRResponse) HasReviews() bool {
	return r.Data.Repository.PullRequest.Reviews != nil
}

// HasThreads checks if threads data was requested and available
func (r *UniversalPRResponse) HasThreads() bool {
	return r.Data.Repository.PullRequest.ReviewThreads != nil
}

// HasStatus checks if status data was requested and available
func (r *UniversalPRResponse) HasStatus() bool {
	return r.Data.Repository.PullRequest.Commits != nil
}

// GetComments returns PR comments if included, nil otherwise
func (r *UniversalPRResponse) GetComments() []CommentFields {
	if r.Data.Repository.PullRequest.Comments == nil {
		return nil
	}
	return r.Data.Repository.PullRequest.Comments.Nodes
}

// GetTitle returns the PR title
func (r *UniversalPRResponse) GetTitle() string {
	return r.Data.Repository.PullRequest.Title
}

// GetCreatedAt returns the PR creation time if included
func (r *UniversalPRResponse) GetCreatedAt() string {
	if r.Data.Repository.PullRequest.CreatedAt != nil {
		return *r.Data.Repository.PullRequest.CreatedAt
	}
	return ""
}

// GetLastPushAt returns the last push time if included
func (r *UniversalPRResponse) GetLastPushAt() string {
	if r.Data.Repository.PullRequest.TimelineItems == nil {
		return ""
	}
	
	// The query uses `last: 1`, so we only need to check the first (and only) node
	nodes := r.Data.Repository.PullRequest.TimelineItems.Nodes
	if len(nodes) > 0 {
		if nodeMap, ok := nodes[0].(map[string]interface{}); ok {
			if typename, ok := nodeMap["__typename"].(string); ok {
				switch typename {
				case "HeadRefForcePushedEvent":
					if createdAt, ok := nodeMap["createdAt"].(string); ok {
						return createdAt
					}
				case "PullRequestCommit":
					if commit, ok := nodeMap["commit"].(map[string]interface{}); ok {
						if committedDate, ok := commit["committedDate"].(string); ok {
							return committedDate
						}
					}
				}
			}
		}
	}
	
	// Fallback to creation date if no push events found
	return r.GetCreatedAt()
}

// FetchPRData executes the universal PR query with given configuration
func (client *GitHubClient) FetchPRData(config *PRQueryConfig) (*UniversalPRResponse, error) {
	variables := config.ToGraphQLVariables()

	result, err := client.RunGraphQLQueryWithVariables(UniversalPRQuery, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR data for #%d: %w", config.PRNumber, err)
	}

	var response UniversalPRResponse
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse universal PR response: %w", err)
	}

	return &response, nil
}

// Legacy compatibility - converts UniversalPRResponse to existing types when needed

// ToReviewsResponse converts to legacy ReviewsResponse format
func (r *UniversalPRResponse) ToReviewsResponse() *ReviewsResponse {
	if !r.HasReviews() {
		return &ReviewsResponse{}
	}

	return &ReviewsResponse{
		Data: struct {
			Repository struct {
				PullRequest struct {
					Reviews struct {
						Nodes []ReviewFields `json:"nodes"`
					} `json:"reviews"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}{
			Repository: struct {
				PullRequest struct {
					Reviews struct {
						Nodes []ReviewFields `json:"nodes"`
					} `json:"reviews"`
				} `json:"pullRequest"`
			}{
				PullRequest: struct {
					Reviews struct {
						Nodes []ReviewFields `json:"nodes"`
					} `json:"reviews"`
				}{
					Reviews: struct {
						Nodes []ReviewFields `json:"nodes"`
					}{
						Nodes: r.GetReviews(),
					},
				},
			},
		},
	}
}

// ToReviewsWithStatusResponse converts to legacy format
func (r *UniversalPRResponse) ToReviewsWithStatusResponse() *ReviewsWithStatusResponse {
	return &ReviewsWithStatusResponse{
		Data: struct {
			Repository struct {
				PullRequest struct {
					Mergeable        string `json:"mergeable"`
					MergeStateStatus string `json:"mergeStateStatus"`
					Reviews          struct {
						Nodes []ReviewFields `json:"nodes"`
					} `json:"reviews"`
					Commits struct {
						Nodes []struct {
							Commit CommitWithStatusFields `json:"commit"`
						} `json:"nodes"`
					} `json:"commits"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}{
			Repository: struct {
				PullRequest struct {
					Mergeable        string `json:"mergeable"`
					MergeStateStatus string `json:"mergeStateStatus"`
					Reviews          struct {
						Nodes []ReviewFields `json:"nodes"`
					} `json:"reviews"`
					Commits struct {
						Nodes []struct {
							Commit CommitWithStatusFields `json:"commit"`
						} `json:"nodes"`
					} `json:"commits"`
				} `json:"pullRequest"`
			}{
				PullRequest: struct {
					Mergeable        string `json:"mergeable"`
					MergeStateStatus string `json:"mergeStateStatus"`
					Reviews          struct {
						Nodes []ReviewFields `json:"nodes"`
					} `json:"reviews"`
					Commits struct {
						Nodes []struct {
							Commit CommitWithStatusFields `json:"commit"`
						} `json:"nodes"`
					} `json:"commits"`
				}{
					Mergeable:        func() string { m, _ := r.GetMergeStatus(); return m }(),
					MergeStateStatus: func() string { _, s := r.GetMergeStatus(); return s }(),
					Reviews: struct {
						Nodes []ReviewFields `json:"nodes"`
					}{
						Nodes: r.GetReviews(),
					},
					Commits: struct {
						Nodes []struct {
							Commit CommitWithStatusFields `json:"commit"`
						} `json:"nodes"`
					}{
						Nodes: func() []struct {
							Commit CommitWithStatusFields `json:"commit"`
						} {
							if status := r.GetStatus(); status != nil {
								return []struct {
									Commit CommitWithStatusFields `json:"commit"`
								}{{Commit: *status}}
							}
							return nil
						}(),
					},
				},
			},
		},
	}
}