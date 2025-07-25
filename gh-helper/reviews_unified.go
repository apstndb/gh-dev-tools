package main

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

// prNumberArgsHelp is defined in main.go and used here for consistency


var fetchReviewsCmd = &cobra.Command{
	Use:   "fetch [pr-number]",
	Short: "Fetch review data with configurable options",
	Long: `Fetch reviews and threads in a single optimized GraphQL query.

`+prNumberArgsHelp+`

Examples:
  # Full fetch (reviews + threads + bodies)
  gh-helper reviews fetch 306
  gh-helper reviews fetch        # Auto-detect current branch PR

  # Reviews only (no threads)
  gh-helper reviews fetch 306 --no-threads

  # Lightweight - just review states, no bodies
  gh-helper reviews fetch 306 --no-bodies

  # Custom limits and pagination
  gh-helper reviews fetch 306 --review-limit 10 --thread-limit 30
  gh-helper reviews fetch 306 --reviews-after CURSOR`,
	Args: cobra.MaximumNArgs(1),
	RunE: fetchReviews,
}

var (
	includeThreads        bool
	includeReviewBodies   bool
	threadLimit           int
	reviewLimit           int
	reviewAfterCursor     string
	reviewBeforeCursor    string
	threadAfterCursor     string
)

func init() {
	// Fetch command flags
	fetchReviewsCmd.Flags().BoolVar(&includeThreads, "threads", true, "Include review threads")
	fetchReviewsCmd.Flags().BoolVar(&includeReviewBodies, "bodies", true, "Include review bodies")
	fetchReviewsCmd.Flags().IntVar(&threadLimit, "thread-limit", 50, "Maximum threads to fetch")
	fetchReviewsCmd.Flags().IntVar(&reviewLimit, "review-limit", 20, "Maximum reviews to fetch")
	fetchReviewsCmd.Flags().Bool("threads-only", false, "Output only threads (implies --no-bodies --json)")
	fetchReviewsCmd.Flags().Bool("list-threads", false, "List thread IDs only, one per line (implies --threads-only)")
	fetchReviewsCmd.Flags().Bool("unresolved-only", false, "Include only unresolved threads")

	// Pagination flags
	fetchReviewsCmd.Flags().StringVar(&reviewAfterCursor, "reviews-after", "", "Reviews pagination: fetch after this cursor")
	fetchReviewsCmd.Flags().StringVar(&reviewBeforeCursor, "reviews-before", "", "Reviews pagination: fetch before this cursor") 
	fetchReviewsCmd.Flags().StringVar(&threadAfterCursor, "threads-after", "", "Threads pagination: fetch after this cursor")

	// Convenience flags
	fetchReviewsCmd.Flags().Bool("no-threads", false, "Exclude threads (shorthand for --threads=false)")
	fetchReviewsCmd.Flags().Bool("no-bodies", false, "Exclude bodies (shorthand for --bodies=false)")
	fetchReviewsCmd.Flags().Bool("exclude-urls", false, "Exclude URLs from output")
}

func fetchReviews(cmd *cobra.Command, args []string) error {
	client := NewGitHubClient(owner, repo)
	prNumber, err := resolvePRNumberFromArgs(args, client)
	if err != nil {
		return err
	}

	// Handle convenience and specialized flags
	if noThreads, _ := cmd.Flags().GetBool("no-threads"); noThreads {
		includeThreads = false
	}
	if noBodies, _ := cmd.Flags().GetBool("no-bodies"); noBodies {
		includeReviewBodies = false
	}
	
	// Check for specialized thread modes
	threadsOnly, _ := cmd.Flags().GetBool("threads-only")
	listThreads, _ := cmd.Flags().GetBool("list-threads")
	unresolvedOnly, _ := cmd.Flags().GetBool("unresolved-only")
	
	// Get output format using unified resolver
	// Get exclude-urls flag
	excludeURLs, err := cmd.Flags().GetBool("exclude-urls")
	if err != nil {
		return fmt.Errorf("failed to read 'exclude-urls' flag: %w", err)
	}
	
	// Adjust flags for thread-focused modes
	if listThreads || threadsOnly {
		// Force JSON for thread-focused modes by programmatically setting the flag.
		// This ensures EncodeOutputWithCmd will use JSON format when it calls ResolveFormat(cmd).
		// This approach maintains centralized output handling while allowing commands to
		// override format for specific output modes.
		if err := cmd.Flags().Set("format", "json"); err != nil {
			return fmt.Errorf("failed to set format flag: %w", err)
		}
		includeReviewBodies = false
		// No longer implicitly filter to unresolved only
		if listThreads {
			threadsOnly = true
		}
	}
	
	opts := UnifiedReviewOptions{
		IncludeThreads:      includeThreads,
		IncludeReviewBodies: includeReviewBodies,
		ThreadLimit:         threadLimit,
		ReviewLimit:         reviewLimit,
		UnresolvedOnly:      unresolvedOnly,  // Use the clearer name
		ExcludeURLs:         excludeURLs,
	}

	// Use structured logging (slog) for consistent format with JSON/YAML output
	slog.Info("fetching review data",
		"pr", prNumber,
		"options", map[string]interface{}{
			"threads": opts.IncludeThreads,
			"bodies": opts.IncludeReviewBodies,
			"review_limit": opts.ReviewLimit,
			"thread_limit": opts.ThreadLimit,
			"unresolved_only": opts.UnresolvedOnly,
		})

	data, err := client.GetUnifiedReviewData(prNumber, opts)
	if err != nil {
		return fmt.Errorf("failed to fetch unified data: %w", err)
	}

	// Handle specialized modes
	if listThreads {
		// Simple list mode: thread IDs based on filter
		for _, thread := range data.Threads {
			// If unresolvedOnly was set, data.Threads already contains only unresolved threads
			// Otherwise, show all threads
			fmt.Println(thread.ID)
		}
		return nil
	}
	
	if threadsOnly {
		// Output threads based on filter
		// If unresolvedOnly was set, data.Threads already contains only unresolved threads
		return EncodeOutputWithCmd(cmd, data.Threads)
	}
	
	// Use specialized output function to create a consistent structure for both YAML and JSON
	return outputFetch(cmd, data, includeReviewBodies, includeThreads, unresolvedOnly)
}

