package main

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAPIUsageRecorderEstimatesGraphQLCostFromSequentialHeaders(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	recorder.Reset(true)

	headers := http.Header{}
	headers.Set("x-ratelimit-limit", "5000")
	headers.Set("x-ratelimit-remaining", "4900")
	headers.Set("x-ratelimit-used", "100")
	headers.Set("x-ratelimit-reset", "1777809256")
	recorder.RecordGraphQLResponse(headers, nil, graphQLRequestTrace{})

	headers = http.Header{}
	headers.Set("x-ratelimit-limit", "5000")
	headers.Set("x-ratelimit-remaining", "4889")
	headers.Set("x-ratelimit-used", "111")
	headers.Set("x-ratelimit-reset", "1777809256")
	recorder.RecordGraphQLResponse(headers, nil, graphQLRequestTrace{})

	recorder.mu.Lock()
	report := recorder.buildReportLocked()
	recorder.mu.Unlock()

	if report.Backend != "graphql" {
		t.Fatalf("Backend = %q, want graphql", report.Backend)
	}
	if report.Observed.GraphQLRequests != 2 {
		t.Fatalf("GraphQLRequests = %d, want 2", report.Observed.GraphQLRequests)
	}
	if report.Observed.GraphQLCost != 11 {
		t.Fatalf("GraphQLCost = %d, want 11", report.Observed.GraphQLCost)
	}
	if report.Observed.GraphQLCostEstimatedRequests != 1 {
		t.Fatalf("GraphQLCostEstimatedRequests = %d, want 1", report.Observed.GraphQLCostEstimatedRequests)
	}
	if report.Observed.GraphQLCostUnavailableRequests != 1 {
		t.Fatalf("GraphQLCostUnavailableRequests = %d, want 1", report.Observed.GraphQLCostUnavailableRequests)
	}
	if report.RateLimit == nil || report.RateLimit.GraphQL == nil {
		t.Fatalf("GraphQL rate limit report is nil")
	}
	if report.RateLimit.GraphQL.Remaining == nil || *report.RateLimit.GraphQL.Remaining != 4889 {
		t.Fatalf("Remaining = %v, want 4889", report.RateLimit.GraphQL.Remaining)
	}
}

func TestAPIUsageRecorderReportsMixedBackend(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	recorder.Reset(true)

	restHeaders := http.Header{}
	restHeaders.Set("x-ratelimit-limit", "5000")
	restHeaders.Set("x-ratelimit-remaining", "4998")
	restHeaders.Set("x-ratelimit-used", "2")
	recorder.RecordRESTResponse(restHeaders, restRequestTrace{})

	graphQLHeaders := http.Header{}
	graphQLHeaders.Set("x-ratelimit-limit", "5000")
	graphQLHeaders.Set("x-ratelimit-remaining", "4990")
	graphQLHeaders.Set("x-ratelimit-used", "10")
	recorder.RecordGraphQLResponse(graphQLHeaders, nil, graphQLRequestTrace{})

	recorder.mu.Lock()
	report := recorder.buildReportLocked()
	recorder.mu.Unlock()

	if report.Backend != "mixed" {
		t.Fatalf("Backend = %q, want mixed", report.Backend)
	}
	if report.Observed.RESTRequests != 1 {
		t.Fatalf("RESTRequests = %d, want 1", report.Observed.RESTRequests)
	}
	if report.Observed.GraphQLRequests != 1 {
		t.Fatalf("GraphQLRequests = %d, want 1", report.Observed.GraphQLRequests)
	}
}

