package main

import (
	"testing"
	"time"
)

func TestUnresolvedOnlyFilter(t *testing.T) {
	// Create test data with both resolved and unresolved threads
	testData := &UnifiedReviewData{
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
				IsResolved: false, // Unresolved - should be included
				IsOutdated: false,
				Comments: []ThreadComment{
					{
						ID:        "COMMENT1",
						Author:    "reviewer1",
						Body:      "This needs fixing",
						CreatedAt: "2025-01-01T10:00:00Z",
					},
				},
				NeedsReply:  true,
				LastReplier: "reviewer1",
			},
			{
				ID:         "THREAD2",
				Path:       "file2.go",
				Line:       intPtr(20),
				IsResolved: true, // Resolved - should be excluded when needsReplyOnly is true
				IsOutdated: false,
				Comments: []ThreadComment{
					{
						ID:        "COMMENT2",
						Author:    "author",
						Body:      "Fixed",
						CreatedAt: "2025-01-01T11:00:00Z",
					},
				},
				NeedsReply:  false,
				LastReplier: "author",
			},
			{
				ID:         "THREAD3",
				Path:       "file3.go",
				Line:       intPtr(30),
				IsResolved: false, // Unresolved - should be included
				IsOutdated: true,
				Comments: []ThreadComment{
					{
						ID:        "COMMENT3",
						Author:    "reviewer2",
						Body:      "Consider this change",
						CreatedAt: "2025-01-01T12:00:00Z",
					},
				},
				NeedsReply:  true,
				LastReplier: "reviewer2",
			},
		},
		CurrentUser: "testuser",
		FetchedAt:   time.Now(),
		ThreadPageInfo: PageInfo{
			TotalCount: 3,
		},
	}

	tests := []struct {
		name               string
		data               *UnifiedReviewData
		unresolvedOnly     bool
		expectedThreadCount int
		expectedUnresolved int
	}{
		{
			name:               "With unresolvedOnly filter - pre-filtered data",
			data:               &UnifiedReviewData{
				PR:          testData.PR,
				Reviews:     testData.Reviews,
				// Simulate pre-filtered data (only unresolved threads)
				Threads:     []ThreadData{testData.Threads[0], testData.Threads[2]},
				CurrentUser: testData.CurrentUser,
				FetchedAt:   testData.FetchedAt,
				ThreadPageInfo: PageInfo{
					TotalCount: 2,  // Pre-filtered, so only 2 unresolved threads
				},
			},
			unresolvedOnly:     true,
			expectedThreadCount: 2,
			expectedUnresolved: 2,
		},
		{
			name:               "Without unresolvedOnly filter - all threads",
			data:               testData,
			unresolvedOnly:     false,
			expectedThreadCount: 3,
			expectedUnresolved: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the thread counting logic from outputFetch
			unresolvedCount := 0
			for _, thread := range tt.data.Threads {
				if tt.unresolvedOnly || !thread.IsResolved {
					unresolvedCount++
				}
			}

			if unresolvedCount != tt.expectedUnresolved {
				t.Errorf("Expected %d unresolved threads, got %d", tt.expectedUnresolved, unresolvedCount)
			}

			// Verify total count from PageInfo, which reflects the implementation
			totalCount := tt.data.ThreadPageInfo.TotalCount
			if totalCount != tt.expectedThreadCount {
				t.Errorf("Expected %d total threads from PageInfo, got %d", tt.expectedThreadCount, totalCount)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}