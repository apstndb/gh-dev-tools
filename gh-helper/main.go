package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)



// Helper functions for common patterns

// resolvePRNumberFromArgs provides backwards compatibility wrapper
func resolvePRNumberFromArgs(args []string, client *GitHubClient) (string, error) {
	var input string
	if len(args) > 0 {
		input = args[0]
	}
	
	prNumberInt, _, err := client.ResolvePRNumber(input)
	if err != nil {
		return "", FetchError("PR number", err)
	}
	
	// Suppress informational messages for structured output (YAML/JSON by default)
	// These messages are for human-readable context only
	
	return fmt.Sprintf("%d", prNumberInt), nil
}

// parseTimeout provides backwards compatibility wrapper  
func parseTimeout() (time.Duration, error) {
	return ParseTimeoutString(timeoutStr)
}

// calculateEffectiveTimeout handles timeout calculation with Claude Code constraints consistently
// Returns the effective timeout and a user-friendly display string
func calculateEffectiveTimeout() (time.Duration, string, error) {
	result, err := CalculateTimeoutFromString(timeoutStr)
	if err != nil {
		return 0, "", err
	}
	
	// Show warning if timeout was constrained
	if result.Requested > 0 && result.Effective != result.Requested {
		WarningMsg("Requested timeout (%v) exceeds Claude Code limit. Using %v.", 
			result.Requested, result.Effective).Print()
	}
	
	return result.Effective, result.Display, nil
}

var rootCmd = &cobra.Command{
	Use:   "gh-helper",
	Short: "Generic GitHub operations helper",
	Long: `Generic GitHub operations optimized for AI assistants.

COMMON PATTERNS:
  gh-helper reviews wait <PR> --request-review     # Complete review workflow
  gh-helper reviews fetch <PR> --list-threads      # List thread IDs needing replies
  gh-helper threads reply <THREAD_ID> --commit-hash abc123 --message "Fixed as suggested"

See dev-tools/gh-helper/README.md for detailed documentation, design rationale,
and migration guide from shell scripts.`,
	// Cobra assumes all errors are usage errors and shows help by default
	// For operational tools, most errors are runtime issues (API failures, merge conflicts)
	// not syntax errors, so we disable automatic error printing
	SilenceErrors: true,
}

var reviewsCmd = &cobra.Command{
	Use:   "reviews",
	Short: "GitHub Pull Request review operations",
}

var threadsCmd = &cobra.Command{
	Use:   "threads",
	Short: "GitHub review thread operations",
}


var waitReviewsCmd = NewOperationalCommand(
	"wait [pr-number]",
	"Wait for both reviews and PR checks (default behavior)",
	`Continuously monitor for both new reviews AND PR checks completion by default.

This command polls every 30 seconds and waits until BOTH conditions are met:
1. New reviews are available
2. All PR checks have completed (success, failure, or cancelled)

Use --exclude-reviews to wait for PR checks only.
Use --exclude-checks to wait for reviews only.
Use --request-review to automatically request Gemini review before waiting.
Use --async to check reviews once and return immediately (non-blocking).
Use --async --detailed to get comprehensive status data including PR comments.
Use --request-summary to request and wait for Gemini summary.

`+prNumberArgsHelp+`

AI-FRIENDLY: Designed for autonomous workflows that need complete feedback.
Default timeout is 5 minutes, configurable with --timeout flag.`,
	waitForReviews,
)

// waitAllCmd removed - redundant with waitReviewsCmd which supports the same functionality


var replyThreadsCmd = NewOperationalCommand(
	"reply <thread-id> [<thread-id>...]",
	"Reply to one or more review threads",
	`Reply to GitHub pull request review threads with support for bulk operations.

This command accepts one or more thread IDs, allowing you to reply to multiple
threads with the same message or custom messages per thread.

AI-FRIENDLY DESIGN (Issue #301): The reply text can be provided via:
- --message flag for single-line responses
- stdin for multi-line content or heredoc (preferred by AI assistants)
- --commit-hash for standardized commit references
- --resolve to automatically resolve thread after replying

BULK OPERATIONS: You can specify custom messages for individual threads using
the format THREAD_ID:"Custom message" or use a uniform message for all threads.

Examples:
  # Standard response with immediate resolution
  gh-helper threads reply PRRT_kwDONC6gMM5SU-GH --message "Fixed as suggested" --resolve
  
  # Reply to multiple threads with same message
  gh-helper threads reply PRRT_1 PRRT_2 PRRT_3 --commit-hash abc123 --message "Fixed in commit" --resolve
  
  # Reply with custom messages per thread
  gh-helper threads reply PRRT_1:"Fixed the typo" PRRT_2:"Refactored as suggested" --resolve
  
  # Mix custom and default messages
  gh-helper threads reply PRRT_1 PRRT_2:"Custom fix" PRRT_3 --message "Default fix" --resolve
  
  # Explain without code changes
  gh-helper threads reply PRRT_kwDONC6gMM5SU-GH --message "This is intentional behavior for compatibility" --resolve
  
  # Multi-line response using stdin
  echo "Thank you for the feedback!" | gh-helper threads reply PRRT_kwDONC6gMM5SU-GH --resolve
  
  # Complex explanation with detailed reasoning
  gh-helper threads reply PRRT_kwDONC6gMM5SU-GH --resolve <<EOF
  After investigating, I've decided not to make this change because:
  - It would break backward compatibility with existing users
  - The current behavior is documented and expected
  - Alternative approach is available via the --legacy-mode flag
  EOF
  
  # Reference current commit hash  
  gh-helper threads reply PRRT_kwDONC6gMM5SU-GH --commit-hash <HASH> --message "Implemented suggested changes" --resolve`,
	replyToThread,
)

var showThreadCmd = &cobra.Command{
	Use:   "show <thread-id> [<thread-id>...]",
	Short: "Show detailed view of one or more review threads", 
	Long: `Show detailed view of review threads including all comments.

This command accepts one or more thread IDs, allowing you to inspect multiple
threads in a single operation. When showing a single thread, the output is a
single object. When showing multiple threads, the output is an array.

This provides full context for understanding review feedback before replying.
Useful for getting complete thread history and comment details.

Examples:
  # Show a single thread
  gh-helper threads show PRRT_kwDONC6gMM5SgXT2
  
  # Show multiple threads at once
  gh-helper threads show PRRT_kwDONC6gMM5SgXT2 PRRT_kwDONC6gMM5SgXT3
  
  # Show many threads (useful for batch inspection)
  gh-helper threads show PRRT_kwDONC6gMM5SgXT2 PRRT_kwDONC6gMM5SgXT3 PRRT_kwDONC6gMM5SgXT4`,
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
	RunE:         showThread,
}

