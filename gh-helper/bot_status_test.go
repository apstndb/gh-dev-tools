package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildReviewBotStatusGeminiIgnoresSummaryReview(t *testing.T) {
	t.Parallel()

	reviews := []botReviewNode{
		{
			ID:        "summary",
			Author:    geminiReviewBotLogin,
			State:     "COMMENTED",
			Body:      geminiSummaryHeader + "\n\nSummary only.",
			CreatedAt: "2026-05-03T01:00:00Z",
			CommitOID: "head",
		},
		{
			ID:        "code-review",
			Author:    geminiReviewBotLogin,
			State:     "COMMENTED",
			Body:      "I have no feedback to provide.",
			CreatedAt: "2026-05-03T00:59:00Z",
			CommitOID: "head",
		},
	}

	status := buildReviewBotStatus("gemini", geminiReviewBotLogin, "head", reviews, nil, botStatusCompleteness{})
	if !status.ReviewedCurrentHead {
		t.Fatal("Gemini code review was not recognized for current head")
	}
	if !status.Ready {
		t.Fatal("Gemini positive review should be ready")
	}
	if status.LatestCurrentHeadReviewID != "code-review" {
		t.Fatalf("LatestCurrentHeadReviewID = %q, want code-review", status.LatestCurrentHeadReviewID)
	}
}

func TestBuildReviewBotStatusGeminiRequiresPositiveSignal(t *testing.T) {
	t.Parallel()

	reviews := []botReviewNode{
		{
			ID:        "needs-inspection",
			Author:    geminiReviewBotLogin,
			State:     "COMMENTED",
			Body:      geminiReviewHeader + "\n\nPlease inspect this.",
			CreatedAt: "2026-05-03T01:00:00Z",
			CommitOID: "head",
		},
	}

	status := buildReviewBotStatus("gemini", geminiReviewBotLogin, "head", reviews, nil, botStatusCompleteness{})
	if !status.ReviewedCurrentHead {
		t.Fatal("Gemini review was not recognized for current head")
	}
	if status.Ready {
		t.Fatal("Gemini review without the no-feedback signal should not be ready")
	}
	if status.ReadinessPolicy == "" {
		t.Fatal("ReadinessPolicy is empty")
	}
}

func TestBuildReviewBotStatusCurrentHeadThreads(t *testing.T) {
	t.Parallel()

	line := 42
	threads := []botThreadNode{
		{
			ID:         "current-thread",
			Path:       "main.go",
			Line:       &line,
			IsResolved: false,
			Comments: []botThreadComment{
				{
					Author:    copilotReviewBotLogin,
					Body:      "Please fix this current-head issue.",
					CreatedAt: "2026-05-03T01:00:00Z",
					CommitOID: "head",
				},
			},
		},
		{
			ID:         "old-thread",
			Path:       "old.go",
			IsResolved: false,
			Comments: []botThreadComment{
				{
					Author:    copilotReviewBotLogin,
					Body:      "Old feedback.",
					CreatedAt: "2026-05-02T01:00:00Z",
					CommitOID: "old",
				},
			},
		},
		{
			ID:         "resolved-thread",
			Path:       "resolved.go",
			IsResolved: true,
			Comments: []botThreadComment{
				{
					Author:    copilotReviewBotLogin,
					Body:      "Resolved feedback.",
					CreatedAt: "2026-05-03T02:00:00Z",
					CommitOID: "head",
				},
			},
		},
	}

	status := buildReviewBotStatus("copilot", copilotReviewBotLogin, "head", nil, threads, botStatusCompleteness{})
	if len(status.UnresolvedCurrentHeadThreads) != 1 {
		t.Fatalf("UnresolvedCurrentHeadThreads len = %d, want 1", len(status.UnresolvedCurrentHeadThreads))
	}
	if len(status.UnresolvedOtherThreads) != 1 {
		t.Fatalf("UnresolvedOtherThreads len = %d, want 1", len(status.UnresolvedOtherThreads))
	}
	if status.UnresolvedCurrentHeadThreads[0].ID != "current-thread" {
		t.Fatalf("Thread ID = %q, want current-thread", status.UnresolvedCurrentHeadThreads[0].ID)
	}
	if status.NextAction != "address, reply to, and resolve the unresolved current-head threads" {
		t.Fatalf("NextAction = %q, want address guidance", status.NextAction)
	}
}

