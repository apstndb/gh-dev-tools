package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestOutputFetchThreadFiltering(t *testing.T) {
	// Create test data with both resolved and unresolved threads
	baseData := &UnifiedReviewData{
		PR: PRMetadata{
			Number: 123,
			Title:  "Test PR",
			State:  "OPEN",
		},
		Reviews: []ReviewData{},
		Threads: []ThreadData{
			{
				ID:         "THREAD1",
				Path:       "file1.go",
				Line:       intPtr(10),
				IsResolved: false, // Unresolved
				IsOutdated: false,
				Comments: []ThreadComment{
					{
						ID:        "COMMENT1",
						Author:    "reviewer1",
						Body:      "This needs fixing",
						CreatedAt: "2025-01-01T10:00:00Z",
					},
				},
			},
			{
				ID:         "THREAD2",
				Path:       "file2.go",
				Line:       intPtr(20),
				IsResolved: true, // Resolved
				IsOutdated: false,
				Comments: []ThreadComment{
					{
						ID:        "COMMENT2",
						Author:    "author",
						Body:      "Fixed",
						CreatedAt: "2025-01-01T11:00:00Z",
					},
				},
			},
			{
				ID:         "THREAD3",
				Path:       "file3.go",
				Line:       intPtr(30),
				IsResolved: false, // Unresolved
				IsOutdated: true,
				Comments: []ThreadComment{
					{
						ID:        "COMMENT3",
						Author:    "reviewer2",
						Body:      "Consider this change",
						CreatedAt: "2025-01-01T12:00:00Z",
					},
				},
			},
		},
		CurrentUser: "testuser",
		FetchedAt:   time.Now(),
		ThreadPageInfo: PageInfo{
			TotalCount: 3,
		},
	}

	tests := []struct {
		name                      string
		data                      *UnifiedReviewData
		includeReviewBodies       bool
		includeThreads            bool
		expectedTotalCount        int
		expectedUnresolvedCount   int
		expectedUnresolvedThreads int
	}{
		{
			name:                      "All threads included",
			data:                      baseData,
			includeReviewBodies:       false,
			includeThreads:            true,
			expectedTotalCount:        3,
			expectedUnresolvedCount:   2,
			expectedUnresolvedThreads: 2,
		},
		{
			name: "Pre-filtered data (only unresolved)",
			data: &UnifiedReviewData{
				PR:      baseData.PR,
				Reviews: baseData.Reviews,
				// Simulate pre-filtered data from GetUnifiedReviewData
				Threads: []ThreadData{
					baseData.Threads[0], // THREAD1 (unresolved)
					baseData.Threads[2], // THREAD3 (unresolved)
				},
				CurrentUser: baseData.CurrentUser,
				FetchedAt:   baseData.FetchedAt,
				ThreadPageInfo: PageInfo{
					TotalCount: 2, // Total reflects pre-filtered count
				},
			},
			includeReviewBodies:       false,
			includeThreads:            true,
			expectedTotalCount:        2,
			expectedUnresolvedCount:   2,
			expectedUnresolvedThreads: 2,
		},
		{
			name:                      "Threads not included",
			data:                      baseData,
			includeReviewBodies:       false,
			includeThreads:            false,
			expectedTotalCount:        0, // No threads section in output
			expectedUnresolvedCount:   0,
			expectedUnresolvedThreads: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a command with output buffer
			cmd := &cobra.Command{}
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.Flags().String("format", "json", "Output format")

			// Call the actual outputFetch function
			err := outputFetch(cmd, tt.data, tt.includeReviewBodies, tt.includeThreads)
			if err != nil {
				t.Fatalf("outputFetch returned error: %v", err)
			}

			// Parse the JSON output
			var output map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
				t.Fatalf("Failed to parse JSON output: %v", err)
			}

			// Verify thread section exists when expected
			if tt.includeThreads {
				reviewThreads, ok := output["reviewThreads"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected reviewThreads in output")
				}

				// Check totalCount
				totalCount := int(reviewThreads["totalCount"].(float64))
				if totalCount != tt.expectedTotalCount {
					t.Errorf("Expected totalCount %d, got %d", tt.expectedTotalCount, totalCount)
				}

				// Check unresolvedCount
				unresolvedCount := int(reviewThreads["unresolvedCount"].(float64))
				if unresolvedCount != tt.expectedUnresolvedCount {
					t.Errorf("Expected unresolvedCount %d, got %d", tt.expectedUnresolvedCount, unresolvedCount)
				}

				// Check unresolvedThreads array length
				unresolvedThreads, ok := reviewThreads["unresolvedThreads"].([]interface{})
				if !ok {
					t.Fatal("Expected unresolvedThreads array in output")
				}
				if len(unresolvedThreads) != tt.expectedUnresolvedThreads {
					t.Errorf("Expected %d unresolved threads, got %d", tt.expectedUnresolvedThreads, len(unresolvedThreads))
				}

				// Verify all threads in output are actually unresolved
				for _, thread := range unresolvedThreads {
					threadMap := thread.(map[string]interface{})
					isResolved := threadMap["isResolved"].(bool)
					if isResolved {
						t.Error("Found resolved thread in unresolvedThreads array")
					}
				}
			} else {
				// Verify reviewThreads section doesn't exist
				if _, ok := output["reviewThreads"]; ok {
					t.Error("Did not expect reviewThreads in output when includeThreads is false")
				}
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}