var resolveThreadCmd = NewOperationalCommand(
	"resolve <thread-id> [<thread-id>...]",
	"Resolve one or more review threads",
	`Resolve GitHub pull request review threads.

This command accepts one or more thread IDs, allowing you to resolve multiple
threads in a single operation. This is useful after addressing feedback from
multiple reviewers or when cleaning up several related discussions.

This marks the threads as resolved, indicating that the feedback has been addressed.
Use this after making the requested changes or providing sufficient response.

Examples:
  # Resolve a single thread
  gh-helper threads resolve PRRT_kwDONC6gMM5SgXT2
  
  # Resolve multiple threads at once
  gh-helper threads resolve PRRT_kwDONC6gMM5SgXT2 PRRT_kwDONC6gMM5SgXT3
  
  # Resolve many threads after addressing all feedback
  gh-helper threads resolve PRRT_kwDONC6gMM5SgXT2 PRRT_kwDONC6gMM5SgXT3 PRRT_kwDONC6gMM5SgXT4`,
	resolveThread,
)

// replyWithCommitCmd removed - use 'threads reply' with --message for commit references

var (
	owner          string
	repo           string
	message        string
	mentionUser    string
	commitHash     string
	timeoutStr     string
	requestReview  bool
	excludeReviews bool
	excludeChecks  bool
	autoResolve    bool
	async          bool
	detailed       bool
	requestSummary bool
)

// Common help text for PR number arguments
const prNumberArgsHelp = `Arguments:
- No argument: Uses current branch's PR
- Plain number (123): PR number
- Explicit PR (pull/123, pr/123): PR reference`

// Gemini comment headers
const (
	geminiSummaryHeader = "## Summary of Changes"
	geminiReviewHeader  = "## Code Review"
)

func init() {
	// Configure Args for operational commands (using NewOperationalCommand)
	waitReviewsCmd.Args = cobra.MaximumNArgs(1)
	replyThreadsCmd.Args = cobra.MinimumNArgs(1)
	// showThreadCmd already configured with MinimumNArgs(1) in command definition
	resolveThreadCmd.Args = cobra.MinimumNArgs(1)
	
	// Configure flags
	rootCmd.PersistentFlags().StringVar(&owner, "owner", DefaultOwner, "GitHub repository owner")
	rootCmd.PersistentFlags().StringVar(&repo, "repo", DefaultRepo, "GitHub repository name")
	rootCmd.PersistentFlags().StringVar(&timeoutStr, "timeout", "5m", "Timeout duration (e.g., 90s, 1.5m, 2m30s, 15m)")
	rootCmd.PersistentFlags().String("format", "yaml", "Output format (yaml|json)")
	rootCmd.PersistentFlags().Bool("json", false, "Output JSON format (alias for --format=json)")
	rootCmd.PersistentFlags().Bool("yaml", false, "Output YAML format (alias for --format=yaml)")
	
	// Mark all format flags as mutually exclusive
	rootCmd.MarkFlagsMutuallyExclusive("format", "json")
	rootCmd.MarkFlagsMutuallyExclusive("format", "yaml")
	rootCmd.MarkFlagsMutuallyExclusive("json", "yaml")

	replyThreadsCmd.Flags().StringVar(&message, "message", "", "Reply message (or use stdin)")
	replyThreadsCmd.Flags().StringVar(&mentionUser, "mention", "", "Username to mention (without @)")
	replyThreadsCmd.Flags().StringVar(&commitHash, "commit-hash", "", "Commit hash to reference in reply")
	replyThreadsCmd.Flags().BoolVar(&autoResolve, "resolve", false, "Automatically resolve thread after replying")
	replyThreadsCmd.Flags().Bool("parallel", true, "Execute mutations concurrently")
	replyThreadsCmd.Flags().Int("max-concurrent", 5, "Maximum concurrent requests")

	waitReviewsCmd.Flags().BoolVar(&excludeReviews, "exclude-reviews", false, "Exclude reviews, wait for PR checks only")
	waitReviewsCmd.Flags().BoolVar(&excludeChecks, "exclude-checks", false, "Exclude checks, wait for reviews only")
	waitReviewsCmd.Flags().BoolVar(&requestReview, "request-review", false, "Request Gemini review before waiting")
	waitReviewsCmd.Flags().BoolVar(&async, "async", false, "Check reviews once and return immediately (non-blocking, replaces 'reviews check' for review functionality)")
	waitReviewsCmd.Flags().BoolVar(&detailed, "detailed", false, "Include comprehensive status data including PR comments (requires --async)")
	waitReviewsCmd.Flags().BoolVar(&requestSummary, "request-summary", false, "Request Gemini summary and wait for it (mutually exclusive with --async)")

	// Thread command flags
	showThreadCmd.Flags().Bool("exclude-urls", false, "Exclude URLs from output")

	// Add subcommands
	reviewsCmd.AddCommand(fetchReviewsCmd, waitReviewsCmd)
	threadsCmd.AddCommand(showThreadCmd, replyThreadsCmd, resolveThreadCmd)
	rootCmd.AddCommand(reviewsCmd, threadsCmd, labelsCmd)
}

func main() {
	// Set log level to WARN by default (suppress INFO logs)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})))
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}


// getCurrentUser returns the current authenticated GitHub username
func getCurrentUser() (string, error) {
	client := NewGitHubClient(owner, repo)
	return client.GetCurrentUser()
}

// ReviewState represents the state of the last known review
type ReviewState struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
}

// DetailedStatus represents comprehensive PR status data
type DetailedStatus struct {
	PR       string           `json:"pr"`
	Title    string           `json:"title"`
	Timeline TimelineInfo     `json:"timeline"`
	Checks   StatusChecks     `json:"checks"`
}

// TimelineInfo represents important timestamps
type TimelineInfo struct {
	PRCreated                string `json:"prCreated"`
	LastPush                 string `json:"lastPush"`
	LastReviewThreadResolved string `json:"lastReviewThreadResolved,omitempty"`
}

// StatusChecks represents all status checks
type StatusChecks struct {
	ReviewThreads   ThreadStatus         `json:"reviewThreads"`
	Reviews         ReviewApprovalStatus `json:"reviews"`
	CIStatus        CICheckStatus        `json:"ciStatus"`
	Mergeability    MergeConflictStatus  `json:"mergeability"`
	GeminiComments  CommentAnalysis      `json:"geminiComments,omitempty"`
}

// ThreadStatus represents review thread status
type ThreadStatus struct {
	Status     string `json:"status"`
	Resolved   int    `json:"resolved"`
	Unresolved int    `json:"unresolved"`
}

// ReviewApprovalStatus represents review approval status
type ReviewApprovalStatus struct {
	Status           string `json:"status"`
	Required         int    `json:"required"`
	Approved         int    `json:"approved"`
	ChangesRequested int    `json:"changesRequested"`
}

// CICheckStatus represents CI/CD check status
type CICheckStatus struct {
	Status   string   `json:"status"`
	Required []string `json:"required"`
	Passed   []string `json:"passed"`
	Failed   []string `json:"failed"`
}

// MergeConflictStatus represents merge conflict status
type MergeConflictStatus struct {
	Status    string `json:"status"`
	Conflicts bool   `json:"conflicts"`
	State     string `json:"state"`
}

