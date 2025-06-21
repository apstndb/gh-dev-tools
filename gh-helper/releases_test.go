package main

import (
	"testing"
)

func TestAnalyzePRs(t *testing.T) {
	tests := []struct {
		name     string
		prs      []PRData
		expected struct {
			totalPRs              int
			missingClassification int
			shouldIgnore          int
			inconsistentLabeling  int
			wellLabeled           int
			needsAttention        int
			readyForRelease       bool
		}
	}{
		{
			name: "PRs with feat prefix need enhancement label",
			prs: []PRData{
				{
					Number: 1,
					Title:  "feat: add new feature",
					Labels: []string{},
				},
				{
					Number: 2,
					Title:  "fix: resolve bug",
					Labels: []string{},
				},
			},
			expected: struct {
				totalPRs              int
				missingClassification int
				shouldIgnore          int
				inconsistentLabeling  int
				wellLabeled           int
				needsAttention        int
				readyForRelease       bool
			}{
				totalPRs:              2,
				missingClassification: 2,
				shouldIgnore:          0,
				inconsistentLabeling:  0,
				wellLabeled:           0,
				needsAttention:        2,
				readyForRelease:       false,
			},
		},
		{
			name: "PRs with proper labels",
			prs: []PRData{
				{
					Number: 1,
					Title:  "feat: add new feature",
					Labels: []string{"enhancement"},
				},
				{
					Number: 2,
					Title:  "fix: resolve bug",
					Labels: []string{"bug"},
				},
			},
			expected: struct {
				totalPRs              int
				missingClassification int
				shouldIgnore          int
				inconsistentLabeling  int
				wellLabeled           int
				needsAttention        int
				readyForRelease       bool
			}{
				totalPRs:              2,
				missingClassification: 0,
				shouldIgnore:          0,
				inconsistentLabeling:  0,
				wellLabeled:           2,
				needsAttention:        0,
				readyForRelease:       true,
			},
		},
		{
			name: "Internal docs should be ignored",
			prs: []PRData{
				{
					Number: 1,
					Title:  "docs: update CLAUDE.md",
					Labels: []string{},
				},
				{
					Number: 2,
					Title:  "Update dev-docs/architecture.md",
					Labels: []string{},
				},
			},
			expected: struct {
				totalPRs              int
				missingClassification int
				shouldIgnore          int
				inconsistentLabeling  int
				wellLabeled           int
				needsAttention        int
				readyForRelease       bool
			}{
				totalPRs:              2,
				missingClassification: 0,
				shouldIgnore:          2,
				inconsistentLabeling:  0,
				wellLabeled:           0,
				needsAttention:        2,
				readyForRelease:       false,
			},
		},
		{
			name: "User-facing docs with ignore label is inconsistent",
			prs: []PRData{
				{
					Number: 1,
					Title:  "docs: update README installation guide",
					Labels: []string{"documentation", "ignore-for-release"},
				},
			},
			expected: struct {
				totalPRs              int
				missingClassification int
				shouldIgnore          int
				inconsistentLabeling  int
				wellLabeled           int
				needsAttention        int
				readyForRelease       bool
			}{
				totalPRs:              1,
				missingClassification: 0,
				shouldIgnore:          0,
				inconsistentLabeling:  1,
				wellLabeled:           0,
				needsAttention:        1,
				readyForRelease:       false,
			},
		},
		{
			name: "PR fixing bug issue inherits label",
			prs: []PRData{
				{
					Number: 1,
					Title:  "Fix error handling",
					Labels: []string{},
					LinkedIssues: []LinkedIssue{
						{
							Number: 10,
							Labels: []string{"bug", "high-priority"},
						},
					},
				},
			},
			expected: struct {
				totalPRs              int
				missingClassification int
				shouldIgnore          int
				inconsistentLabeling  int
				wellLabeled           int
				needsAttention        int
				readyForRelease       bool
			}{
				totalPRs:              1,
				missingClassification: 1, // Should suggest "bug" label
				shouldIgnore:          0,
				inconsistentLabeling:  0,
				wellLabeled:           0,
				needsAttention:        1,
				readyForRelease:       false,
			},
		},
		{
			name: "PR in multiple categories counts once",
			prs: []PRData{
				{
					Number: 1,
					Title:  "Update CLAUDE.md", // Should be ignored AND missing classification
					Labels: []string{},
				},
			},
			expected: struct {
				totalPRs              int
				missingClassification int
				shouldIgnore          int
				inconsistentLabeling  int
				wellLabeled           int
				needsAttention        int
				readyForRelease       bool
			}{
				totalPRs:              1,
				missingClassification: 0, // Ignored PRs don't need classification
				shouldIgnore:          1,
				inconsistentLabeling:  0,
				wellLabeled:           0,
				needsAttention:        1, // Only counted once
				readyForRelease:       false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := analyzePRs(tt.prs)

			// Check counts
			if analysis.TotalPRs != tt.expected.totalPRs {
				t.Errorf("TotalPRs = %d, want %d", analysis.TotalPRs, tt.expected.totalPRs)
			}

			if len(analysis.MissingClassification) != tt.expected.missingClassification {
				t.Errorf("MissingClassification = %d, want %d", len(analysis.MissingClassification), tt.expected.missingClassification)
			}

			if len(analysis.ShouldIgnore) != tt.expected.shouldIgnore {
				t.Errorf("ShouldIgnore = %d, want %d", len(analysis.ShouldIgnore), tt.expected.shouldIgnore)
			}

			if len(analysis.InconsistentLabeling) != tt.expected.inconsistentLabeling {
				t.Errorf("InconsistentLabeling = %d, want %d", len(analysis.InconsistentLabeling), tt.expected.inconsistentLabeling)
			}

			if analysis.Summary.WellLabeled != tt.expected.wellLabeled {
				t.Errorf("WellLabeled = %d, want %d", analysis.Summary.WellLabeled, tt.expected.wellLabeled)
			}

			if analysis.Summary.NeedsAttention != tt.expected.needsAttention {
				t.Errorf("NeedsAttention = %d, want %d", analysis.Summary.NeedsAttention, tt.expected.needsAttention)
			}

			if analysis.Summary.ReadyForRelease != tt.expected.readyForRelease {
				t.Errorf("ReadyForRelease = %v, want %v", analysis.Summary.ReadyForRelease, tt.expected.readyForRelease)
			}
		})
	}
}