func TestAPIUsageRecorderPrefersGraphQLCostFromResponse(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	recorder.Reset(true)
	resetAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	headers := http.Header{}
	headers.Set("x-ratelimit-used", "999")

	recorder.RecordGraphQLResponse(headers, []byte(fmt.Sprintf(`{
		"data": {
			"ghHelperApiUsageRateLimit": {
				"cost": 4,
				"limit": 5000,
				"nodeCount": 9,
				"remaining": 4996,
				"used": 104,
				"resetAt": %q
			}
		}
	}`, resetAt.Format(time.RFC3339))), graphQLRequestTrace{})

	recorder.mu.Lock()
	report := recorder.buildReportLocked()
	recorder.mu.Unlock()

	if report.Observed.GraphQLCost != 4 {
		t.Fatalf("GraphQLCost = %d, want response cost 4", report.Observed.GraphQLCost)
	}
	if report.Observed.GraphQLCostKnownRequests != 1 {
		t.Fatalf("GraphQLCostKnownRequests = %d, want 1", report.Observed.GraphQLCostKnownRequests)
	}
	if report.Observed.GraphQLNodeCount != 9 {
		t.Fatalf("GraphQLNodeCount = %d, want response node count 9", report.Observed.GraphQLNodeCount)
	}
	if report.RateLimit == nil || report.RateLimit.GraphQL == nil {
		t.Fatalf("GraphQL rate limit report is nil")
	}
	if report.RateLimit.GraphQL.Limit == nil || *report.RateLimit.GraphQL.Limit != 5000 {
		t.Fatalf("GraphQL limit = %v, want 5000", report.RateLimit.GraphQL.Limit)
	}
	if report.RateLimit.GraphQL.Remaining == nil || *report.RateLimit.GraphQL.Remaining != 4996 {
		t.Fatalf("GraphQL remaining = %v, want 4996", report.RateLimit.GraphQL.Remaining)
	}
	if report.RateLimit.GraphQL.Used == nil || *report.RateLimit.GraphQL.Used != 104 {
		t.Fatalf("GraphQL used = %v, want 104", report.RateLimit.GraphQL.Used)
	}
	if report.RateLimit.GraphQL.ResetAt != resetAt.Format(time.RFC3339) {
		t.Fatalf("GraphQL resetAt = %q, want %s", report.RateLimit.GraphQL.ResetAt, resetAt.Format(time.RFC3339))
	}
	if report.RateLimit.GraphQL.ResetInSeconds == nil || *report.RateLimit.GraphQL.ResetInSeconds == 0 {
		t.Fatalf("GraphQL resetInSeconds = %v, want positive value", report.RateLimit.GraphQL.ResetInSeconds)
	}
}

func TestFormatAPIUsageSummary(t *testing.T) {
	t.Parallel()

	remaining := 123
	summary := formatAPIUsageSummary(&APIUsageReport{
		Backend: "graphql",
		Observed: APIUsageObserved{
			GraphQLRequests: 2,
			GraphQLCost:     7,
		},
		RateLimit: &APIUsageRateLimit{
			GraphQL: &APIUsageRateLimitResource{
				Remaining: &remaining,
			},
		},
	})

	for _, want := range []string{"backend=graphql", "graphqlRequests=2", "graphqlCost=7", "graphqlRemaining=123"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary %q does not contain %q", summary, want)
		}
	}
}

func TestAPIUsageRecorderOmitsUnseenRateLimit(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	recorder.Reset(true)

	recorder.mu.Lock()
	report := recorder.buildReportLocked()
	recorder.mu.Unlock()

	if report.RateLimit != nil {
		t.Fatalf("RateLimit = %#v, want nil", report.RateLimit)
	}
}

func TestAPIUsageRecorderUsesWarningWriter(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	var stderr bytes.Buffer
	recorder.Reset(false)
	recorder.SetWarningThreshold(0.5)
	recorder.SetWarningWriter(&stderr)

	headers := http.Header{}
	headers.Set("x-ratelimit-limit", "100")
	headers.Set("x-ratelimit-used", "75")
	headers.Set("x-ratelimit-remaining", "25")
	recorder.RecordGraphQLResponse(headers, nil, graphQLRequestTrace{})

	if got := stderr.String(); !strings.Contains(got, "GitHub graphql API rate limit is 75% used") {
		t.Fatalf("warning output = %q", got)
	}
}