// CommentAnalysis represents PR comment analysis
type CommentAnalysis struct {
	HasSummaryComment    bool             `json:"hasSummaryComment"`
	HasReviewComment     bool             `json:"hasReviewComment"`
	LastCommentIsSummary bool             `json:"lastCommentIsSummary"`
	Comments             []PRCommentInfo  `json:"comments"`
}

// PRCommentInfo represents a PR comment
type PRCommentInfo struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Body      string `json:"body"`
}

// loadReviewState loads the last known review state from cache
func loadReviewState(prNumber string) (*ReviewState, error) {
	stateDir := filepath.Join(GetCacheDir(), "reviews")
	lastReviewFile := filepath.Join(stateDir, fmt.Sprintf("pr-%s-last-review.json", prNumber))
	
	data, err := os.ReadFile(lastReviewFile)
	if err != nil {
		return nil, err
	}
	
	var state ReviewState
	if err := Unmarshal(data, &state); err != nil {
		return nil, err
	}
	
	return &state, nil
}

// saveReviewState saves the review state to cache
func saveReviewState(prNumber string, state ReviewState) error {
	stateDir := filepath.Join(GetCacheDir(), "reviews")
	lastReviewFile := filepath.Join(stateDir, fmt.Sprintf("pr-%s-last-review.json", prNumber))
	
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	
	data, err := yaml.MarshalWithOptions(state, yaml.UseJSONMarshaler())
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	
	if err := os.WriteFile(lastReviewFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}
	
	return nil
}

// hasNewReviews checks if there are new reviews since the last known state
func hasNewReviews(reviews []ReviewFields, lastState *ReviewState) bool {
	if lastState == nil {
		// No previous state, any review is "new"
		return len(reviews) > 0
	}
	
	for _, review := range reviews {
		if review.CreatedAt > lastState.CreatedAt ||
			(review.CreatedAt == lastState.CreatedAt && review.ID != lastState.ID) {
			return true
		}
	}
	
	return false
}

// checkClaudeCodeEnvironment checks for Claude Code timeout environment variables
// and provides guidance based on GitHub issues research.
//
// Key findings from anthropics/claude-code#1039, anthropics/claude-code#1216, anthropics/claude-code#1717:
// - BASH_MAX_TIMEOUT_MS: Upper limit for explicit timeout requests (our use case)
// - BASH_DEFAULT_TIMEOUT_MS: Default timeout when no explicit timeout specified  
// - Claude Code defaults to 2-minute hard limit when no env vars are set
// - Environment variables are read from ~/.claude/settings.json or project .claude/settings.json
// - Project settings should be committed, local settings (.claude/settings.local.json) should not
func checkClaudeCodeEnvironment() (time.Duration, bool) {
	// Check for BASH_MAX_TIMEOUT_MS (upper limit for explicit timeouts)
	if maxTimeout, err := ParseClaudeCodeTimeoutEnv("BASH_MAX_TIMEOUT_MS"); err != nil {
		fmt.Printf("⚠️  %v\n", err)
	} else if maxTimeout > 0 {
		fmt.Printf("🔧 Claude Code BASH_MAX_TIMEOUT_MS detected: %v\n", maxTimeout)
		return maxTimeout, true
	}
	
	// Check for BASH_DEFAULT_TIMEOUT_MS (default when no timeout specified)
	if defaultTimeout, err := ParseClaudeCodeTimeoutEnv("BASH_DEFAULT_TIMEOUT_MS"); err != nil {
		fmt.Printf("⚠️  %v\n", err)
	} else if defaultTimeout > 0 {
		fmt.Printf("🔧 Claude Code BASH_DEFAULT_TIMEOUT_MS detected: %v\n", defaultTimeout)
		return defaultTimeout, true
	}
	
	return 0, false
}

// performAsyncReviewCheck performs a single review check without waiting
// This replaces the functionality of the removed checkReviews command
func performAsyncReviewCheck(client *GitHubClient, prNumber string) error {
	StatusMsg("Checking reviews for PR #%s in %s/%s...", prNumber, owner, repo).Print()

	// Fetch current review data
	opts := DefaultUnifiedReviewOptions()
	opts.IncludeReviewBodies = true
	opts.IncludeThreads = false // For check mode, we just need review state

	data, err := client.GetUnifiedReviewData(prNumber, opts)
	if err != nil {
		return fmt.Errorf("failed to fetch review data: %w", err)
	}

	StatusMsg("Found %d reviews for PR #%s", len(data.Reviews), prNumber).Print()

	// Load existing review state
	lastState, err := loadReviewState(prNumber)
	if err == nil {
		fmt.Printf("Last known review: %s at %s\n", lastState.ID, lastState.CreatedAt)

		// Check for new reviews
		hasNew := false
		for _, review := range data.Reviews {
			if review.CreatedAt > lastState.CreatedAt ||
				(review.CreatedAt == lastState.CreatedAt && review.ID != lastState.ID) {
				hasNew = true
				fmt.Printf("\n🎉 New review from %s at %s (%s)\n", review.Author, review.CreatedAt, review.State)
				if review.Body != "" {
					preview := review.Body
					if len(preview) > 100 {
						preview = preview[:100] + "..."
					}
					fmt.Printf("Preview: %s\n", preview)
				}
			}
		}

		if !hasNew {
			InfoMsg("No new reviews since last check").Print()
		}
	} else {
		// Provide more specific error information for non-file-not-found errors
		if !os.IsNotExist(err) {
			slog.Info("failed to load previous review state", "pr", prNumber, "error", err)
		}
		WarningMsg("No previous state found or state could not be loaded, showing all recent reviews...").Print()
		fmt.Printf("\n📋 Found %d review(s) total\n", len(data.Reviews))
		for _, review := range data.Reviews {
			fmt.Printf("  - %s at %s (%s)\n", review.Author, review.CreatedAt, review.State)
		}
	}

	// Update state with the latest review
	if len(data.Reviews) > 0 {
		latestReview := data.Reviews[len(data.Reviews)-1]
		newState := &ReviewState{
			ID:        latestReview.ID,
			CreatedAt: latestReview.CreatedAt,
		}
		if err := saveReviewState(prNumber, *newState); err != nil {
			slog.Warn("failed to save review state", "pr", prNumber, "error", err)
		} else {
			fmt.Printf("\n💾 Updated state: Latest review %s at %s\n", latestReview.ID, latestReview.CreatedAt)
		}
	}

	fmt.Println("\n✅ Review check complete")
	return nil
}

