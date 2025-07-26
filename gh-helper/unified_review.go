package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// UnifiedReviewData represents all review-related data fetched in a single GraphQL query
type UnifiedReviewData struct {
	PR                PRMetadata       `json:"pr"`
	Reviews           []ReviewData     `json:"reviews"`
	Threads           []ThreadData     `json:"threads"`
	CurrentUser       string           `json:"currentUser"`
	FetchedAt         time.Time        `json:"fetchedAt"`
	ReviewPageInfo    PageInfo         `json:"reviewPageInfo"`
	ThreadPageInfo    PageInfo         `json:"threadPageInfo"`
}

// PageInfo contains pagination metadata following GitHub's GraphQL Relay spec
type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     string `json:"startCursor"`
	EndCursor       string `json:"endCursor"`
	TotalCount      int    `json:"totalCount,omitempty"` // Not always available
}

// PRMetadata contains basic PR information
type PRMetadata struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	State       string `json:"state"`
	Mergeable   string `json:"mergeable"`
	MergeStatus string `json:"mergeStatus"`
}

// ReviewData represents a complete review with all its context
type ReviewData struct {
	ID           string           `json:"id"`
	Author       string           `json:"author"`
	CreatedAt    string           `json:"createdAt"`
	State        string           `json:"state"`
	Body         string           `json:"body"`
	Comments     []ReviewComment  `json:"comments"`
	Severity     ReviewSeverity   `json:"severity"`
	ActionItems  []string         `json:"actionItems"`
}

// ReviewComment represents a comment within a review
type ReviewComment struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	Path      string `json:"path"`
	Line      *int   `json:"line"`
	CreatedAt string `json:"createdAt"`
}

// ThreadData represents a review thread with full context
type ThreadData struct {
	ID          string        `json:"id"`
	URL         string        `json:"url,omitempty"`
	Path        string        `json:"path"`
	Line        *int          `json:"line"`
	IsResolved  bool          `json:"isResolved"`
	IsOutdated  bool          `json:"isOutdated"`
	Comments    []ThreadComment `json:"comments"`
	NeedsReply  bool          `json:"needsReply"`
	LastReplier string        `json:"lastReplier"`
}