func TestAPIUsageRecorderWarnsPerResource(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	var stderr bytes.Buffer
	recorder.Reset(false)
	recorder.SetWarningThreshold(0.5)
	recorder.SetWarningWriter(&stderr)

	restHeaders := http.Header{}
	restHeaders.Set("x-ratelimit-limit", "100")
	restHeaders.Set("x-ratelimit-used", "60")
	restHeaders.Set("x-ratelimit-remaining", "40")
	recorder.RecordRESTResponse(restHeaders, restRequestTrace{})

	graphQLHeaders := http.Header{}
	graphQLHeaders.Set("x-ratelimit-limit", "100")
	graphQLHeaders.Set("x-ratelimit-used", "70")
	graphQLHeaders.Set("x-ratelimit-remaining", "30")
	recorder.RecordGraphQLResponse(graphQLHeaders, nil, graphQLRequestTrace{})

	got := stderr.String()
	for _, want := range []string{
		"GitHub core API rate limit is 60% used",
		"GitHub graphql API rate limit is 70% used",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("warning output %q does not contain %q", got, want)
		}
	}
}

func TestAPIUsageRecorderIncludesRateLimitWarningsInReport(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	var stderr bytes.Buffer
	recorder.Reset(true)
	recorder.SetWarningThreshold(0.5)
	recorder.SetWarningWriter(&stderr)

	headers := http.Header{}
	headers.Set("x-ratelimit-limit", "100")
	headers.Set("x-ratelimit-used", "75")
	headers.Set("x-ratelimit-remaining", "25")
	recorder.RecordGraphQLResponse(headers, nil, graphQLRequestTrace{})

	recorder.mu.Lock()
	report := recorder.buildReportLocked()
	recorder.mu.Unlock()

	if len(report.Warnings) != 1 {
		t.Fatalf("Warnings length = %d, want 1: %#v", len(report.Warnings), report.Warnings)
	}
	if !strings.Contains(report.Warnings[0], "GitHub graphql API rate limit is 75% used") {
		t.Fatalf("warning = %q", report.Warnings[0])
	}
	if got := stderr.String(); !strings.Contains(got, "warning: "+report.Warnings[0]) {
		t.Fatalf("stderr warning %q does not contain structured warning %q", got, report.Warnings[0])
	}
}

func TestAPIUsageRecorderSkipsGraphQLBodyParseForWarningOnly(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	recorder.Reset(false)
	recorder.SetWarningThreshold(0.5)

	headers := http.Header{}
	headers.Set("x-ratelimit-limit", "5000")
	headers.Set("x-ratelimit-remaining", "4999")
	headers.Set("x-ratelimit-used", "1")

	recorder.RecordGraphQLResponse(headers, []byte(`{
		"data": {
			"ghHelperApiUsageRateLimit": {
				"cost": 99,
				"limit": 5000,
				"nodeCount": 999,
				"remaining": 4900,
				"used": 100,
				"resetAt": "2026-05-03T12:00:00Z"
			}
		}
	}`), graphQLRequestTrace{})

	recorder.mu.Lock()
	report := recorder.buildReportLocked()
	recorder.mu.Unlock()

	if report.Observed.GraphQLRequests != 0 {
		t.Fatalf("GraphQLRequests = %d, want 0", report.Observed.GraphQLRequests)
	}
	if report.Observed.GraphQLCost != 0 {
		t.Fatalf("GraphQLCost = %d, want 0", report.Observed.GraphQLCost)
	}
	if report.RateLimit == nil || report.RateLimit.GraphQL == nil {
		t.Fatalf("GraphQL rate limit report is nil")
	}
	if report.RateLimit.GraphQL.Used == nil || *report.RateLimit.GraphQL.Used != 1 {
		t.Fatalf("GraphQL used = %v, want header value 1", report.RateLimit.GraphQL.Used)
	}
}