// performDetailedStatusCheck performs comprehensive status check including PR comments
func performDetailedStatusCheck(cmd *cobra.Command, client *GitHubClient, prNumber string) error {
	StatusMsg("Collecting detailed status for PR #%s...", prNumber).Print()
	
	// Convert PR number to integer
	prNumberInt, err := strconv.Atoi(prNumber)
	if err != nil {
		return fmt.Errorf("invalid PR number format: %w", err)
	}
	
	// Fetch comprehensive PR data
	config := NewPRQueryConfig(owner, repo, prNumberInt).
		ForReviewsAndStatus().
		WithThreads().
		WithComments()
	
	response, err := client.FetchPRData(config)
	if err != nil {
		return fmt.Errorf("failed to fetch PR data: %w", err)
	}
	
	// Build detailed status
	status := DetailedStatus{
		PR:    prNumber,
		Title: response.GetTitle(),
	}
	
	// Timeline information
	status.Timeline = TimelineInfo{
		PRCreated: response.GetCreatedAt(),
		LastPush:  response.GetLastPushAt(),
	}
	
	// Review threads status
	threads := response.GetThreads()
	resolved := 0
	unresolved := 0
	var lastResolvedTime string
	
	for _, thread := range threads {
		if thread.IsResolved {
			resolved++
			// Track last resolved time
			if len(thread.Comments.Nodes) > 0 {
				lastComment := thread.Comments.Nodes[len(thread.Comments.Nodes)-1]
				if lastResolvedTime == "" || lastComment.CreatedAt > lastResolvedTime {
					lastResolvedTime = lastComment.CreatedAt
				}
			}
		} else {
			unresolved++
		}
	}
	
	status.Timeline.LastReviewThreadResolved = lastResolvedTime
	
	threadStatus := "pass"
	if unresolved > 0 {
		threadStatus = "fail"
	}
	status.Checks.ReviewThreads = ThreadStatus{
		Status:     threadStatus,
		Resolved:   resolved,
		Unresolved: unresolved,
	}
	
	// Reviews status
	reviews := response.GetReviews()
	approved := 0
	changesRequested := 0
	
	// Track latest review per author
	latestReviews := make(map[string]ReviewFields)
	for _, review := range reviews {
		if existing, ok := latestReviews[review.Author.Login]; !ok || review.CreatedAt > existing.CreatedAt {
			latestReviews[review.Author.Login] = review
		}
	}
	
	// Count current state
	for _, review := range latestReviews {
		switch review.State {
		case "APPROVED":
			approved++
		case "CHANGES_REQUESTED":
			changesRequested++
		}
	}
	
	// TODO: Get required reviews from branch protection rules
	// For now, assume 1 required
	required := 1
	
	reviewStatus := "pass"
	if changesRequested > 0 || approved < required {
		reviewStatus = "fail"
	}
	
	status.Checks.Reviews = ReviewApprovalStatus{
		Status:           reviewStatus,
		Required:         required,
		Approved:         approved,
		ChangesRequested: changesRequested,
	}
	
	// CI Status
	statusCheckRollup := response.GetStatusCheckRollup()
	if statusCheckRollup != nil {
		ciStatus := "pass"
		var passed []string
		var failed []string
		var required []string
		
		// Extract check information from contexts
		for _, context := range statusCheckRollup.Contexts.Nodes {
			contextName := ""
			contextState := ""
			
			switch context.Typename {
			case "StatusContext":
				contextName = context.Context
				contextState = context.State
			case "CheckRun":
				contextName = context.Name
				// Map CheckRun status/conclusion to state
				if context.Conclusion != "" {
					// CheckRun completed
					switch context.Conclusion {
					case "SUCCESS":
						contextState = "SUCCESS"
					case "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED":
						contextState = "FAILURE"
					default:
						contextState = "ERROR"
					}
				} else {
					// CheckRun in progress
					contextState = "PENDING"
				}
			}
			
			if contextName != "" {
				if context.IsRequired {
					required = append(required, contextName)
				}
				
				switch contextState {
				case "SUCCESS":
					passed = append(passed, contextName)
				case "FAILURE", "ERROR":
					failed = append(failed, contextName)
					if context.IsRequired {
						ciStatus = "fail"
					}
				case "PENDING":
					if context.IsRequired {
						ciStatus = "pending"
					}
				}
			}
		}
		
		status.Checks.CIStatus = CICheckStatus{
			Status:   ciStatus,
			Required: required,
			Passed:   passed,
			Failed:   failed,
		}
	} else {
		// No CI checks configured
		status.Checks.CIStatus = CICheckStatus{
			Status:   "pass",
			Required: []string{},
			Passed:   []string{},
			Failed:   []string{},
		}
	}
	
	// Mergeability
	mergeable, mergeState := response.GetMergeStatus()
	mergeStatus := "pass"
	conflicts := false
	
	switch mergeable {
	case "CONFLICTING":
		mergeStatus = "fail"
		conflicts = true
	case "UNKNOWN":
		mergeStatus = "pending"
	}
	
	status.Checks.Mergeability = MergeConflictStatus{
		Status:    mergeStatus,
		Conflicts: conflicts,
		State:     mergeState,
	}
	
	// PR Comments Analysis
	comments := response.GetComments()
	if len(comments) > 0 {
		analysis := CommentAnalysis{
			Comments: []PRCommentInfo{},
		}
		
		for _, comment := range comments {
			// Categorize comment
			commentType := "comment"
			if strings.Contains(comment.Body, geminiSummaryHeader) {
				commentType = "summary"
				analysis.HasSummaryComment = true
			} else if strings.Contains(comment.Body, geminiReviewHeader) {
				commentType = "review"
				analysis.HasReviewComment = true
			}
			
			analysis.Comments = append(analysis.Comments, PRCommentInfo{
				Type:      commentType,
				Timestamp: comment.CreatedAt,
				Body:      comment.Body,
			})
		}
		
		// Check if last comment is summary
		if len(analysis.Comments) > 0 {
			lastComment := analysis.Comments[len(analysis.Comments)-1]
			analysis.LastCommentIsSummary = lastComment.Type == "summary"
		}
		
		status.Checks.GeminiComments = analysis
	}
	
	// Output the detailed status
	format := ResolveFormat(cmd)
	output := map[string]interface{}{
		"detailedStatus": status,
	}
	
	return EncodeOutput(os.Stdout, format, output)
}

