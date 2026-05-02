package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	copilotReviewBotLogin = "copilot-pull-request-reviewer[bot]"
	geminiReviewBotLogin  = "gemini-code-assist[bot]"
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
	ThreadsTruncated   bool              `json:"threadsTruncated"`
	Bots               []ReviewBotStatus `json:"bots"`
}

type ReviewBotStatus struct {
	Bot                          string             `json:"bot"`
	Login                        string             `json:"login"`
	ReviewedCurrentHead          bool               `json:"reviewedCurrentHead"`
	Ready                        bool               `json:"ready"`
	PositiveSignal               string             `json:"positiveSignal,omitempty"`
	LatestReviewID               string             `json:"latestReviewId,omitempty"`
	LatestReviewState            string             `json:"latestReviewState,omitempty"`
	LatestReviewCommitOID        string             `json:"latestReviewCommitOid,omitempty"`
	LatestReviewCreatedAt        string             `json:"latestReviewCreatedAt,omitempty"`
	LatestCurrentHeadReviewID    string             `json:"latestCurrentHeadReviewId,omitempty"`
	LatestCurrentHeadReviewState string             `json:"latestCurrentHeadReviewState,omitempty"`
	UnresolvedCurrentHeadThreads []BotThreadSummary `json:"unresolvedCurrentHeadThreads"`
	NextAction                   string             `json:"nextAction"`
}

type BotThreadSummary struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Line        *int   `json:"line"`
	IsOutdated  bool   `json:"isOutdated"`
	Author      string `json:"author"`
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
	Comments   []botThreadComment
}

type botThreadComment struct {
	Author    string
	Body      string
	CreatedAt string
	State     string
	CommitOID string
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

	query := `
query($owner: String!, $repo: String!, $prNumber: Int!) {
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
      reviews(last: 100) {
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
      reviewThreads(first: 100) {
        nodes {
          id
          path
          line
          isResolved
          isOutdated
          comments(first: 100) {
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
        }
      }
    }
  }
}`

	variables := map[string]interface{}{
		"owner":    c.Owner,
		"repo":     c.Repo,
		"prNumber": prNumberInt,
	}

	result, err := c.RunGraphQLQueryWithVariables(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bot review status: %w", err)
	}

	var response struct {
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
					ReviewThreads struct {
						PageInfo struct {
							HasNextPage bool `json:"hasNextPage"`
						} `json:"pageInfo"`
						Nodes []struct {
							ID         string `json:"id"`
							Path       string `json:"path"`
							Line       *int   `json:"line"`
							IsResolved bool   `json:"isResolved"`
							IsOutdated bool   `json:"isOutdated"`
							Comments   struct {
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

	if err := Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse bot review status: %w", err)
	}

	pr := response.Data.Repository.PullRequest
	prHeadOID := ""
	if len(pr.Commits.Nodes) > 0 {
		prHeadOID = pr.Commits.Nodes[0].Commit.OID
	}

	reviews := make([]botReviewNode, 0, len(pr.Reviews.Nodes))
	for _, review := range pr.Reviews.Nodes {
		reviews = append(reviews, botReviewNode{
			ID:        review.ID,
			Author:    review.Author.Login,
			State:     review.State,
			Body:      review.Body,
			CreatedAt: review.CreatedAt,
			CommitOID: review.Commit.OID,
		})
	}

	threads := make([]botThreadNode, 0, len(pr.ReviewThreads.Nodes))
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
		threads = append(threads, botThreadNode{
			ID:         thread.ID,
			Path:       thread.Path,
			Line:       thread.Line,
			IsResolved: thread.IsResolved,
			IsOutdated: thread.IsOutdated,
			Comments:   comments,
		})
	}

	localHeadOID := localGitHeadOID()
	report := buildBotReviewReport(pr.Number, pr.Title, prHeadOID, localHeadOID, reviews, threads)
	report.ThreadsTruncated = pr.ReviewThreads.PageInfo.HasNextPage
	return report, nil
}

func buildBotReviewReport(
	prNumber int,
	title string,
	prHeadOID string,
	localHeadOID string,
	reviews []botReviewNode,
	threads []botThreadNode,
) *BotReviewReport {
	bots := []ReviewBotStatus{
		buildReviewBotStatus("copilot", copilotReviewBotLogin, prHeadOID, reviews, threads),
		buildReviewBotStatus("gemini", geminiReviewBotLogin, prHeadOID, reviews, threads),
	}

	return &BotReviewReport{
		PR:                 prNumber,
		Title:              title,
		PRHeadOID:          prHeadOID,
		LocalHeadOID:       localHeadOID,
		LocalHeadMatchesPR: localHeadOID != "" && prHeadOID != "" && localHeadOID == prHeadOID,
		Bots:               bots,
	}
}

func buildReviewBotStatus(
	bot string,
	login string,
	headOID string,
	reviews []botReviewNode,
	threads []botThreadNode,
) ReviewBotStatus {
	var latest *botReviewNode
	var latestCurrentHead *botReviewNode
	for i := range reviews {
		review := &reviews[i]
		if review.Author != login || !isRelevantBotReview(login, review.Body) {
			continue
		}
		if latest == nil || review.CreatedAt > latest.CreatedAt {
			latest = review
		}
		if review.CommitOID == headOID && (latestCurrentHead == nil || review.CreatedAt > latestCurrentHead.CreatedAt) {
			latestCurrentHead = review
		}
	}

	unresolvedThreads := unresolvedBotThreads(login, headOID, threads)
	status := ReviewBotStatus{
		Bot:                          bot,
		Login:                        login,
		ReviewedCurrentHead:          latestCurrentHead != nil,
		UnresolvedCurrentHeadThreads: unresolvedThreads,
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

	status.Ready = botReady(login, status.ReviewedCurrentHead, status.PositiveSignal, unresolvedThreads)
	status.NextAction = botNextAction(bot, status)
	return status
}

func botReady(login string, reviewedCurrentHead bool, positiveSignal string, unresolvedThreads []BotThreadSummary) bool {
	if !reviewedCurrentHead || len(unresolvedThreads) > 0 {
		return false
	}
	if login == geminiReviewBotLogin {
		return positiveSignal != ""
	}
	return positiveSignal != "" || len(unresolvedThreads) == 0
}

func unresolvedBotThreads(login string, headOID string, threads []botThreadNode) []BotThreadSummary {
	summaries := []BotThreadSummary{}
	for _, thread := range threads {
		if thread.IsResolved {
			continue
		}

		var botComment *botThreadComment
		for i := range thread.Comments {
			comment := &thread.Comments[i]
			if comment.Author == login && comment.CommitOID == headOID {
				botComment = comment
			}
		}
		if botComment == nil {
			continue
		}

		summaries = append(summaries, BotThreadSummary{
			ID:          thread.ID,
			Path:        thread.Path,
			Line:        thread.Line,
			IsOutdated:  thread.IsOutdated,
			Author:      botComment.Author,
			CreatedAt:   botComment.CreatedAt,
			BodyPreview: truncateForStatus(botComment.Body, 160),
		})
	}
	return summaries
}

func isRelevantBotReview(login string, body string) bool {
	if login != geminiReviewBotLogin {
		return true
	}
	if strings.Contains(body, geminiReviewHeader) {
		return true
	}
	return strings.Contains(body, "I have no feedback to provide.")
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