func TestAPIUsageRecorderSkipsGraphQLBodyParseWithoutAlias(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	recorder.Reset(true)

	headers := http.Header{}
	headers.Set("x-ratelimit-limit", "5000")
	headers.Set("x-ratelimit-remaining", "4999")
	headers.Set("x-ratelimit-used", "1")

	recorder.RecordGraphQLResponse(headers, []byte(`{
		"data": {
			"repository": {
				"id": "R_1"
			}
		}
	}`), graphQLRequestTrace{})

	recorder.mu.Lock()
	report := recorder.buildReportLocked()
	recorder.mu.Unlock()

	if report.Observed.GraphQLRequests != 1 {
		t.Fatalf("GraphQLRequests = %d, want 1", report.Observed.GraphQLRequests)
	}
	if report.Observed.GraphQLCostKnownRequests != 0 {
		t.Fatalf("GraphQLCostKnownRequests = %d, want 0", report.Observed.GraphQLCostKnownRequests)
	}
	if report.Observed.GraphQLCostUnavailableRequests != 1 {
		t.Fatalf("GraphQLCostUnavailableRequests = %d, want 1", report.Observed.GraphQLCostUnavailableRequests)
	}
}

func TestAPIUsageRecorderWritesGraphQLTrace(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	var stderr bytes.Buffer
	recorder.Reset(false)
	recorder.SetTrace(true, &stderr)
	resetAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	headers := http.Header{}
	headers.Set("x-ratelimit-used", "999")

	recorder.RecordGraphQLResponse(headers, []byte(fmt.Sprintf(`{
		"data": {
			"ghHelperApiUsageRateLimit": {
				"cost": 4,
				"limit": 5000,
				"nodeCount": 9,
				"remaining": 4996,
				"used": 104,
				"resetAt": %q
			}
		}
	}`, resetAt.Format(time.RFC3339))), graphQLRequestTrace{
		OperationType: "query",
		OperationName: "ReviewThreads",
		StatusCode:    http.StatusOK,
	})

	got := stderr.String()
	for _, want := range []string{
		"api-usage: graphql request=1",
		"operation=query",
		"name=ReviewThreads",
		"status=200",
		"costSource=response",
		"cost=4",
		"nodeCount=9",
		"used=104/5000",
		"remaining=4996",
		"resetIn=",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("trace output %q does not contain %q", got, want)
		}
	}
}

func TestAttachAPIUsageToOutputAlwaysWrapsData(t *testing.T) {
	oldAPIUsage := apiUsage
	defer func() { apiUsage = oldAPIUsage }()
	apiUsage = true

	commandAPIUsage.Reset(true)
	got, attached := attachAPIUsageToOutput(nil, map[string]interface{}{"value": 1})
	if !attached {
		t.Fatal("attached = false, want true")
	}

	output, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("output type = %T, want map[string]interface{}", got)
	}
	if _, ok := output["apiUsage"]; !ok {
		t.Fatalf("output missing apiUsage: %#v", output)
	}
	data, ok := output["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data type = %T, want map[string]interface{}", output["data"])
	}
	if data["value"] != 1 {
		t.Fatalf("data[value] = %v, want 1", data["value"])
	}
	if _, ok := output["value"]; ok {
		t.Fatalf("output should not inject original map fields at root: %#v", output)
	}
}

func TestAPIUsageRecorderWritesRESTTrace(t *testing.T) {
	t.Parallel()

	recorder := &apiUsageRecorder{}
	var stderr bytes.Buffer
	recorder.Reset(false)
	recorder.SetTrace(true, &stderr)
	headers := http.Header{}
	headers.Set("x-ratelimit-limit", "5000")
	headers.Set("x-ratelimit-used", "2")
	headers.Set("x-ratelimit-remaining", "4998")

	recorder.RecordRESTResponse(headers, restRequestTrace{
		Method:     http.MethodPost,
		Path:       "/repos/apstndb/gh-dev-tools/issues/63/comments",
		StatusCode: http.StatusCreated,
	})

	got := stderr.String()
	for _, want := range []string{
		"api-usage: rest request=1",
		"method=POST",
		"path=/repos/apstndb/gh-dev-tools/issues/63/comments",
		"status=201",
		"used=2/5000",
		"remaining=4998",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("trace output %q does not contain %q", got, want)
		}
	}
}