// ThreadComment represents a comment in a thread
type ThreadComment struct {
	ID        string `json:"id"`
	URL       string `json:"url,omitempty"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

// UnifiedReviewOptions controls what data to fetch
type UnifiedReviewOptions struct {
	IncludeThreads       bool   // Include review threads (inline comments)
	IncludeReviewBodies  bool   // Include full review bodies
	ThreadLimit          int    // Max threads to fetch (default: 50)
	ReviewLimit          int    // Max reviews to fetch (default: 20)
	ReviewAfterCursor    string // Pagination cursor for reviews (for next page)
	ReviewBeforeCursor   string // Pagination cursor for reviews (for previous page)
	ThreadAfterCursor    string // Pagination cursor for threads
	UnresolvedOnly       bool   // Filter to only unresolved threads
	ExcludeURLs          bool   // Exclude URLs from GraphQL query
}

// DefaultUnifiedReviewOptions returns sensible defaults
func DefaultUnifiedReviewOptions() UnifiedReviewOptions {
	return UnifiedReviewOptions{
		IncludeThreads:      true,
		IncludeReviewBodies: true,
		ThreadLimit:         50,
		ReviewLimit:         20,
	}
}

// GetUnifiedReviewData fetches all review data in a single optimized GraphQL query
// 
// CRITICAL LESSON: Unified fetching prevents missing feedback (review bodies contain architecture insights)
// Uses GraphQL @include directives for flexible data fetching and parameterized queries for safety
func (c *GitHubClient) GetUnifiedReviewData(prNumber string, opts UnifiedReviewOptions) (*UnifiedReviewData, error) {
	prNumberInt, err := strconv.Atoi(prNumber)
	if err != nil {
		return nil, fmt.Errorf("invalid PR number format: %w", err)
	}

	// Apply defaults
	if opts.ThreadLimit <= 0 {
		opts.ThreadLimit = 50
	}
	if opts.ReviewLimit <= 0 {
		opts.ReviewLimit = 20
	}

	// Determine pagination strategy based on provided cursors
	useReviewsAfter := opts.ReviewAfterCursor != ""
	useReviewsBefore := opts.ReviewBeforeCursor != ""
	useThreadsAfter := opts.ThreadAfterCursor != ""
	useDefaultReviews := !useReviewsAfter && !useReviewsBefore
	useDefaultThreads := opts.IncludeThreads && !useThreadsAfter

	// GraphQL query with safe parameterized pagination
	// Uses variables and conditional directives for safety
	query := `
query($owner: String!, $repo: String!, $prNumber: Int!, 
      $includeReviewBodies: Boolean!,
      $reviewLimit: Int!, $threadLimit: Int!,
      $useDefaultReviews: Boolean!,
      $useReviewsAfter: Boolean!, $reviewAfterCursor: String!,
      $useReviewsBefore: Boolean!, $reviewBeforeCursor: String!,
      $useDefaultThreads: Boolean!,
      $useThreadsAfter: Boolean!, $threadAfterCursor: String!,
      $excludeUrls: Boolean!) {
  viewer {
    login
  }
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $prNumber) {
      number
      title
      state
      mergeable
      mergeStateStatus
      
      # Default reviews (latest)
      reviews(last: $reviewLimit) @include(if: $useDefaultReviews) {
        totalCount
        pageInfo {
          hasNextPage
          hasPreviousPage
          startCursor
          endCursor
        }
        nodes {
          id
          author { login }
          createdAt
          state
          body @include(if: $includeReviewBodies)
          comments(first: 50) @include(if: $includeReviewBodies) {
            nodes {
              id
              body
              path
              line
              createdAt
            }
          }
        }
      }
      
      # Reviews paginated forward
      reviewsAfter: reviews(first: $reviewLimit, after: $reviewAfterCursor) @include(if: $useReviewsAfter) {
        totalCount
        pageInfo {
          hasNextPage
          hasPreviousPage
          startCursor
          endCursor
        }
        nodes {
          id
          author { login }
          createdAt
          state
          body @include(if: $includeReviewBodies)
          comments(first: 50) @include(if: $includeReviewBodies) {
            nodes {
              id
              body
              path
              line
              createdAt
            }
          }
        }
      }
      
      # Reviews paginated backward
      reviewsBefore: reviews(last: $reviewLimit, before: $reviewBeforeCursor) @include(if: $useReviewsBefore) {
        totalCount
        pageInfo {
          hasNextPage
          hasPreviousPage
          startCursor
          endCursor
        }
        nodes {
          id
          author { login }
          createdAt
          state
          body @include(if: $includeReviewBodies)
          comments(first: 50) @include(if: $includeReviewBodies) {
            nodes {
              id
              body
              path
              line
              createdAt
            }
          }
        }
      }
      
      # Default threads
      reviewThreads(first: $threadLimit) @include(if: $useDefaultThreads) {
        totalCount
        pageInfo {
          hasNextPage
          hasPreviousPage
          startCursor
          endCursor
        }
        nodes {
          id
          path
          line
          isResolved
          isOutdated
          comments(first: 20) {
            nodes {
              id
              url @skip(if: $excludeUrls)
              author { login }
              body
              createdAt
            }
          }
        }
      }
      
      # Threads paginated forward
      reviewThreadsAfter: reviewThreads(first: $threadLimit, after: $threadAfterCursor) @include(if: $useThreadsAfter) {
        totalCount
        pageInfo {
          hasNextPage
          hasPreviousPage
          startCursor
          endCursor
        }
        nodes {
          id
          path
          line
          isResolved
          isOutdated
          comments(first: 20) {
            nodes {
              id
              url @skip(if: $excludeUrls)
              author { login }
              body
              createdAt
            }
          }
        }
      }
    }
  }
}`

	variables := map[string]interface{}{
		"owner":               c.Owner,
		"repo":                c.Repo,
		"prNumber":            prNumberInt,
		"includeReviewBodies": opts.IncludeReviewBodies,
		"reviewLimit":         opts.ReviewLimit,
		"threadLimit":         opts.ThreadLimit,
		"useDefaultReviews":   useDefaultReviews,
		"useReviewsAfter":     useReviewsAfter,
		"reviewAfterCursor":   opts.ReviewAfterCursor,
		"useReviewsBefore":    useReviewsBefore,
		"reviewBeforeCursor":  opts.ReviewBeforeCursor,
		"useDefaultThreads":   useDefaultThreads,
		"useThreadsAfter":     useThreadsAfter,
		"threadAfterCursor":   opts.ThreadAfterCursor,
		"excludeUrls":         opts.ExcludeURLs,
	}

	result, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch unified review data: %w", err)
	}

	// Parse the comprehensive response
	var response struct {
		Data struct {
			Viewer struct {
				Login string `json:"login"`
			} `json:"viewer"`
			Repository struct {
				PullRequest struct {
					Number           int    `json:"number"`
					Title            string `json:"title"`
					State            string `json:"state"`
					Mergeable        string `json:"mergeable"`
					MergeStateStatus string `json:"mergeStateStatus"`
					Reviews struct {
						TotalCount int      `json:"totalCount"`
						PageInfo   PageInfo `json:"pageInfo"`
						Nodes []struct {
							ID        string `json:"id"`
							Author    struct {
								Login string `json:"login"`
							} `json:"author"`
							CreatedAt string `json:"createdAt"`
							State     string `json:"state"`
							Body      string `json:"body"`
							Comments  struct {
								Nodes []struct {
									ID        string `json:"id"`
									Body      string `json:"body"`
									Path      string `json:"path"`
									Line      *int   `json:"line"`
									CreatedAt string `json:"createdAt"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviews"`
					ReviewsAfter struct {
						TotalCount int      `json:"totalCount"`
						PageInfo   PageInfo `json:"pageInfo"`
						Nodes []struct {
							ID        string `json:"id"`
							Author    struct {
								Login string `json:"login"`
							} `json:"author"`
							CreatedAt string `json:"createdAt"`
							State     string `json:"state"`
							Body      string `json:"body"`
							Comments  struct {
								Nodes []struct {
									ID        string `json:"id"`
									Body      string `json:"body"`
									Path      string `json:"path"`
									Line      *int   `json:"line"`
									CreatedAt string `json:"createdAt"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewsAfter"`
					ReviewsBefore struct {
						TotalCount int      `json:"totalCount"`
						PageInfo   PageInfo `json:"pageInfo"`
						Nodes []struct {
							ID        string `json:"id"`
							Author    struct {
								Login string `json:"login"`
							} `json:"author"`
							CreatedAt string `json:"createdAt"`
							State     string `json:"state"`
							Body      string `json:"body"`
							Comments  struct {
								Nodes []struct {
									ID        string `json:"id"`
									Body      string `json:"body"`
									Path      string `json:"path"`
									Line      *int   `json:"line"`
									CreatedAt string `json:"createdAt"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewsBefore"`
					ReviewThreads struct {
						TotalCount int      `json:"totalCount"`
						PageInfo   PageInfo `json:"pageInfo"`
						Nodes []struct {
							ID         string `json:"id"`
							Path       string `json:"path"`
							Line       *int   `json:"line"`
							IsResolved bool   `json:"isResolved"`
							IsOutdated bool   `json:"isOutdated"`
							Comments   struct {
								Nodes []struct {
									ID        string `json:"id"`
									URL       string `json:"url"`
									Author    struct {
										Login string `json:"login"`
									} `json:"author"`
									Body      string `json:"body"`
									CreatedAt string `json:"createdAt"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
					ReviewThreadsAfter struct {
						TotalCount int      `json:"totalCount"`
						PageInfo   PageInfo `json:"pageInfo"`
						Nodes []struct {
							ID         string `json:"id"`
							Path       string `json:"path"`
							Line       *int   `json:"line"`
							IsResolved bool   `json:"isResolved"`
							IsOutdated bool   `json:"isOutdated"`
							Comments   struct {
								Nodes []struct {
									ID        string `json:"id"`
									URL       string `json:"url"`
									Author    struct {
										Login string `json:"login"`
									} `json:"author"`
									Body      string `json:"body"`
									CreatedAt string `json:"createdAt"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreadsAfter"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse unified review response: %w", err)
	}

	pr := response.Data.Repository.PullRequest
	currentUser := response.Data.Viewer.Login

	// Determine which review data to use based on pagination
	var reviewNodes []interface{}
	var reviewPageInfo PageInfo
	
	if useReviewsAfter {
		for _, review := range pr.ReviewsAfter.Nodes {
			reviewNodes = append(reviewNodes, review)
		}
		reviewPageInfo = pr.ReviewsAfter.PageInfo
		reviewPageInfo.TotalCount = pr.ReviewsAfter.TotalCount
	} else if useReviewsBefore {
		for _, review := range pr.ReviewsBefore.Nodes {
			reviewNodes = append(reviewNodes, review)
		}
		reviewPageInfo = pr.ReviewsBefore.PageInfo
		reviewPageInfo.TotalCount = pr.ReviewsBefore.TotalCount
	} else {
		for _, review := range pr.Reviews.Nodes {
			reviewNodes = append(reviewNodes, review)
		}
		reviewPageInfo = pr.Reviews.PageInfo
		reviewPageInfo.TotalCount = pr.Reviews.TotalCount
	}

	// Process reviews with severity analysis
	var reviews []ReviewData
	for _, reviewNode := range reviewNodes {
		// Convert interface{} back to structured data
		review := reviewNode.(struct {
			ID        string `json:"id"`
			Author    struct {
				Login string `json:"login"`
			} `json:"author"`
			CreatedAt string `json:"createdAt"`
			State     string `json:"state"`
			Body      string `json:"body"`
			Comments  struct {
				Nodes []struct {
					ID        string `json:"id"`
					Body      string `json:"body"`
					Path      string `json:"path"`
					Line      *int   `json:"line"`
					CreatedAt string `json:"createdAt"`
				} `json:"nodes"`
			} `json:"comments"`
		})
		
		if review.State == "PENDING" {
			continue
		}

		// Analyze review body
		severity := analyzeReviewSeverity(review.Body)
		actionItems := extractActionItems(review.Body)

		// Process review comments
		var comments []ReviewComment
		for _, comment := range review.Comments.Nodes {
			comments = append(comments, ReviewComment{
				ID:        comment.ID,
				Body:      comment.Body,
				Path:      comment.Path,
				Line:      comment.Line,
				CreatedAt: comment.CreatedAt,
			})
		}

		reviews = append(reviews, ReviewData{
			ID:          review.ID,
			Author:      review.Author.Login,
			CreatedAt:   review.CreatedAt,
			State:       review.State,
			Body:        review.Body,
			Comments:    comments,
			Severity:    severity,
			ActionItems: actionItems,
		})
	}

	// Process threads - focus on resolved status only
	var threads []ThreadData
	
	// Determine which thread nodes to process based on pagination
	threadNodes := pr.ReviewThreads.Nodes
	if useThreadsAfter && pr.ReviewThreadsAfter.Nodes != nil {
		threadNodes = pr.ReviewThreadsAfter.Nodes
	}
	
	for _, thread := range threadNodes {
		// Apply unresolved-only filter
		if opts.UnresolvedOnly && thread.IsResolved {
			continue  // Skip resolved threads when only unresolved threads are requested
		}
		
		var comments []ThreadComment
		lastReplier := ""

		for _, comment := range thread.Comments.Nodes {
			comments = append(comments, ThreadComment{
				ID:        comment.ID,
				URL:       comment.URL,
				Author:    comment.Author.Login,
				Body:      comment.Body,
				CreatedAt: comment.CreatedAt,
			})

			lastReplier = comment.Author.Login
		}

		// Simplified: NeedsReply = !IsResolved
		// The workflow is: reply with fix/explanation → resolve thread
		// So unresolved threads are the ones that need attention
		needsReply := !thread.IsResolved

		// Thread URL is the URL of the first comment
		threadURL := ""
		if len(comments) > 0 {
			threadURL = comments[0].URL
		}
		
		threads = append(threads, ThreadData{
			ID:          thread.ID,
			URL:         threadURL,
			Path:        thread.Path,
			Line:        thread.Line,
			IsResolved:  thread.IsResolved,
			IsOutdated:  thread.IsOutdated,
			Comments:    comments,
			NeedsReply:  needsReply,
			LastReplier: lastReplier,
		})
	}

	// Use the pagination info from the appropriate source

	var threadPageInfo PageInfo
	if opts.IncludeThreads {
		// Select the appropriate thread source based on pagination
		if useThreadsAfter {
			threadPageInfo = pr.ReviewThreadsAfter.PageInfo
			threadPageInfo.TotalCount = pr.ReviewThreadsAfter.TotalCount
		} else {
			threadPageInfo = pr.ReviewThreads.PageInfo
			threadPageInfo.TotalCount = pr.ReviewThreads.TotalCount
		}
	}

	return &UnifiedReviewData{
		PR: PRMetadata{
			Number:      pr.Number,
			Title:       pr.Title,
			State:       pr.State,
			Mergeable:   pr.Mergeable,
			MergeStatus: pr.MergeStateStatus,
		},
		Reviews:        reviews,
		Threads:        threads,
		CurrentUser:    currentUser,
		FetchedAt:      time.Now(),
		ReviewPageInfo: reviewPageInfo,
		ThreadPageInfo: threadPageInfo,
	}, nil
}

// analyzeReviewSeverity determines the severity of review feedback
func analyzeReviewSeverity(body string) ReviewSeverity {
	bodyLower := strings.ToLower(body)

	// Check explicit severity markers (like Gemini uses)
	if strings.Contains(body, "![critical]") || strings.Contains(bodyLower, "critical") {
		return SeverityCritical
	}
	if strings.Contains(body, "![high]") || strings.Contains(bodyLower, "high-severity") ||
		strings.Contains(bodyLower, "high-priority") {
		return SeverityHigh
	}
	// Removed medium/low levels for simplification

	// Check for critical keywords
	criticalPatterns := []string{
		"panic", "crash", "security", "vulnerability", "injection",
		"leak", "exposed", "hardcoded.*password", "hardcoded.*credential",
	}
	for _, pattern := range criticalPatterns {
		if strings.Contains(bodyLower, pattern) {
			return SeverityCritical
		}
	}

	// Check for high priority keywords
	highPatterns := []string{
		"bug", "error", "broken", "incorrect", "wrong",
		"fail", "nil.*handling", "null.*reference",
	}
	for _, pattern := range highPatterns {
		if strings.Contains(bodyLower, pattern) {
			return SeverityHigh
		}
	}

	return SeverityInfo
}

// extractActionItems finds specific issues mentioned in review
func extractActionItems(body string) []string {
	var items []string
	lines := strings.Split(body, "\n")
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		
		// Look for specific patterns that indicate action items
		actionPatterns := []string{
			"should", "must", "need to", "please", "consider",
			"fix", "update", "change", "remove", "add",
		}
		
		hasAction := false
		for _, pattern := range actionPatterns {
			if strings.Contains(lower, pattern) {
				hasAction = true
				break
			}
		}
		
		// Also check for issue indicators
		issuePatterns := []string{
			"issue", "problem", "concern", "bug", "error",
			"incorrect", "wrong", "missing", "todo", "fixme",
		}
		
		for _, pattern := range issuePatterns {
			if strings.Contains(lower, pattern) {
				hasAction = true
				break
			}
		}
		
		if hasAction && len(trimmed) > 20 && len(trimmed) < 200 {
			items = append(items, trimmed)
		}
	}
	
	// Deduplicate similar items
	seen := make(map[string]bool)
	unique := []string{}
	for _, item := range items {
		key := strings.ToLower(item)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, item)
		}
	}
	
	return unique
}


