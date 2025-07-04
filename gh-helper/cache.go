package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WorkflowCache manages persistent state for development workflows
type WorkflowCache struct {
	CacheDir string
}

// NewWorkflowCache creates a new workflow cache instance
func NewWorkflowCache() *WorkflowCache {
	cacheDir := GetCacheDir()
	return &WorkflowCache{CacheDir: cacheDir}
}

// BranchPRMapping represents the mapping between a branch and its PR
type BranchPRMapping struct {
	Branch    string    `json:"branch"`
	PRNumber  int       `json:"pr_number"`
	PRTitle   string    `json:"pr_title"`
	IssueNumber int     `json:"issue_number,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SaveBranchPRMapping saves the mapping between current branch and PR number
func (c *WorkflowCache) SaveBranchPRMapping(branch string, prNumber int, prTitle string, issueNumber int) error {
	if err := os.MkdirAll(c.CacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	mapping := BranchPRMapping{
		Branch:      branch,
		PRNumber:    prNumber,
		PRTitle:     prTitle,
		IssueNumber: issueNumber,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mapping: %w", err)
	}

	cacheFile := filepath.Join(c.CacheDir, fmt.Sprintf("branch-%s.json", branch))
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	fmt.Printf("💾 Saved branch-PR mapping: %s -> PR #%d\n", branch, prNumber)
	return nil
}

// GetBranchPRMapping retrieves the PR number for the current branch
func (c *WorkflowCache) GetBranchPRMapping(branch string) (*BranchPRMapping, error) {
	cacheFile := filepath.Join(c.CacheDir, fmt.Sprintf("branch-%s.json", branch))
	
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("no cached mapping found for branch %s", branch)
	}

	var mapping BranchPRMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("failed to parse cached mapping: %w", err)
	}

	return &mapping, nil
}

// GetCacheDir returns the cache directory for the current repository
//
// In git worktree environments, each worktree gets its own cache directory.
// This ensures independent caching per worktree, which is desirable for
// parallel development workflows where different worktrees may be working
// on different features, branches, or PRs simultaneously.
func GetCacheDir() string {
	repoRoot, err := GetRepositoryRoot()
	if err != nil {
		// Fallback to current directory if not in git repository
		repoRoot = "."
	}
	return filepath.Join(repoRoot, ".cache")
}

// GetCurrentBranch is now defined in git_remote.go