func TestBuildReviewBotStatusUsesFirstThreadCommentForBotAttribution(t *testing.T) {
	t.Parallel()

	threads := []botThreadNode{
		{
			ID:         "human-thread-with-bot-reply",
			Path:       "human.go",
			IsResolved: false,
			Comments: []botThreadComment{
				{
					Author:    "apstndb",
					Body:      "Human feedback.",
					CreatedAt: "2026-05-03T01:00:00Z",
					CommitOID: "head",
				},
				{
					Author:    copilotReviewBotLogin,
					Body:      "Bot follow-up should not own this thread.",
					CreatedAt: "2026-05-03T01:01:00Z",
					CommitOID: "head",
				},
			},
		},
		{
			ID:         "bot-thread-with-human-reply",
			Path:       "bot.go",
			IsResolved: false,
			Comments: []botThreadComment{
				{
					Author:    copilotReviewBotLogin,
					Body:      "Bot feedback.",
					CreatedAt: "2026-05-03T02:00:00Z",
					CommitOID: "head",
				},
				{
					Author:    "apstndb",
					Body:      "Human reply should not replace the thread origin.",
					CreatedAt: "2026-05-03T02:01:00Z",
					CommitOID: "head",
				},
			},
		},
	}

	status := buildReviewBotStatus("copilot", copilotReviewBotLogin, "head", nil, threads, botStatusCompleteness{})
	if len(status.UnresolvedCurrentHeadThreads) != 1 {
		t.Fatalf("UnresolvedCurrentHeadThreads len = %d, want 1", len(status.UnresolvedCurrentHeadThreads))
	}
	thread := status.UnresolvedCurrentHeadThreads[0]
	if thread.ID != "bot-thread-with-human-reply" {
		t.Fatalf("Thread ID = %q, want bot-thread-with-human-reply", thread.ID)
	}
	if thread.Author != copilotReviewBotLogin {
		t.Fatalf("Thread author = %q, want %q", thread.Author, copilotReviewBotLogin)
	}
	if thread.BodyPreview != "Bot feedback." {
		t.Fatalf("BodyPreview = %q, want first bot comment body", thread.BodyPreview)
	}
}

func TestBuildBotReviewReportLocalHeadMatch(t *testing.T) {
	t.Parallel()

	report := buildBotReviewReport(12, "test", "abc", "abc", nil, nil, botStatusCompleteness{})
	if !report.LocalHeadMatchesPR {
		t.Fatal("LocalHeadMatchesPR = false, want true")
	}

	report = buildBotReviewReport(12, "test", "abc", "def", nil, nil, botStatusCompleteness{})
	if report.LocalHeadMatchesPR {
		t.Fatal("LocalHeadMatchesPR = true, want false")
	}
}

func TestBuildReviewBotStatusNormalizesBotLogin(t *testing.T) {
	t.Parallel()

	reviews := []botReviewNode{
		{
			ID:        "copilot-review",
			Author:    copilotReviewBotLogin + "[bot]",
			State:     "COMMENTED",
			Body:      "Copilot generated no new comments.",
			CreatedAt: "2026-05-03T01:00:00Z",
			CommitOID: "head",
		},
	}

	status := buildReviewBotStatus("copilot", copilotReviewBotLogin, "head", reviews, nil, botStatusCompleteness{})
	if !status.ReviewedCurrentHead {
		t.Fatal("Copilot review with [bot] suffix was not recognized")
	}
	if !status.Ready {
		t.Fatal("Copilot review with no unresolved threads should be ready")
	}
}

func TestBuildReviewBotStatusIncompleteDataIsNotReady(t *testing.T) {
	t.Parallel()

	reviews := []botReviewNode{
		{
			ID:        "copilot-review",
			Author:    copilotReviewBotLogin,
			State:     "COMMENTED",
			Body:      "Copilot generated no new comments.",
			CreatedAt: "2026-05-03T01:00:00Z",
			CommitOID: "head",
		},
	}

	status := buildReviewBotStatus("copilot", copilotReviewBotLogin, "head", reviews, nil, botStatusCompleteness{ThreadsTruncated: true})
	if status.Ready {
		t.Fatal("Incomplete review data should not be ready")
	}
	if !status.ReviewDataIncomplete {
		t.Fatal("ReviewDataIncomplete = false, want true")
	}
}

func TestBuildReviewBotStatusTruncatedReviewsWithoutCurrentHeadIsIncomplete(t *testing.T) {
	t.Parallel()

	status := buildReviewBotStatus("copilot", copilotReviewBotLogin, "head", nil, nil, botStatusCompleteness{ReviewsTruncated: true})
	if status.Ready {
		t.Fatal("Truncated reviews without a current-head review should not be ready")
	}
	if !status.ReviewDataIncomplete {
		t.Fatal("ReviewDataIncomplete = false, want true")
	}
}

func TestResolveGitDirFromFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	gitDir := filepath.Join(root, "actual.git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dotGit := filepath.Join(root, ".git")
	if err := os.WriteFile(dotGit, []byte("gitdir: actual.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveGitDir(dotGit)
	if err != nil {
		t.Fatal(err)
	}
	if got != gitDir {
		t.Fatalf("resolveGitDir() = %q, want %q", got, gitDir)
	}
}

func TestReadGitHeadOIDFromPackedRef(t *testing.T) {
	t.Parallel()

	gitDir := t.TempDir()
	const oid = "0123456789abcdef0123456789abcdef01234567"
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "packed-refs"), []byte(oid+" refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := readGitHeadOID(gitDir); got != oid {
		t.Fatalf("readGitHeadOID() = %q, want %q", got, oid)
	}
}
