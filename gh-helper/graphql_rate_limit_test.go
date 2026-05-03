package main

import (
	"strings"
	"testing"
)

func TestInjectGraphQLRateLimitIntoNamedQuery(t *testing.T) {
	t.Parallel()

	query := `query($owner: String!, $repo: String!) {
  repository(owner: $owner, name: $repo) {
    id
  }
}`

	got, ok := injectGraphQLRateLimit(query)
	if !ok {
		t.Fatal("injectGraphQLRateLimit() ok = false, want true")
	}
	if !strings.Contains(got, "ghHelperApiUsageRateLimit: rateLimit") {
		t.Fatalf("injected query does not contain rateLimit alias:\n%s", got)
	}
	if !strings.Contains(got, "cost") {
		t.Fatalf("injected query does not contain cost field:\n%s", got)
	}
	if !strings.Contains(got, "nodeCount") {
		t.Fatalf("injected query does not contain nodeCount field:\n%s", got)
	}
}

func TestInjectGraphQLRateLimitWithLeadingFragment(t *testing.T) {
	t.Parallel()

	query := `fragment RepoFields on Repository {
  id
}

query($owner: String!, $repo: String!) {
  repository(owner: $owner, name: $repo) {
    ...RepoFields
  }
}`

	got, ok := injectGraphQLRateLimit(query)
	if !ok {
		t.Fatal("injectGraphQLRateLimit() ok = false, want true")
	}
	if !strings.Contains(got, "ghHelperApiUsageRateLimit: rateLimit") {
		t.Fatalf("injected query does not contain rateLimit alias:\n%s", got)
	}
	if strings.Index(got, "ghHelperApiUsageRateLimit") < strings.Index(got, "query(") {
		t.Fatalf("rateLimit was injected before operation instead of inside query:\n%s", got)
	}
}

func TestInjectGraphQLRateLimitSkipsDirectiveArgumentObject(t *testing.T) {
	t.Parallel()

	query := `query($owner: String!, $repo: String!) @custom(config: {key: "val"}) {
  repository(owner: $owner, name: $repo) {
    id
  }
}`

	got, ok := injectGraphQLRateLimit(query)
	if !ok {
		t.Fatal("injectGraphQLRateLimit() ok = false, want true")
	}
	injectedAt := strings.Index(got, graphQLRateLimitAlias)
	repositoryAt := strings.Index(got, "repository")
	if injectedAt < 0 {
		t.Fatalf("injected query does not contain rateLimit alias:\n%s", got)
	}
	if injectedAt < repositoryAt {
		t.Fatalf("rateLimit was injected before the root selection body:\n%s", got)
	}
	if strings.Contains(got, "@custom(config: {\n  "+graphQLRateLimitAlias) {
		t.Fatalf("rateLimit was injected into directive arguments:\n%s", got)
	}
}

func TestInjectGraphQLRateLimitSkipsMutation(t *testing.T) {
	t.Parallel()

	mutation := `mutation($id: ID!) {
  resolveReviewThread(input: {threadId: $id}) {
    thread {
      id
    }
  }
}`

	got, ok := injectGraphQLRateLimit(mutation)
	if ok {
		t.Fatal("injectGraphQLRateLimit() ok = true, want false")
	}
	if got != mutation {
		t.Fatal("mutation should not be modified")
	}
}

func TestGraphQLRateLimitFromResponse(t *testing.T) {
	t.Parallel()

	rateLimit, ok := graphQLRateLimitFromResponse([]byte(`{
		"data": {
			"repository": {"id": "R_1"},
			"ghHelperApiUsageRateLimit": {
				"cost": 12,
				"limit": 5000,
				"nodeCount": 42,
				"remaining": 4988,
				"used": 12,
				"resetAt": "2026-05-03T12:00:00Z"
			}
		}
	}`))
	if !ok {
		t.Fatal("graphQLRateLimitFromResponse() ok = false, want true")
	}
	if rateLimit.Cost != 12 {
		t.Fatalf("graphQLRateLimitFromResponse() cost = %d, want 12", rateLimit.Cost)
	}
	if rateLimit.NodeCount != 42 {
		t.Fatalf("graphQLRateLimitFromResponse() nodeCount = %d, want 42", rateLimit.NodeCount)
	}
}

func TestGraphQLOperationTrace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		query         string
		wantOperation string
		wantName      string
	}{
		{
			name:          "named query",
			query:         `query ReviewThreads($owner: String!) { repository(owner: $owner, name: "repo") { id } }`,
			wantOperation: "query",
			wantName:      "ReviewThreads",
		},
		{
			name:          "anonymous query with variables",
			query:         `query($owner: String!) { viewer { login } }`,
			wantOperation: "query",
			wantName:      "anonymous",
		},
		{
			name:          "short anonymous query",
			query:         `{ viewer { login } }`,
			wantOperation: "query",
			wantName:      "anonymous",
		},
		{
			name:          "named mutation",
			query:         `mutation ResolveThread($id: ID!) { resolveReviewThread(input: {threadId: $id}) { thread { id } } }`,
			wantOperation: "mutation",
			wantName:      "ResolveThread",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotOperation, gotName := graphQLOperationTrace(tt.query)
			if gotOperation != tt.wantOperation || gotName != tt.wantName {
				t.Fatalf("graphQLOperationTrace() = (%q, %q), want (%q, %q)", gotOperation, gotName, tt.wantOperation, tt.wantName)
			}
		})
	}
}
