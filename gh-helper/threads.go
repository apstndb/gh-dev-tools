package main

import (
	"fmt"
	"strconv"
)

// ThreadInfo represents a review thread with metadata
type ThreadInfo struct {
	ID          string `json:"id"`
	URL         string `json:"url,omitempty"`
	Line        *int   `json:"line"`
	Path        string `json:"path"`
	IsResolved  bool   `json:"isResolved"`
	SubjectType string `json:"subjectType"`
	NeedsReply  bool   `json:"needsReply"`
	Comments    []CommentInfo `json:"comments"`
}

// CommentInfo represents a comment within a thread
type CommentInfo struct {
	ID        string `json:"id"`
	URL       string `json:"url,omitempty"`
	Body      string `json:"body"`
	Author    string `json:"author"`
	CreatedAt string `json:"createdAt"`
	State     string `json:"state,omitempty"` // PENDING or SUBMITTED
	DiffHunk  string `json:"diffHunk,omitempty"`
}

// BatchThreadsResponse represents the response for batch thread operations
type BatchThreadsResponse struct {
	Threads     []ThreadInfo `json:"threads"`
	CurrentUser string       `json:"currentUser"`
	TotalCount  int          `json:"totalCount"`
}

// ListReviewThreads fetches all review threads for a PR with filtering and batch optimization  
// Eliminates N+1 query problem with single GraphQL request
func (c *GitHubClient) ListReviewThreads(prNumber string, needsReplyOnly, unresolvedOnly bool, limit int, excludeURLs bool) (*BatchThreadsResponse, error) {
	prNumberInt, err := strconv.Atoi(prNumber)
	if err != nil {
		return nil, fmt.Errorf("invalid PR number format: %w", err)
	}

	if limit <= 0 {
		limit = 50 // Reasonable default for batch operations
	}

	// Single GraphQL query to fetch all required data
	// OPTIMIZATION: Include viewer info to eliminate separate getCurrentUser() API call
	query := `
query($owner: String!, $repo: String!, $prNumber: Int!, $limit: Int!, $excludeUrls: Boolean!) {
  viewer {
    login
  }
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $prNumber) {
      reviewThreads(first: $limit) {
        totalCount
        nodes {
          id
          line
          path
          isResolved
          subjectType
          comments(first: 20) {
            nodes {
              id
              url @skip(if: $excludeUrls)
              body
              author {
                login
              }
              createdAt
              state
              diffHunk
            }
          }
        }
      }
    }
  }
}`

	variables := map[string]interface{}{
		"owner":      c.Owner,
		"repo":       c.Repo,
		"prNumber":   prNumberInt,
		"limit":      limit,
		"excludeUrls": excludeURLs,
	}

	result, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch threads batch: %w", err)
	}

	var response struct {
		Data struct {
			Viewer struct {
				Login string `json:"login"`
			} `json:"viewer"`
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						TotalCount int `json:"totalCount"`
						Nodes      []struct {
							ID          string `json:"id"`
							Line        *int   `json:"line"`
							Path        string `json:"path"`
							IsResolved  bool   `json:"isResolved"`
							SubjectType string `json:"subjectType"`
							Comments    struct {
								Nodes []struct {
									ID        string `json:"id"`
									URL       string `json:"url"`
									Body      string `json:"body"`
									Author    struct {
										Login string `json:"login"`
									} `json:"author"`
									CreatedAt string `json:"createdAt"`
									State     string `json:"state"`
									DiffHunk  string `json:"diffHunk"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse threads batch response: %w", err)
	}

	currentUser := response.Data.Viewer.Login
	var filteredThreads []ThreadInfo

	// Process and filter threads based on criteria
	for _, thread := range response.Data.Repository.PullRequest.ReviewThreads.Nodes {
		// Apply unresolved filter
		if unresolvedOnly && thread.IsResolved {
			continue
		}

		// Process comments
		var comments []CommentInfo
		
		for _, comment := range thread.Comments.Nodes {
			comments = append(comments, CommentInfo{
				ID:        comment.ID,
				URL:       comment.URL,
				Body:      comment.Body,
				Author:    comment.Author.Login,
				CreatedAt: comment.CreatedAt,
				State:     comment.State,
				DiffHunk:  comment.DiffHunk,
			})
		}
		
		// Simplified logic: unresolved threads need attention
		// Workflow: reply with fix/explanation → resolve thread
		needsReply := !thread.IsResolved

		// Apply needs reply filter
		if needsReplyOnly && !needsReply {
			continue
		}

		// Thread URL is the URL of the first comment
		threadURL := ""
		if len(comments) > 0 {
			threadURL = comments[0].URL
		}
		
		filteredThreads = append(filteredThreads, ThreadInfo{
			ID:          thread.ID,
			URL:         threadURL,
			Line:        thread.Line,
			Path:        thread.Path,
			IsResolved:  thread.IsResolved,
			SubjectType: thread.SubjectType,
			NeedsReply:  needsReply,
			Comments:    comments,
		})
	}

	return &BatchThreadsResponse{
		Threads:     filteredThreads,
		CurrentUser: currentUser,
		TotalCount:  response.Data.Repository.PullRequest.ReviewThreads.TotalCount,
	}, nil
}

// GetThreadBatch gets multiple threads by their IDs in a single GraphQL call
//
// BATCH PROCESSING OPTIMIZATION:
// This method demonstrates GraphQL's capability for multi-resource fetching:
// - Uses GraphQL's multi-node query to fetch multiple threads simultaneously
// - Eliminates N API calls for N threads (O(N) → O(1) optimization)
// - Maintains thread order and provides error context for invalid IDs
func (c *GitHubClient) GetThreadBatch(threadIDs []string, excludeURLs bool) (map[string]*ThreadInfo, error) {
	if len(threadIDs) == 0 {
		return map[string]*ThreadInfo{}, nil
	}

	// Construct nodes query for multiple threads
	// GraphQL allows querying multiple nodes by ID in single request
	query := `
query($ids: [ID!]!, $excludeUrls: Boolean!) {
  nodes(ids: $ids) {
    id
    ... on PullRequestReviewThread {
      line
      path
      isResolved
      subjectType
      pullRequest {
        number
        title
      }
      comments(first: 20) {
        nodes {
          id
          url @skip(if: $excludeUrls)
          body
          author {
            login
          }
          createdAt
          state
          diffHunk
        }
      }
    }
  }
}`

	variables := map[string]interface{}{
		"ids":         threadIDs,
		"excludeUrls": excludeURLs,
	}

	result, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch thread batch: %w", err)
	}

	var response struct {
		Data struct {
			Nodes []struct {
				ID          string `json:"id"`
				Line        *int   `json:"line"`
				Path        string `json:"path"`
				IsResolved  bool   `json:"isResolved"`
				SubjectType string `json:"subjectType"`
				PullRequest struct {
					Number int    `json:"number"`
					Title  string `json:"title"`
				} `json:"pullRequest"`
				Comments struct {
					Nodes []struct {
						ID        string `json:"id"`
						URL       string `json:"url"`
						Body      string `json:"body"`
						Author    struct {
							Login string `json:"login"`
						} `json:"author"`
						CreatedAt string `json:"createdAt"`
						State     string `json:"state"`
						DiffHunk  string `json:"diffHunk"`
					} `json:"nodes"`
				} `json:"comments"`
			} `json:"nodes"`
		} `json:"data"`
	}

	if err := Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse thread batch response: %w", err)
	}

	threads := make(map[string]*ThreadInfo)
	
	for _, node := range response.Data.Nodes {
		if node.ID == "" {
			continue // Skip null nodes (invalid IDs)
		}

		var comments []CommentInfo
		for _, comment := range node.Comments.Nodes {
			comments = append(comments, CommentInfo{
				ID:        comment.ID,
				URL:       comment.URL,
				Body:      comment.Body,
				Author:    comment.Author.Login,
				CreatedAt: comment.CreatedAt,
				State:     comment.State,
				DiffHunk:  comment.DiffHunk,
			})
		}

		// Thread URL is the URL of the first comment
		threadURL := ""
		if len(comments) > 0 {
			threadURL = comments[0].URL
		}
		
		threads[node.ID] = &ThreadInfo{
			ID:          node.ID,
			URL:         threadURL,
			Line:        node.Line,
			Path:        node.Path,
			IsResolved:  node.IsResolved,
			SubjectType: node.SubjectType,
			Comments:    comments,
		}
	}

	return threads, nil
}

// ReplyToThread adds a reply to a review thread and intelligently handles review submission
// The mutation will either:
// 1. Create a new review and auto-submit it (if no pending review exists)
// 2. Add to an existing pending review (requires manual submission later)
// Returns the comment ID and URL of the created comment
func (c *GitHubClient) ReplyToThread(threadID, body string, autoSubmit bool) (string, string, error) {
	// First, add the reply to the thread
	// This will either create a new review or add to an existing pending review
	replyMutation := `
mutation($threadID: ID!, $body: String!) {
  addPullRequestReviewThreadReply(input: {
    pullRequestReviewThreadId: $threadID
    body: $body
  }) {
    comment {
      id
      url
      state
      pullRequestReview {
        id
        state
        pullRequest {
          id
        }
      }
    }
  }
}`

	variables := map[string]interface{}{
		"threadID": threadID,
		"body":     body,
	}

	result, err := c.RunGraphQLQueryWithVariables(replyMutation, variables)
	if err != nil {
		return "", "", fmt.Errorf("failed to reply to thread: %w", err)
	}

	// Parse the response to check if the comment is pending
	var replyResponse struct {
		Data struct {
			AddPullRequestReviewThreadReply struct {
				Comment struct {
					ID                string `json:"id"`
					URL               string `json:"url"`
					State             string `json:"state"` // PENDING or SUBMITTED
					PullRequestReview struct {
						ID    string `json:"id"`
						State string `json:"state"` // PENDING, COMMENTED, etc.
						PullRequest struct {
							ID string `json:"id"`
						} `json:"pullRequest"`
					} `json:"pullRequestReview"`
				} `json:"comment"`
			} `json:"addPullRequestReviewThreadReply"`
		} `json:"data"`
	}

	if err := Unmarshal(result, &replyResponse); err != nil {
		return "", "", fmt.Errorf("failed to parse reply response: %w", err)
	}

	comment := replyResponse.Data.AddPullRequestReviewThreadReply.Comment
	review := comment.PullRequestReview
	
	// Store comment ID and URL to return
	commentID := comment.ID
	commentURL := comment.URL
	
	// Only attempt to submit if:
	// 1. autoSubmit is enabled (default true)
	// 2. The comment is PENDING (meaning it's part of a pending review)
	// 3. We have a valid review ID
	if autoSubmit && comment.State == "PENDING" && review.ID != "" {
		// The comment is pending, so we need to submit the review
		submitMutation := `
mutation($reviewID: ID!, $event: PullRequestReviewEvent!) {
  submitPullRequestReview(input: {
    pullRequestReviewId: $reviewID
    event: $event
  }) {
    pullRequestReview {
      id
      state
    }
  }
}`

		submitVars := map[string]interface{}{
			"reviewID": review.ID,
			"event":    "COMMENT",
		}

		_, submitErr := c.RunGraphQLQueryWithVariables(submitMutation, submitVars)
		if submitErr != nil {
			// The submission with a specific review ID failed. This can happen in a race condition
			// if a concurrent reply already submitted the review. To handle this gracefully,
			// we re-fetch the comment's state to see if it's now SUBMITTED.
			checkStateQuery := `query($commentID: ID!) { node(id: $commentID) { ... on PullRequestReviewComment { state } } }`
			checkStateVars := map[string]interface{}{"commentID": commentID}
			
			stateResult, stateErr := c.RunGraphQLQueryWithVariables(checkStateQuery, checkStateVars)
			if stateErr != nil {
				// We failed to submit and also failed to check the current state.
				// It's safest to report both errors to the user.
				return "", "", fmt.Errorf("failed to submit review, and state re-check also failed: original error: %w, check error: %w", submitErr, stateErr)
			}
			
			var stateResponse struct {
				Data struct {
					Node struct {
						State string `json:"state"`
					} `json:"node"`
				} `json:"data"`
			}
			if err := Unmarshal(stateResult, &stateResponse); err != nil {
				// The state check query returned something unexpected.
				return "", "", fmt.Errorf("failed to parse comment state check response: %w", err)
			}
			
			if stateResponse.Data.Node.State == "SUBMITTED" {
				// The comment is now submitted. This confirms a concurrent operation succeeded.
				// We can treat this as a success for the current operation as well.
				return commentID, commentURL, nil
			}
			
			// If the comment is still PENDING, the original submission error is a real issue.
			return "", "", fmt.Errorf("failed to submit review, and comment is still pending: %w", submitErr)
		}
	}

	return commentID, commentURL, nil
}

// ResolveThread resolves a review thread using GraphQL mutation
// 
// IMPORTANT: Thread resolution should only be used after addressing the feedback.
// Common workflow: reply to thread → make code changes → resolve thread
func (c *GitHubClient) ResolveThread(threadID string) error {
	mutation := `
mutation($threadID: ID!) {
  resolveReviewThread(input: {
    threadId: $threadID
  }) {
    thread {
      id
      isResolved
    }
  }
}`

	variables := map[string]interface{}{
		"threadID": threadID,
	}

	_, err := c.RunGraphQLQueryWithVariables(mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to resolve thread: %w", err)
	}

	return nil
}