func TestSuggestClassificationLabel(t *testing.T) {
	tests := []struct {
		name     string
		pr       PRData
		expected *PRClassificationSuggestion
	}{
		{
			name: "feat prefix suggests enhancement",
			pr: PRData{
				Number: 1,
				Title:  "feat: add new command",
			},
			expected: &PRClassificationSuggestion{
				Number:         1,
				Title:          "feat: add new command",
				SuggestedLabel: "enhancement",
				Reasoning:      "Title starts with 'feat:' prefix",
			},
		},
		{
			name: "fix prefix suggests bug",
			pr: PRData{
				Number: 2,
				Title:  "fix: resolve crash",
			},
			expected: &PRClassificationSuggestion{
				Number:         2,
				Title:          "fix: resolve crash",
				SuggestedLabel: "bug",
				Reasoning:      "Title starts with 'fix:' prefix",
			},
		},
		{
			name: "docs prefix suggests documentation",
			pr: PRData{
				Number: 3,
				Title:  "docs: update API guide",
			},
			expected: &PRClassificationSuggestion{
				Number:         3,
				Title:          "docs: update API guide",
				SuggestedLabel: "documentation",
				Reasoning:      "Title starts with 'docs:' prefix",
			},
		},
		{
			name: "chore prefix suggests chore",
			pr: PRData{
				Number: 4,
				Title:  "chore: update dependencies",
			},
			expected: &PRClassificationSuggestion{
				Number:         4,
				Title:          "chore: update dependencies",
				SuggestedLabel: "chore",
				Reasoning:      "Title indicates maintenance work",
			},
		},
		{
			name: "linked bug issue",
			pr: PRData{
				Number: 5,
				Title:  "Resolve error handling issue",
				LinkedIssues: []LinkedIssue{
					{
						Number: 100,
						Labels: []string{"bug"},
					},
				},
			},
			expected: &PRClassificationSuggestion{
				Number:         5,
				Title:          "Resolve error handling issue",
				SuggestedLabel: "bug",
				Reasoning:      "Fixes issue #100 which has 'bug' label",
			},
		},
		{
			name: "add keyword suggests enhancement",
			pr: PRData{
				Number: 6,
				Title:  "Add support for JSON output",
			},
			expected: &PRClassificationSuggestion{
				Number:         6,
				Title:          "Add support for JSON output",
				SuggestedLabel: "enhancement",
				Reasoning:      "Title suggests new functionality",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := suggestClassificationLabel(tt.pr)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected suggestion, got nil")
				return
			}

			if result.Number != tt.expected.Number {
				t.Errorf("Number = %d, want %d", result.Number, tt.expected.Number)
			}

			if result.SuggestedLabel != tt.expected.SuggestedLabel {
				t.Errorf("SuggestedLabel = %s, want %s", result.SuggestedLabel, tt.expected.SuggestedLabel)
			}

			if result.Reasoning != tt.expected.Reasoning {
				t.Errorf("Reasoning = %s, want %s", result.Reasoning, tt.expected.Reasoning)
			}
		})
	}
}

func TestIsUserFacingDoc(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		body     string
		expected bool
	}{
		{
			name:     "README is user-facing",
			title:    "Update README.md",
			body:     "Added installation instructions",
			expected: true,
		},
		{
			name:     "Tutorial is user-facing",
			title:    "Add beginner tutorial",
			body:     "New tutorial for getting started",
			expected: true,
		},
		{
			name:     "Usage guide is user-facing",
			title:    "Document command usage",
			body:     "Added usage examples",
			expected: true,
		},
		{
			name:     "CLAUDE.md is not user-facing",
			title:    "Update CLAUDE.md",
			body:     "Added developer guidelines",
			expected: false,
		},
		{
			name:     "Internal docs are not user-facing",
			title:    "Update internal architecture docs",
			body:     "Technical details for maintainers",
			expected: false,
		},
		{
			name:     "Case insensitive matching",
			title:    "update readme",
			body:     "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUserFacingDoc(tt.title, tt.body)
			if result != tt.expected {
				t.Errorf("isUserFacingDoc(%q, %q) = %v, want %v", tt.title, tt.body, result, tt.expected)
			}
		})
	}
}

func TestHasLabel(t *testing.T) {
	tests := []struct {
		name     string
		labels   []string
		target   string
		expected bool
	}{
		{
			name:     "exact match",
			labels:   []string{"bug", "enhancement"},
			target:   "bug",
			expected: true,
		},
		{
			name:     "case insensitive match",
			labels:   []string{"Bug", "Enhancement"},
			target:   "bug",
			expected: true,
		},
		{
			name:     "no match",
			labels:   []string{"feature", "documentation"},
			target:   "bug",
			expected: false,
		},
		{
			name:     "empty labels",
			labels:   []string{},
			target:   "bug",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasLabel(tt.labels, tt.target)
			if result != tt.expected {
				t.Errorf("hasLabel(%v, %q) = %v, want %v", tt.labels, tt.target, result, tt.expected)
			}
		})
	}
}