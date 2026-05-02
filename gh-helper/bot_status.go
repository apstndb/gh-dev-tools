package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	copilotReviewBotLogin = "copilot-pull-request-reviewer"
	geminiReviewBotLogin  = "gemini-code-assist"
	maxBotReviewPages     = 20
	maxBotThreadPages     = 20
)

var botStatusCmd = &cobra.Command{
	Use:   "bot-status [pr-number]",
	Short: "Show Copilot and Gemini review status for the current PR head",
	Long: `Show whether Copilot and Gemini have reviewed the current PR head.

This command is designed for review-loop automation. It reports the PR head
commit, the local HEAD commit when available, each bot's latest current-head
review, and unresolved current-head threads authored by that bot.

` + prNumberArgsHelp + `

Examples:
  gh-helper reviews bot-status 306
  gh-helper reviews bot-status        # Auto-detect current branch PR
  gh-helper reviews bot-status 306 --json`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         showBotStatus,
}

type BotReviewReport struct {
	PR                 int               `json:"pr"`
	Title              string            `json:"title"`
	PRHeadOID          string            `json:"prHeadOid"`
	LocalHeadOID       string            `json:"localHeadOid,omitempty"`
	LocalHeadMatchesPR bool              `json:"localHeadMatchesPr"`
	ReviewsTruncated   bool              `json:"reviewsTruncated"`
	ThreadsTruncated   bool              `json:"threadsTruncated"`
	Bots               []ReviewBotStatus `json:"bots"`
}

type ReviewBotStatus struct {
	Bot                          string             `json:"bot"`
	Login                        string             `json:"login"`
	ReviewedCurrentHead          bool               `json:"reviewedCurrentHead"`
	Ready                        bool               `json:"ready"`
	ReadinessPolicy              string             `json:"readinessPolicy"`
	PositiveSignal               string             `json:"positiveSignal,omitempty"`
	LatestReviewID               string             `json:"latestReviewId,omitempty"`
	LatestReviewState            string             `json:"latestReviewState,omitempty"`
	LatestReviewCommitOID        string             `json:"latestReviewCommitOid,omitempty"`
	LatestReviewCreatedAt        string             `json:"latestReviewCreatedAt,omitempty"`
	LatestCurrentHeadReviewID    string             `json:"latestCurrentHeadReviewId,omitempty"`
	LatestCurrentHeadReviewState string             `json:"latestCurrentHeadReviewState,omitempty"`
	UnresolvedCurrentHeadThreads []BotThreadSummary `json:"unresolvedCurrentHeadThreads"`
	UnresolvedOtherThreads       []BotThreadSummary `json:"unresolvedOtherThreads"`
	ReviewDataIncomplete         bool               `json:"reviewDataIncomplete"`
	NextAction                   string             `json:"nextAction"`
}

type BotThreadSummary struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Line        *int   `json:"line"`
	IsOutdated  bool   `json:"isOutdated"`
	Author      string `json:"author"`
	CommitOID   string `json:"commitOid,omitempty"`
	CreatedAt   string `json:"createdAt"`
	BodyPreview string `json:"bodyPreview"`
}

type botReviewNode struct {
	ID        string
	Author    string
	State     string
	Body      string
	CreatedAt string
	CommitOID string
}

type botThreadNode struct {
	ID         string
	Path       string
	Line       *int
	IsResolved bool
	IsOutdated bool
	Truncated  bool
	Comments   []botThreadComment
}

type botThreadComment struct {
	Author    string
	Body      string
	CreatedAt string
	State     string
	CommitOID string
}

type botStatusCompleteness struct {
	ReviewsTruncated bool
	ThreadsTruncated bool
}

type botReviewMetadata struct {
	Number           int
	Title            string
	PRHeadOID        string
	Reviews          []botReviewNode
	ReviewsTruncated bool
}

type botReviewMetadataResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				Number  int    `json:"number"`
				Title   string `json:"title"`
				Commits struct {
					Nodes []struct {
						Commit struct {
							OID string `json:"oid"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"commits"`
				Reviews struct {
					PageInfo struct {
						HasPreviousPage bool   `json:"hasPreviousPage"`
						StartCursor     string `json:"startCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						ID     string `json:"id"`
						Author struct {
							Login string `json:"login"`
						} `json:"author"`
						State     string `json:"state"`
						Body      string `json:"body"`
						CreatedAt string `json:"createdAt"`
						Commit    struct {
							OID string `json:"oid"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"reviews"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

type botReviewThreadsResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				ReviewThreads struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						ID         string `json:"id"`
						Path       string `json:"path"`
						Line       *int   `json:"line"`
						IsResolved bool   `json:"isResolved"`
						IsOutdated bool   `json:"isOutdated"`
						Comments   struct {
							PageInfo struct {
								HasNextPage bool `json:"hasNextPage"`
							} `json:"pageInfo"`
							Nodes []struct {
								Body      string `json:"body"`
								CreatedAt string `json:"createdAt"`
								State     string `json:"state"`
								Author    struct {
									Login string `json:"login"`
								} `json:"author"`
								PullRequestReview struct {
									Commit struct {
										OID string `json:"oid"`
									} `json:"commit"`
								} `json:"pullRequestReview"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

func showBotStatus(cmd *cobra.Command, args []string) error {
	client := NewGitHubClient(owner, repo)
	prNumber, err := resolvePRNumberFromArgs(args, client)
	if err != nil {
		return err
	}

	report, err := client.GetBotReviewReport(prNumber)
	if err != nil {
		return err
	}

	return EncodeOutputWithCmd(cmd, map[string]interface{}{
		"botStatus": report,
	})
}

func (c *GitHubClient) GetBotReviewReport(prNumber string) (*BotReviewReport, error) {
	prNumberInt, err := strconv.Atoi(prNumber)
	if err != nil {
		return nil, fmt.Errorf("invalid PR number format: %w", err)
	}

	metadata, err := c.fetchBotReviewMetadata(prNumberInt)
	if err != nil {
		return nil, err
	}
	threads, threadsTruncated, err := c.fetchBotReviewThreads(prNumberInt)
	if err != nil {
		return nil, err
	}

	localHeadOID := localGitHeadOID()
	return buildBotReviewReport(
		metadata.Number,
		metadata.Title,
		metadata.PRHeadOID,
		localHeadOID,
		metadata.Reviews,
		threads,
		botStatusCompleteness{
			ReviewsTruncated: metadata.ReviewsTruncated,
			ThreadsTruncated: threadsTruncated,
		},
	), nil
}

func (c *GitHubClient) fetchBotReviewMetadata(prNumberInt int) (*botReviewMetadata, error) {
	query := `
query($owner: String!, $repo: String!, $prNumber: Int!, $reviewBefore: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $prNumber) {
      number
      title
      commits(last: 1) {
        nodes {
          commit {
            oid
          }
        }
      }
      reviews(last: 100, before: $reviewBefore) {
        pageInfo {
          hasPreviousPage
          startCursor
        }
        nodes {
          id
          author { login }
          state
          body
          createdAt
          commit {
            oid
          }
        }
      }
    }
  }
}`

	metadata := &botReviewMetadata{}
	reviewBefore := ""
	for page := 0; ; page++ {
		variables := map[string]interface{}{
			"owner":        c.Owner,
			"repo":         c.Repo,
			"prNumber":     prNumberInt,
			"reviewBefore": nil,
		}
		if reviewBefore != "" {
			variables["reviewBefore"] = reviewBefore
		}

		result, err := c.RunGraphQLQueryWithVariables(query, variables)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch bot review metadata: %w", err)
		}

		var response botReviewMetadataResponse
		if err := Unmarshal(result, &response); err != nil {
			return nil, fmt.Errorf("failed to parse bot review metadata: %w", err)
		}

		pr := response.Data.Repository.PullRequest
		if metadata.Number == 0 {
			metadata.Number = pr.Number
			metadata.Title = pr.Title
			if len(pr.Commits.Nodes) > 0 {
				metadata.PRHeadOID = pr.Commits.Nodes[0].Commit.OID
			}
		}
		for _, review := range pr.Reviews.Nodes {
			metadata.Reviews = append(metadata.Reviews, botReviewNode{
				ID:        review.ID,
				Author:    review.Author.Login,
				State:     review.State,
				Body:      review.Body,
				CreatedAt: review.CreatedAt,
				CommitOID: review.Commit.OID,
			})
		}

		pageInfo := pr.Reviews.PageInfo
		if !pageInfo.HasPreviousPage {
			break
		}
		if pageInfo.StartCursor == "" {
			metadata.ReviewsTruncated = true
			break
		}
		if page+1 >= maxBotReviewPages {
			metadata.ReviewsTruncated = true
			break
		}
		reviewBefore = pageInfo.StartCursor
	}

	return metadata, nil
}

func (c *GitHubClient) fetchBotReviewThreads(prNumberInt int) ([]botThreadNode, bool, error) {
	query := `
query($owner: String!, $repo: String!, $prNumber: Int!, $threadAfter: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $prNumber) {
      reviewThreads(first: 100, after: $threadAfter) {
        nodes {
          id
          path
          line
          isResolved
          isOutdated
          comments(first: 100) {
            pageInfo {
              hasNextPage
            }
            nodes {
              body
              createdAt
              state
              author { login }
              pullRequestReview {
                commit {
                  oid
                }
              }
            }
          }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
  }
}`

	threads := []botThreadNode{}
	threadsTruncated := false
	threadAfter := ""
	for page := 0; ; page++ {
		variables := map[string]interface{}{
			"owner":       c.Owner,
			"repo":        c.Repo,
			"prNumber":    prNumberInt,
			"threadAfter": nil,
		}
		if threadAfter != "" {
			variables["threadAfter"] = threadAfter
		}

		result, err := c.RunGraphQLQueryWithVariables(query, variables)
		if err != nil {
			return nil, false, fmt.Errorf("failed to fetch bot review threads: %w", err)
		}

		var pageResponse botReviewThreadsResponse
		if err := Unmarshal(result, &pageResponse); err != nil {
			return nil, false, fmt.Errorf("failed to parse bot review threads: %w", err)
		}

		pageThreads, pageTruncated := botThreadNodesFromResponse(pageResponse)
		threads = append(threads, pageThreads...)
		threadsTruncated = threadsTruncated || pageTruncated

		pageInfo := pageResponse.Data.Repository.PullRequest.ReviewThreads.PageInfo
		if !pageInfo.HasNextPage {
			break
		}
		if pageInfo.EndCursor == "" {
			threadsTruncated = true
			break
		}
		if page+1 >= maxBotThreadPages {
			threadsTruncated = true
			break
		}
		threadAfter = pageInfo.EndCursor
	}

	return threads, threadsTruncated, nil
}

func botThreadNodesFromResponse(response botReviewThreadsResponse) ([]botThreadNode, bool) {
	pr := response.Data.Repository.PullRequest
	threads := make([]botThreadNode, 0, len(pr.ReviewThreads.Nodes))
	threadsTruncated := false
	for _, thread := range pr.ReviewThreads.Nodes {
		comments := make([]botThreadComment, 0, len(thread.Comments.Nodes))
		for _, comment := range thread.Comments.Nodes {
			comments = append(comments, botThreadComment{
				Author:    comment.Author.Login,
				Body:      comment.Body,
				CreatedAt: comment.CreatedAt,
				State:     comment.State,
				CommitOID: comment.PullRequestReview.Commit.OID,
			})
		}
		commentsTruncated := thread.Comments.PageInfo.HasNextPage
		threadsTruncated = threadsTruncated || commentsTruncated
		threads = append(threads, botThreadNode{
			ID:         thread.ID,
			Path:       thread.Path,
			Line:       thread.Line,
			IsResolved: thread.IsResolved,
			IsOutdated: thread.IsOutdated,
			Truncated:  commentsTruncated,
			Comments:   comments,
		})
	}
	return threads, threadsTruncated
}

func buildBotReviewReport(
	prNumber int,
	title string,
	prHeadOID string,
	localHeadOID string,
	reviews []botReviewNode,
	threads []botThreadNode,
	completeness botStatusCompleteness,
) *BotReviewReport {
	bots := []ReviewBotStatus{
		buildReviewBotStatus(
			"copilot",
			copilotReviewBotLogin,
			prHeadOID,
			reviews,
			threads,
			completeness,
		),
		buildReviewBotStatus(
			"gemini",
			geminiReviewBotLogin,
			prHeadOID,
			reviews,
			threads,
			completeness,
		),
	}

	return &BotReviewReport{
		PR:                 prNumber,
		Title:              title,
		PRHeadOID:          prHeadOID,
		LocalHeadOID:       localHeadOID,
		LocalHeadMatchesPR: localHeadOID != "" && prHeadOID != "" && localHeadOID == prHeadOID,
		ReviewsTruncated:   completeness.ReviewsTruncated,
		ThreadsTruncated:   completeness.ThreadsTruncated,
		Bots:               bots,
	}
}

func buildReviewBotStatus(
	bot string,
	login string,
	headOID string,
	reviews []botReviewNode,
	threads []botThreadNode,
	completeness botStatusCompleteness,
) ReviewBotStatus {
	var latest *botReviewNode
	var latestCurrentHead *botReviewNode
	for i := range reviews {
		review := &reviews[i]
		if !botLoginMatches(review.Author, login) || !isRelevantBotReview(login, review.Body) {
			continue
		}
		if latest == nil || review.CreatedAt > latest.CreatedAt {
			latest = review
		}
		if review.CommitOID == headOID && (latestCurrentHead == nil || review.CreatedAt > latestCurrentHead.CreatedAt) {
			latestCurrentHead = review
		}
	}

	currentHeadThreads, otherThreads := unresolvedBotThreads(login, headOID, threads)
	status := ReviewBotStatus{
		Bot:                          bot,
		Login:                        login,
		ReviewedCurrentHead:          latestCurrentHead != nil,
		ReadinessPolicy:              readinessPolicy(login),
		UnresolvedCurrentHeadThreads: currentHeadThreads,
		UnresolvedOtherThreads:       otherThreads,
		ReviewDataIncomplete:         completeness.ThreadsTruncated || (completeness.ReviewsTruncated && latestCurrentHead == nil),
	}

	if latest != nil {
		status.LatestReviewID = latest.ID
		status.LatestReviewState = latest.State
		status.LatestReviewCommitOID = latest.CommitOID
		status.LatestReviewCreatedAt = latest.CreatedAt
	}
	if latestCurrentHead != nil {
		status.LatestCurrentHeadReviewID = latestCurrentHead.ID
		status.LatestCurrentHeadReviewState = latestCurrentHead.State
		status.PositiveSignal = positiveBotReviewSignal(login, latestCurrentHead.Body)
	}

	status.Ready = botReady(
		login,
		status.ReviewedCurrentHead,
		status.PositiveSignal,
		currentHeadThreads,
		otherThreads,
		status.ReviewDataIncomplete,
	)
	status.NextAction = botNextAction(bot, status)
	return status
}

func readinessPolicy(login string) string {
	if login == geminiReviewBotLogin {
		return "Gemini is ready only after a current-head no-feedback review and no unresolved Gemini threads."
	}
	return "Copilot is ready after any current-head review and no unresolved Copilot threads."
}

func botReady(
	login string,
	reviewedCurrentHead bool,
	positiveSignal string,
	currentHeadThreads []BotThreadSummary,
	otherThreads []BotThreadSummary,
	reviewDataIncomplete bool,
) bool {
	if !reviewedCurrentHead || len(currentHeadThreads) > 0 || len(otherThreads) > 0 || reviewDataIncomplete {
		return false
	}
	if login == geminiReviewBotLogin {
		return positiveSignal != ""
	}
	return true
}

func unresolvedBotThreads(login string, headOID string, threads []botThreadNode) ([]BotThreadSummary, []BotThreadSummary) {
	currentHeadSummaries := []BotThreadSummary{}
	otherSummaries := []BotThreadSummary{}
	for _, thread := range threads {
		if thread.IsResolved {
			continue
		}

		var botComment *botThreadComment
		for i := range thread.Comments {
			comment := &thread.Comments[i]
			if botLoginMatches(comment.Author, login) {
				botComment = comment
			}
		}
		if botComment == nil {
			continue
		}

		summary := BotThreadSummary{
			ID:          thread.ID,
			Path:        thread.Path,
			Line:        thread.Line,
			IsOutdated:  thread.IsOutdated,
			Author:      botComment.Author,
			CommitOID:   botComment.CommitOID,
			CreatedAt:   botComment.CreatedAt,
			BodyPreview: truncateForStatus(botComment.Body, 160),
		}
		if botComment.CommitOID == headOID {
			currentHeadSummaries = append(currentHeadSummaries, summary)
			continue
		}
		otherSummaries = append(otherSummaries, summary)
	}
	return currentHeadSummaries, otherSummaries
}

func isRelevantBotReview(login string, body string) bool {
	if login != geminiReviewBotLogin {
		return true
	}
	return !strings.Contains(body, geminiSummaryHeader) ||
		strings.Contains(body, geminiReviewHeader) ||
		strings.Contains(body, "I have no feedback to provide.")
}

func positiveBotReviewSignal(login string, body string) string {
	switch login {
	case copilotReviewBotLogin:
		lowerBody := strings.ToLower(body)
		if strings.Contains(lowerBody, "generated no new comments") {
			return "generated no new comments"
		}
	case geminiReviewBotLogin:
		if strings.Contains(body, "I have no feedback to provide.") {
			return "I have no feedback to provide."
		}
	}
	return ""
}

func botNextAction(bot string, status ReviewBotStatus) string {
	if len(status.UnresolvedCurrentHeadThreads) > 0 {
		return "address, reply to, and resolve the unresolved current-head threads"
	}
	if len(status.UnresolvedOtherThreads) > 0 {
		return "address, reply to, and resolve unresolved threads from earlier commits"
	}
	if status.ReviewDataIncomplete {
		return "review thread/comment data is incomplete because pagination limits left some data truncated"
	}
	if status.Ready {
		return "no action needed for the current PR head"
	}
	if status.ReviewedCurrentHead {
		return "inspect the review body before deciding whether to request another review"
	}
	if bot == "gemini" {
		return "request Gemini review for the current PR head"
	}
	return "request Copilot review for the current PR head"
}

func botLoginMatches(actual string, expected string) bool {
	return normalizeBotLogin(actual) == normalizeBotLogin(expected)
}

func normalizeBotLogin(login string) string {
	return strings.TrimSuffix(login, "[bot]")
}

func requestGeminiReviewForCurrentHead(client *GitHubClient, prNumber string) error {
	report, err := client.GetBotReviewReport(prNumber)
	if err != nil {
		return err
	}

	for _, bot := range report.Bots {
		if bot.Login != geminiReviewBotLogin {
			continue
		}
		if bot.ReviewedCurrentHead {
			fmt.Printf("✅ Gemini already reviewed PR head %s; skipping duplicate request\n", report.PRHeadOID)
			return nil
		}
		if bot.ReviewDataIncomplete {
			return fmt.Errorf("cannot safely request Gemini review because bot review data is incomplete; inspect bot-status output or request /gemini review manually")
		}
	}

	if report.ReviewsTruncated {
		return fmt.Errorf("cannot safely request Gemini review because review history is incomplete; inspect bot-status output or request /gemini review manually")
	}

	if report.LocalHeadOID != "" && report.PRHeadOID != "" && !report.LocalHeadMatchesPR {
		fmt.Printf(
			"⚠️  Local HEAD %s differs from PR head %s; requesting review for the pushed PR head\n",
			report.LocalHeadOID,
			report.PRHeadOID,
		)
	}

	fmt.Printf("📝 Requesting Gemini review for PR #%s...\n", prNumber)
	if err := client.CreatePRComment(prNumber, "/gemini review"); err != nil {
		return fmt.Errorf("failed to request Gemini review: %w", err)
	}
	fmt.Println("✅ Gemini review requested")
	return nil
}

func localGitHeadOID() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func truncateForStatus(body string, maxLen int) string {
	body = strings.TrimSpace(body)
	body = strings.Join(strings.Fields(body), " ")
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "..."
}