// performRequestSummaryAndWait requests a Gemini summary and waits for it
func performRequestSummaryAndWait(cmd *cobra.Command, client *GitHubClient, prNumber string) error {
	fmt.Printf("📝 Requesting Gemini summary for PR #%s...\n", prNumber)
	
	// Post /gemini summary comment
	if err := client.CreatePRComment(prNumber, "/gemini summary"); err != nil {
		return fmt.Errorf("failed to request Gemini summary: %w", err)
	}
	
	fmt.Println("✅ Gemini summary requested")
	fmt.Println("⏳ Waiting for summary to be posted...")
	
	// Convert PR number to integer
	prNumberInt, err := strconv.Atoi(prNumber)
	if err != nil {
		return fmt.Errorf("invalid PR number format: %w", err)
	}
	
	// Calculate timeout
	effectiveTimeout, timeoutDisplay, err := calculateEffectiveTimeout()
	if err != nil {
		return err
	}
	
	fmt.Printf("🔄 Waiting for summary (timeout: %s)...\n", timeoutDisplay)
	
	// Get initial comments count
	config := NewPRQueryConfig(owner, repo, prNumberInt).WithComments()
	initialResponse, err := client.FetchPRData(config)
	if err != nil {
		return fmt.Errorf("failed to fetch initial PR data: %w", err)
	}
	
	initialComments := initialResponse.GetComments()
	initialCount := len(initialComments)
	
	// Check if there's already a summary in the last few comments
	foundExistingSummary := false
	for i := len(initialComments) - 1; i >= 0 && i >= len(initialComments)-3; i-- {
		if strings.Contains(initialComments[i].Body, geminiSummaryHeader) {
			foundExistingSummary = true
			break
		}
	}
	
	if foundExistingSummary {
		fmt.Println("✅ Found existing summary in recent comments")
		return nil
	}
	
	startTime := time.Now()
	
	// Poll for new summary comment
	for {
		// Check timeout
		if time.Since(startTime) > effectiveTimeout {
			fmt.Printf("\n⏰ Timeout reached (%v). Summary not posted yet.\n", effectiveTimeout)
			return fmt.Errorf("timeout waiting for Gemini summary")
		}
		
		// Wait before checking
		time.Sleep(5 * time.Second)
		
		// Fetch updated comments
		response, err := client.FetchPRData(config)
		if err != nil {
			fmt.Printf("Error fetching PR data: %v\n", err)
			continue
		}
		
		comments := response.GetComments()
		
		// Check for new comments
		if len(comments) > initialCount {
			// Check new comments for summary
			for i := initialCount; i < len(comments); i++ {
				if strings.Contains(comments[i].Body, geminiSummaryHeader) {
					fmt.Printf("\n🎉 Summary posted by %s at %s\n", 
						comments[i].Author.Login, comments[i].CreatedAt)
					
					// Show preview
					preview := comments[i].Body
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					fmt.Printf("\nPreview:\n%s\n", preview)
					
					return nil
				}
			}
		}
		
		elapsed := time.Since(startTime)
		remaining := effectiveTimeout - elapsed
		fmt.Printf("[%s] Waiting for summary... (remaining: %v)\n",
			time.Now().Format("15:04:05"), remaining.Truncate(time.Second))
	}
}

func waitForReviews(cmd *cobra.Command, args []string) error {
	client := NewGitHubClient(owner, repo)
	prNumber, err := resolvePRNumberFromArgs(args, client)
	if err != nil {
		return err
	}
	
	// Validate flags
	if excludeReviews && excludeChecks {
		return fmt.Errorf("cannot exclude both reviews and checks")
	}
	
	// Validate mutually exclusive flags
	if requestSummary && async {
		return fmt.Errorf("--request-summary and --async are mutually exclusive")
	}
	
	// Validate --detailed requires --async
	if detailed && !async {
		return fmt.Errorf("--detailed requires --async")
	}
	
	// Handle async mode - single check and return (replaces reviews check)
	if async {
		if detailed {
			InfoMsg("Running in async mode with detailed status").Print()
			return performDetailedStatusCheck(cmd, client, prNumber)
		} else if !excludeReviews {
			InfoMsg("Running in async mode (single check, no waiting)").Print()
			return performAsyncReviewCheck(client, prNumber)
		} else {
			return fmt.Errorf("async mode currently only supports review checking")
		}
	}
	
	// Handle request-summary mode
	if requestSummary {
		return performRequestSummaryAndWait(cmd, client, prNumber)
	}
	
	// Determine what to wait for
	waitForReviews := !excludeReviews
	waitForChecks := !excludeChecks
	
	// Request Gemini review if flag is set
	if requestReview && waitForReviews {
		fmt.Printf("📝 Requesting Gemini review for PR #%s...\n", prNumber)
		if err := client.CreatePRComment(prNumber, "/gemini review"); err != nil {
			return fmt.Errorf("failed to request Gemini review: %w", err)
		}
		fmt.Println("✅ Gemini review requested")
	}
	
	// Display what we're waiting for
	waitingFor := []string{}
	if waitForReviews {
		waitingFor = append(waitingFor, "reviews")
	}
	if waitForChecks {
		waitingFor = append(waitingFor, "PR checks")
	}
	
	// Calculate timeout with Claude Code constraints
	_, timeoutDisplay, err := calculateEffectiveTimeout()
	if err != nil {
		return err
	}
	
	fmt.Printf("🔄 Waiting for %s on PR #%s (timeout: %s)...\n", 
		strings.Join(waitingFor, " and "), prNumber, timeoutDisplay)
	fmt.Println("Press Ctrl+C to stop monitoring")

	// For now, simply delegate to waitForReviewsAndChecks with appropriate flags
	// This ensures the new default behavior (both reviews and checks) works
	
	// Temporarily override global flags for delegation
	originalRequestReview := requestReview
	defer func() { requestReview = originalRequestReview }()
	
	// Disable review request in delegated function since we already handled it above
	requestReview = false
	
	// If we're only waiting for reviews, use the original simpler logic
	if waitForReviews && !waitForChecks {
		fmt.Printf("⚠️  Reviews-only mode: Using simplified wait logic\n")
		// Simple polling for reviews only (original behavior)
		return waitForReviewsOnly(prNumber)
	}
	
	// For all other cases (checks-only or both), delegate to the full implementation
	err = waitForReviewsAndChecks(cmd, args)
	// Don't wrap the error to avoid double error messages
	return err
}

// waitForReviewsOnly waits specifically for new reviews without checking PR status
func waitForReviewsOnly(prNumber string) error {
	// Convert PR number to integer for GraphQL
	prNumberInt, err := strconv.Atoi(prNumber)
	if err != nil {
		return fmt.Errorf("invalid PR number format: %w", err)
	}
	
	// Create GitHub client once for better performance (token caching)
	client := NewGitHubClient(owner, repo)
	
	// Apply Claude Code timeout constraints
	effectiveTimeout, timeoutDisplay, err := calculateEffectiveTimeout()
	if err != nil {
		return err
	}
	
	fmt.Printf("🔄 Waiting for reviews only on PR #%s (timeout: %s)...\n", prNumber, timeoutDisplay)
	fmt.Println("Press Ctrl+C to stop monitoring")
	
	// Load existing state
	lastState, err := loadReviewState(prNumber)
	if err == nil {
		fmt.Printf("📊 Tracking reviews since: %s\n", lastState.CreatedAt)
	}
	
	startTime := time.Now()
	for {
		// Check timeout
		if time.Since(startTime) > effectiveTimeout {
			fmt.Printf("\n⏰ Timeout reached (%v). No new reviews found.\n", effectiveTimeout)
			return nil
		}
		
		// Use unified architecture for review polling
		config := NewPRQueryConfig(owner, repo, prNumberInt).ForReviewsOnly()
		response, err := client.FetchPRData(config)
		if err != nil {
			fmt.Printf("Error fetching reviews: %v\n", err)
			time.Sleep(30 * time.Second)
			continue
		}
		
		reviews := response.GetReviews()
		
		if hasNewReviews(reviews, lastState) {
			// Find and display new reviews
			if lastState == nil {
				fmt.Printf("\n🎉 Found %d review(s)\n", len(reviews))
			} else {
				// Show details of new reviews
				for _, review := range reviews {
					if review.CreatedAt > lastState.CreatedAt ||
						(review.CreatedAt == lastState.CreatedAt && review.ID != lastState.ID) {
						fmt.Printf("\n🎉 New review detected from %s at %s\n", review.Author.Login, review.CreatedAt)
						if review.Body != "" && len(review.Body) > 100 {
							fmt.Printf("Preview: %s...\n", review.Body[:100])
						}
						break // Show only the first new review for brevity
					}
				}
			}
			
			// Update state with latest review
			if len(reviews) > 0 {
				latestReview := reviews[len(reviews)-1]
				newState := ReviewState{
					ID:        latestReview.ID,
					CreatedAt: latestReview.CreatedAt,
				}
				_ = saveReviewState(prNumber, newState) // Best effort state save
			}
			
			fmt.Println("\n✅ New reviews available!")
			ListThreadsGuidance(prNumber).Print()
			fmt.Println("⚠️  IMPORTANT: Please read the review feedback carefully before proceeding")
			return nil
		}
		
		elapsed := time.Since(startTime)
		remaining := effectiveTimeout - elapsed
		fmt.Printf("[%s] No new reviews yet (remaining: %v)\n",
			time.Now().Format("15:04:05"), remaining.Truncate(time.Second))
		
		time.Sleep(30 * time.Second)
	}
}