// outputFetch creates unified fetch output using GitHub GraphQL API types
func outputFetch(cmd *cobra.Command, data *UnifiedReviewData, includeReviewBodies bool, includeThreads bool, unresolvedOnly bool) error {
	// Use GitHub GraphQL PR metadata structure directly
	output := map[string]interface{}{
		// GitHub GraphQL PullRequest fields
		"number":           data.PR.Number,
		"title":            data.PR.Title,
		"state":            data.PR.State,
		"mergeable":        data.PR.Mergeable,
		"mergeStateStatus": data.PR.MergeStatus,
		
		// GitHub GraphQL Viewer field
		"currentUser": data.CurrentUser,
		"fetchedAt":   data.FetchedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	
	// Reviews section using GitHub GraphQL Review structure
	if includeReviewBodies {
		// Full review data with bodies
		reviews := []map[string]interface{}{}
		for _, review := range data.Reviews {
			reviewData := map[string]interface{}{
				"id":        review.ID,
				"author":    map[string]string{"login": review.Author},
				"state":     review.State,
				"createdAt": review.CreatedAt,
				"severity":  string(review.Severity),
			}
			
			if review.Body != "" {
				reviewData["body"] = review.Body
			}
			
			if len(review.ActionItems) > 0 {
				reviewData["actionItems"] = review.ActionItems
			}
			
			if len(review.Comments) > 0 {
				reviewData["commentsCount"] = len(review.Comments)
			}
			
			reviews = append(reviews, reviewData)
		}
		output["reviews"] = reviews
	} else {
		// Minimal review data without bodies
		reviews := []map[string]interface{}{}
		for _, review := range data.Reviews {
			reviews = append(reviews, map[string]interface{}{
				"id":        review.ID,
				"author":    map[string]string{"login": review.Author},
				"state":     review.State,
				"createdAt": review.CreatedAt,
			})
		}
		output["reviews"] = reviews
		output["reviewBodiesFetched"] = false
	}
	
	// Threads section using GitHub GraphQL ReviewThread structure
	if includeThreads {
		unresolvedCount := 0
		unresolvedThreads := []map[string]interface{}{}
		
		// Filter for unresolved threads to display in the output
		// Note: If unresolvedOnly flag was set, data.Threads already contains only unresolved threads
		for _, thread := range data.Threads {
			if !thread.IsResolved {
				unresolvedCount++
				
				threadData := map[string]interface{}{
					"id":         thread.ID,
					"path":       thread.Path,
					"line":       thread.Line,
					"isResolved": thread.IsResolved,
					"isOutdated": thread.IsOutdated,
				}
				
				// Include URL only if not empty (respects @skip directive)
				if thread.URL != "" {
					threadData["url"] = thread.URL
				}
				
				// Add comment information
				if len(thread.Comments) > 0 {
					last := thread.Comments[len(thread.Comments)-1]
					threadData["lastCommentBy"] = last.Author
					
					// Include all comments with author information
					comments := []map[string]interface{}{}
					for _, comment := range thread.Comments {
						commentData := map[string]interface{}{
							"id":        comment.ID,
							"author":    comment.Author,
							"createdAt": comment.CreatedAt,
							"body":      comment.Body,
						}
						
						// Include URL only if not empty (respects @skip directive)
						if comment.URL != "" {
							commentData["url"] = comment.URL
						}
						
						comments = append(comments, commentData)
					}
					threadData["comments"] = comments
				}
				
				unresolvedThreads = append(unresolvedThreads, threadData)
			}
		}
		
		// Calculate total count from page info for accuracy
		totalCount := data.ThreadPageInfo.TotalCount
		
		output["reviewThreads"] = map[string]interface{}{
			"totalCount":       totalCount,
			"unresolvedCount":  unresolvedCount,
			"unresolvedThreads": unresolvedThreads, // Unresolved threads
		}
	}
	
	// Output using unified encoder
	return EncodeOutputWithCmd(cmd, output)
}