// Status message maps for consistent display formatting
var (
	mergeStatusMessages = map[string]string{
		"MERGEABLE":   "✅ Ready to merge",
		"CONFLICTING": "❌ Has conflicts",
		"UNKNOWN":     "⏳ Checking...",
	}
	
	// Note: Status formatting moved to FormatStatusState()
	// for reuse across dev-tools
)

// getStatusMessage is a local wrapper for FormatStatusState
func getStatusMessage(state string, withIcon bool) string {
	return FormatStatusState(state, withIcon)
}

func waitForReviewsAndChecks(cmd *cobra.Command, args []string) error {
	// Create GitHub client once for better performance (token caching)
	client := NewGitHubClient(owner, repo)
	
	prNumber, err := resolvePRNumberFromArgs(args, client)
	if err != nil {
		return err
	}

	// Convert to int for GraphQL
	prNumberInt, err := strconv.Atoi(prNumber)
	if err != nil {
		return fmt.Errorf("invalid PR number format: %w", err)
	}
	
	// Calculate timeout with Claude Code constraints
	effectiveTimeout, timeoutDisplay, err := calculateEffectiveTimeout()
	if err != nil {
		return err
	}
	
	// Show additional guidance for extending timeout if needed
	timeoutDuration, parseErr := parseTimeout()
	if parseErr == nil && effectiveTimeout < timeoutDuration {
		fmt.Printf("💡 To extend timeout, set BASH_MAX_TIMEOUT_MS in ~/.claude/settings.json\n")
		fmt.Printf("💡 Example: {\"env\": {\"BASH_MAX_TIMEOUT_MS\": \"900000\"}} for 15 minutes\n")
		fmt.Printf("💡 Manual retry: bin/gh-helper reviews wait %s --timeout=%v\n", prNumber, timeoutDuration)
	}
	
	// Request Gemini review if flag is set
	if requestReview {
		fmt.Printf("📝 Requesting Gemini review for PR #%s...\n", prNumber)
		if err := client.CreatePRComment(prNumber, "/gemini review"); err != nil {
			return fmt.Errorf("failed to request Gemini review: %w", err)
		}
		fmt.Println("✅ Gemini review requested")
	}
	
	fmt.Printf("🔄 Waiting for both reviews AND PR checks for PR #%s (timeout: %s)...\n", prNumber, timeoutDisplay)
	fmt.Println("Press Ctrl+C to stop monitoring")

	// Setup signal handling for graceful termination with proper guidance
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Exit code 130 is standard for SIGINT (Ctrl+C)
	go func() {
		sig := <-sigChan
		fmt.Printf("\n🛑 Received signal %v - terminating gracefully\n", sig)
		if effectiveTimeout < timeoutDuration {
			fmt.Printf("💡 Claude Code timeout interrupted. To continue, run:\n")
			fmt.Printf("    bin/gh-helper reviews wait %s --timeout=%v\n", prNumber, timeoutDuration)
		}
		os.Exit(130) // Standard exit code for SIGINT
	}()

	// Setup timeout (effectiveTimeout is already time.Duration)
	// timeoutDuration := timeoutDuration  // Already defined above
	startTime := time.Now()

	// Get initial state
	initialCheck := true
	reviewsReady := false
	checksComplete := false

	for {
		// Check timeout
		if time.Since(startTime) > effectiveTimeout {
			fmt.Printf("\n⏰ Timeout reached (%v).\n", effectiveTimeout)
			if reviewsReady && checksComplete {
				fmt.Println("✅ Both reviews and checks completed!")
				return nil
			} else {
				fmt.Printf("Status: Reviews ready: %v, Checks complete: %v\n", reviewsReady, checksComplete)
				if effectiveTimeout < timeoutDuration {
					fmt.Printf("💡 To continue waiting, run: bin/gh-helper reviews wait %s\n", prNumber)
				}
				return nil
			}
		}

		// Use unified architecture for reviews + status monitoring
		config := NewPRQueryConfig(owner, repo, prNumberInt).ForReviewsAndStatus()
		response, err := client.FetchPRData(config)
		if err != nil {
			fmt.Printf("Error fetching PR data: %v\n", err)
			time.Sleep(30 * time.Second)
			continue
		}

		// Check reviews status using common state tracking
		reviews := response.GetReviews()
		if len(reviews) > 0 && !reviewsReady {
			lastState, _ := loadReviewState(prNumber)
			reviewsReady = hasNewReviews(reviews, lastState)
		}

		// Check mergeable status first - if conflicting, stop immediately
		// CRITICAL INSIGHT: statusCheckRollup is null when PR has merge conflicts,
		// which prevents CI from running. This is GitHub's intentional behavior.
		// Must check mergeable before assuming "no checks required" scenario.
		mergeable, mergeStatus := response.GetMergeStatus()
		if mergeable == "CONFLICTING" {
			fmt.Printf("\n❌ [%s] PR has merge conflicts (status: %s)\n", time.Now().Format("15:04:05"), mergeStatus)
			fmt.Println("⚠️  CI checks will not run until conflicts are resolved")
			fmt.Printf("💡 Resolve conflicts with: git rebase origin/main\n")
			fmt.Printf("💡 Then push and run: bin/gh-helper reviews wait %s\n", prNumber)
			return fmt.Errorf("merge conflicts prevent CI execution")
		}

		// Check PR checks status
		statusCheckRollup := response.GetStatusCheckRollup()
		if statusCheckRollup != nil {
			rollupState := statusCheckRollup.State
			checksComplete = (rollupState == "SUCCESS" || rollupState == "FAILURE" || rollupState == "ERROR")
		} else {
			// StatusCheckRollup is nil - this can mean:
			// 1. No checks are configured for this repository (truly complete)
			// 2. Checks are configured but haven't started yet (not complete)
			// 3. PR was just created or pushed (checks pending)
			
			// Check merge status for better determination
			mergeable, mergeStatus := response.GetMergeStatus()
			
			// If PR has conflicts, checks won't run until resolved
			if mergeable == "CONFLICTING" {
				checksComplete = true // No point waiting for checks that won't run
			} else if mergeStatus == "CLEAN" || mergeStatus == "HAS_HOOKS" {
				// CLEAN: No checks configured, ready to merge
				// HAS_HOOKS: Only merge hooks configured, no status checks
				checksComplete = true
			} else {
				// PENDING, BLOCKED, DIRTY, UNKNOWN, etc. - checks may still be starting
				checksComplete = false
			}
		}

		if initialCheck {
			fmt.Printf("[%s] Monitoring started.\n", time.Now().Format("15:04:05"))
			fmt.Printf("   Reviews: %d found, Ready: %v\n", len(reviews), reviewsReady)
			
			// Show mergeable status
			mergeable, mergeStatus := response.GetMergeStatus()
			
			msg, exists := mergeStatusMessages[mergeable]
			if !exists {
				msg = mergeable // Use raw value for unknown states
			}
			
			if mergeable == "CONFLICTING" {
				fmt.Printf("   Merge: %s (status: %s)\n", msg, mergeStatus)
			} else {
				fmt.Printf("   Merge: %s\n", msg)
			}
			if statusCheckRollup != nil {
				rollupState := statusCheckRollup.State
				
				statusMsg := getStatusMessage(rollupState, false)
				fmt.Printf("   Checks: %s, Complete: %v\n", statusMsg, checksComplete)
			} else {
				fmt.Printf("   Checks: None required, Complete: %v\n", checksComplete)
			}
			initialCheck = false
		}

		// Check if both conditions are met
		if reviewsReady && checksComplete {
			fmt.Printf("\n🎉 [%s] Both reviews and checks are ready!\n", time.Now().Format("15:04:05"))
			
			if reviewsReady {
				fmt.Println("✅ Reviews: New reviews available")
				
				// Output review details to reduce subsequent API calls
				fmt.Println("\n📋 Recent Reviews:")
				for i, review := range reviews {
					if i >= 5 { // Limit to 5 most recent reviews
						break
					}
					fmt.Printf("   • %s by %s (%s) - %s\n", 
						review.ID, 
						review.Author.Login, 
						review.State,
						review.CreatedAt)
					if review.Body != "" && len(review.Body) > 100 {
						fmt.Printf("     Preview: %s...\n", review.Body[:100])
					} else if review.Body != "" {
						fmt.Printf("     Preview: %s\n", review.Body)
					}
				}
				
				fmt.Println()
				ListThreadsGuidance(prNumber).Print()
				fmt.Println("⚠️  IMPORTANT: Please read the review feedback carefully before proceeding")
			}
			
			// Show merge conflicts warning if present
			mergeable, _ := response.GetMergeStatus()
			if mergeable == "CONFLICTING" {
				fmt.Printf("\n⚠️  Merge conflicts detected - CI may not run until resolved\n")
				fmt.Printf("💡 Resolve conflicts and push to trigger CI checks\n")
			}
			if checksComplete {
				if statusCheckRollup != nil {
					rollupState := statusCheckRollup.State
					
					fmt.Printf("Checks: %s\n", getStatusMessage(rollupState, true))
				} else {
					fmt.Println("✅ Checks: No checks required")
				}
			}
			
			return nil
		}

		elapsed := time.Since(startTime)
		remaining := timeoutDuration - elapsed
		fmt.Printf("[%s] Status: Reviews: %v, Checks: %v (remaining: %v)\n",
			time.Now().Format("15:04:05"), reviewsReady, checksComplete, remaining.Truncate(time.Second))
		
		time.Sleep(30 * time.Second)
	}
}


func showThread(cmd *cobra.Command, args []string) error {
	// Create GitHub client once for better performance (token caching)
	client := NewGitHubClient(owner, repo)

	// Get output format using unified resolver
	format := ResolveFormat(cmd)
	
	// Get exclude-urls flag
	excludeURLs, err := cmd.Flags().GetBool("exclude-urls")
	if err != nil {
		return fmt.Errorf("failed to read 'exclude-urls' flag: %w", err)
	}

	// Use batch query for multiple threads or single thread
	threadsMap, err := client.GetThreadBatch(args, excludeURLs)
	if err != nil {
		return fmt.Errorf("failed to fetch threads: %w", err)
	}

	// Get current user for reply detection
	currentUser, _ := getCurrentUser()
	
	results := []map[string]interface{}{}
	
	// Process each thread ID in order
	for _, threadID := range args {
		thread, exists := threadsMap[threadID]
		if !exists {
			return fmt.Errorf("thread not found: %s", threadID)
		}

		// Build output structure using GitHub GraphQL API field names
		output := map[string]interface{}{
			"id":         thread.ID,
			"isResolved": thread.IsResolved,
			"path":       thread.Path,
			"line":       thread.Line,
		}
		
		// Include URL only if not empty (respects @skip directive)
		if thread.URL != "" {
			output["url"] = thread.URL
		}
		
		if thread.SubjectType != "" {
			output["subjectType"] = thread.SubjectType
		}
		
		// Comments using GitHub GraphQL structure
		comments := []map[string]interface{}{}
		for i, comment := range thread.Comments {
			commentData := map[string]interface{}{
				"id":        comment.ID,
				"author":    map[string]string{"login": comment.Author},
				"createdAt": comment.CreatedAt,
				"body":      comment.Body,
			}
			
			// Include URL only if not empty (respects @skip directive)
			if comment.URL != "" {
				commentData["url"] = comment.URL
			}
			
			if i == 0 && comment.DiffHunk != "" {
				commentData["diffHunk"] = comment.DiffHunk
			}
			
			comments = append(comments, commentData)
		}
		
		output["comments"] = map[string]interface{}{
			"nodes":      comments,
			"totalCount": len(comments),
		}
		
		// Check if needs reply
		if !thread.IsResolved && len(thread.Comments) > 0 {
			lastComment := thread.Comments[len(thread.Comments)-1]
			if currentUser != "" && lastComment.Author != currentUser {
				output["needsReply"] = true
				output["lastCommentBy"] = lastComment.Author
			}
		}
		
		results = append(results, output)
	}
	
	// Output single result for backward compatibility when only one thread
	if len(results) == 1 {
		return EncodeOutput(os.Stdout, format, results[0])
	}
	
	// Output array for multiple threads
	return EncodeOutput(os.Stdout, format, results)
}

func resolveThread(cmd *cobra.Command, args []string) error {
	// Create GitHub client
	client := NewGitHubClient(owner, repo)
	
	// Get output format using unified resolver
	format := ResolveFormat(cmd)
	
	resolvedAt := time.Now().Format("2006-01-02T15:04:05Z07:00")
	results := []map[string]interface{}{}
	
	// Process each thread ID
	for _, threadID := range args {
		if err := client.ResolveThread(threadID); err != nil {
			return fmt.Errorf("failed to resolve thread %s: %w", threadID, err)
		}

		// Collect result for this thread
		results = append(results, map[string]interface{}{
			"id":         threadID,
			"isResolved": true,
			"resolvedAt": resolvedAt,
		})
	}
	
	// Output single result for backward compatibility when only one thread
	if len(results) == 1 {
		return EncodeOutput(os.Stdout, format, results[0])
	}
	
	// Output array for multiple threads
	return EncodeOutput(os.Stdout, format, results)
}

// threadInput represents a thread ID with optional custom message
type threadInput struct {
	ID            string
	CustomMessage string
}

// replyResult represents the result of a thread reply operation
type replyResult struct {
	ThreadID  string `json:"threadId"`
	Status    string `json:"status"`
	CommentID string `json:"commentId,omitempty"`
	URL       string `json:"url,omitempty"`
	Message   string `json:"message"`
	Resolved  bool   `json:"resolved,omitempty"`
	Error     string `json:"error,omitempty"`
}

func replyToThread(cmd *cobra.Command, args []string) error {
	// Create GitHub client once for better performance (token caching)
	client := NewGitHubClient(owner, repo)

	// Parse thread IDs and custom messages
	var threadInputs []threadInput
	for _, arg := range args {
		parts := strings.SplitN(arg, ":", 2)
		input := threadInput{ID: parts[0]}
		if len(parts) == 2 {
			// Custom message provided
			input.CustomMessage = parts[1]
		}
		threadInputs = append(threadInputs, input)
	}

	// Get default message from flag or stdin
	var defaultMessage string
	if message != "" {
		defaultMessage = message
	} else if len(threadInputs) == 1 || hasCustomMessages(threadInputs) {
		// Read from stdin if single thread or some threads have custom messages
		stdinBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		defaultMessage = strings.TrimSpace(string(stdinBytes))
	}

	// Validate that all threads have messages
	for _, input := range threadInputs {
		if input.CustomMessage == "" && defaultMessage == "" {
			// If no message but commit hash is provided, use default message
			if commitHash != "" {
				defaultMessage = "Thank you for the feedback!"
			} else {
				return fmt.Errorf("no message provided for thread %s (use --message, custom message, or pipe content to stdin)", input.ID)
			}
		}
	}

	// Get output format using unified resolver
	format := ResolveFormat(cmd)

	// Get parallel execution flags
	parallel, _ := cmd.Flags().GetBool("parallel")
	maxConcurrent, _ := cmd.Flags().GetInt("max-concurrent")

	// Execute replies in parallel
	results := ExecuteParallel(
		threadInputs,
		func(input threadInput) (replyResult, error) {
			result := replyResult{
				ThreadID: input.ID,
				Status:   "success",
			}

			// Determine message to use
			replyText := input.CustomMessage
			if replyText == "" {
				replyText = defaultMessage
			}
			result.Message = replyText

			// Add commit reference if provided
			if commitHash != "" {
				replyText = fmt.Sprintf("%s\n\nFixed in commit %s.", strings.TrimSpace(replyText), commitHash)
			}

			// Add mention if provided
			if mentionUser != "" {
				replyText = fmt.Sprintf("@%s %s", mentionUser, replyText)
			}

			// Template variable expansion
			replyText = strings.ReplaceAll(replyText, "{commit}", commitHash)

			// Execute mutation
			err := executeReplyMutation(client, input.ID, replyText, &result)
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				return result, nil
			}

			// Auto-resolve thread if requested
			if autoResolve {
				if err := client.ResolveThread(input.ID); err != nil {
					// Don't fail - reply succeeded, resolution failed
					result.Resolved = false
				} else {
					result.Resolved = true
				}
			}

			return result, nil
		},
		parallel,
		maxConcurrent,
	)

	// Single thread backward compatibility
	if len(results) == 1 {
		result := results[0]
		if result.Status == "failed" {
			return fmt.Errorf("failed to reply to thread: %s", result.Error)
		}
		
		outputData := map[string]interface{}{
			"threadId":  result.ThreadID,
			"commentId": result.CommentID,
			"url":       result.URL,
			"repliedAt": time.Now().Format("2006-01-02T15:04:05Z07:00"),
		}
		if autoResolve {
			outputData["isResolved"] = result.Resolved
		}
		return EncodeOutput(os.Stdout, format, outputData)
	}

	// Multiple threads - output bulk results
	summary := map[string]interface{}{
		"bulkReplyResults": results,
		"summary": map[string]interface{}{
			"total":      len(results),
			"successful": countSuccessful(results),
			"failed":     countFailed(results),
			"resolved":   countResolved(results),
		},
	}

	return EncodeOutput(os.Stdout, format, summary)
}

// Helper function to check if any thread has custom messages
func hasCustomMessages(inputs []threadInput) bool {
	for _, input := range inputs {
		if input.CustomMessage != "" {
			return true
		}
	}
	return false
}

// Helper function to execute reply mutation
func executeReplyMutation(client *GitHubClient, threadID, body string, result *replyResult) error {
	mutation := `
mutation($threadID: ID!, $body: String!) {
  addPullRequestReviewThreadReply(input: {
    pullRequestReviewThreadId: $threadID
    body: $body
  }) {
    comment {
      id
      url
      body
    }
  }
}`

	variables := map[string]interface{}{
		"threadID": threadID,
		"body":     body,
	}

	responseData, err := client.RunGraphQLQueryWithVariables(mutation, variables)
	if err != nil {
		return err
	}

	var response struct {
		Data struct {
			AddPullRequestReviewThreadReply struct {
				Comment struct {
					ID  string `json:"id"`
					URL string `json:"url"`
					Body string `json:"body"`
				} `json:"comment"`
			} `json:"addPullRequestReviewThreadReply"`
		} `json:"data"`
	}

	if err := Unmarshal(responseData, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	comment := response.Data.AddPullRequestReviewThreadReply.Comment
	if comment.ID == "" {
		return fmt.Errorf("reply posting failed: empty response")
	}

	result.CommentID = comment.ID
	result.URL = comment.URL
	
	return nil
}

// Helper functions for counting results
func countSuccessful(results []replyResult) int {
	count := 0
	for _, r := range results {
		if r.Status == "success" {
			count++
		}
	}
	return count
}

func countFailed(results []replyResult) int {
	count := 0
	for _, r := range results {
		if r.Status == "failed" {
			count++
		}
	}
	return count
}

func countResolved(results []replyResult) int {
	count := 0
	for _, r := range results {
		if r.Resolved {
			count++
		}
	}
	return count
}

