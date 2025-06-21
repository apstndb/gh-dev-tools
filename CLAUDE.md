# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**IMPORTANT**: This file must be written entirely in English. Do not use Japanese or any other languages in CLAUDE.md.

## Project Overview

gh-dev-tools provides generic GitHub development tools optimized for AI assistants. This is a standalone repository extracted from spanner-mycli dev-tools to create reusable GitHub operations tools.

## CRITICAL REQUIREMENTS

**Before ANY push to the repository**:
1. **Always run `make check`** - runs test && lint (required for quality assurance)
2. **Resolve conflicts with origin/main** - ensure branch can merge cleanly to avoid integration issues
3. **Never push directly to main branch** - always use Pull Requests
4. **Never commit directly to main branch** - always use feature branches

**Before ANY PR merge**:
1. **Resolve ALL review threads** - use `./bin/gh-helper reviews fetch <PR> --list-threads` to verify no unresolved threads remain
2. **Address ALL review feedback** - implement suggested changes or provide clear explanations
3. **Verify all checks pass** - ensure CI/CD pipeline completes successfully

**During issue organization/maintenance**:
1. **Check past PRs for unresolved threads** - use `./bin/gh-helper reviews fetch <PR> --list-threads` on recent merged PRs
2. **Resolve implemented feedback** - if suggestions from past PRs have been implemented, resolve the original threads
3. **Update documentation** - ensure CLAUDE.md reflects current best practices learned from reviews

## Essential Commands

```bash
# Development cycle (CRITICAL)
make check          # REQUIRED before ANY push (runs test && lint)
make build          # Build gh-helper tool
make test-quick     # Quick tests during development

# Tool usage
make build              # Build gh-helper tool
./bin/gh-helper --help  # Show available commands
./bin/gh-helper reviews fetch <PR>    # Fetch review data
./bin/gh-helper threads reply <ID>    # Reply to review thread

# New enhanced review features
./bin/gh-helper reviews wait <PR> --async --detailed  # Get comprehensive PR status
./bin/gh-helper reviews wait <PR> --request-summary   # Request and wait for Gemini summary
./bin/gh-helper threads reply <ID1> <ID2> <ID3> --resolve  # Bulk reply to threads

# Issue and sub-issue management
./bin/gh-helper issues show <number>               # Show basic issue information
./bin/gh-helper issues show <number> --include-sub # Show issue with sub-issues and statistics
./bin/gh-helper issues edit <number> --parent <parent>  # Add issue as sub-issue
./bin/gh-helper issues edit <number> --parent <parent> --overwrite  # Move to new parent
./bin/gh-helper issues edit <number> --unlink-parent  # Remove parent relationship
```

## Core Architecture

### gh-helper
- **Main package**: All functionality merged into single main package
- **Unified output**: JSON/YAML using goccy/go-yaml library
- **GitHub GraphQL**: Direct use of GitHub API types without conversion
- **AI-optimized**: Structured output for assistant workflows

## Development Workflow

### Branch Management
- **Always fetch before creating branches**: Run `git fetch origin` before creating any new branch
- **Base branches on origin/main**: Always create feature branches from `origin/main` to ensure they include latest changes
- **Feature branches**: Always create feature branches for development
- **Pull Requests**: All changes must go through PR review
- **No direct commits**: Never commit directly to main branch

Example:
```bash
# Always fetch latest changes first
git fetch origin
# Create branch from latest origin/main
git checkout -b feature/new-feature origin/main
```

### Testing and Quality
```bash
make test           # Full test suite (required before push)
make lint           # Code quality checks (required before push)
make check          # Combined test && lint (required before push)
```

### Git Practices
- **Always use `git add <specific-files>`** - Never use `git add .` or `git add -A`
- **Check `git status` before committing** - Verify only intended files are staged
- **Verify staged files** - Ensure no build artifacts, temporary, or unintended files are included
- Link PRs to issues when applicable

Example workflow:
```bash
# Make changes to specific files
git add path/to/file1.go path/to/file2.go
git status  # Verify only intended files are staged
git commit -m "description of changes"
git push origin HEAD  # Explicit push to current branch
```

### Review Thread Management
- **Always resolve review threads** after addressing feedback
- **CRITICAL:** Push commits BEFORE replying to threads - GitHub needs the commit to exist for proper linking
- **For threads requiring code changes**:
  1. Make the necessary changes and commit
  2. **Push the commit to GitHub first**: `git push origin HEAD`
  3. Reply with commit hash and resolve: `./bin/gh-helper threads reply <THREAD_ID> --commit-hash <HASH> --message "Fixed as suggested" --resolve`
- **For threads not requiring changes**:
  1. Reply with explanation and resolve: `./bin/gh-helper threads reply <THREAD_ID> --message "Explanation here" --resolve`
- **Batch resolve multiple threads**: `./bin/gh-helper threads resolve <THREAD_ID1> <THREAD_ID2> <THREAD_ID3>`
- **Bulk reply with custom messages**: `./bin/gh-helper threads reply THREAD1:"Fixed typo" THREAD2:"Refactored" --commit-hash <HASH> --resolve`

### Handling Unresolved Review Feedback
- **Before merging any PR**: Ensure ALL review threads are either:
  - Implemented in the PR with corresponding commits
  - Explicitly replied to with clear justification why not implemented
  - Tracked in a follow-up issue for future work
- **Create tracking issues**: For valid feedback that can't be addressed immediately:
  ```bash
  ./bin/gh-helper issues create --title "Address feedback from PR #X" --body "..."
  ./bin/gh-helper threads reply <THREAD_ID> --message "Created issue #Y to track this improvement" --resolve
  ```
- **Audit merged PRs**: Periodically check for missed feedback:
  ```bash
  # Check recent merged PRs for unresolved threads
  gh pr list --state merged --limit 10 | while read pr_num _; do
    echo "Checking PR #$pr_num:"
    ./bin/gh-helper reviews fetch $pr_num --list-threads
  done
  ```
- **IMPORTANT**: Resolved threads without replies may contain valuable feedback that gets lost

### Enhanced Review Workflow
- **After addressing review feedback**: Always request another review and wait for it
  ```bash
  # After fixing all review comments
  ./bin/gh-helper reviews wait <PR> --request-review
  ```
- **Check comprehensive PR status**: Use detailed status to ensure merge readiness
  ```bash
  ./bin/gh-helper reviews wait <PR> --async --detailed
  # Check for: resolved threads, CI status, merge conflicts, required approvals
  ```
- **Request PR summary**: Get AI-generated summary for merge commits
  ```bash
  ./bin/gh-helper reviews wait <PR> --request-summary
  ```

```bash
# Example: Address feedback with commit reference
git commit -m "fix: address review feedback"
./bin/gh-helper threads reply PRRT_kwDONC6gMM5SU-GH --commit-hash $(git rev-parse HEAD) --message "Fixed as suggested" --resolve

# Example: Reply without code changes
./bin/gh-helper threads reply PRRT_kwDONC6gMM5SU-GH --message "This is intentional behavior for compatibility" --resolve
```

### Automated Review Management
- **Gemini Code Assist**: Provides automatic initial review but requires explicit request for follow-up reviews
- **Initial PR creation**: Gemini automatically reviews new PRs, no `--request-review` flag needed initially
- **No manual reviewer assignment needed**: Do NOT add gemini-code-assist as a reviewer using `gh pr edit --add-reviewer` - Gemini reviews PRs automatically
- **Follow-up reviews**: Use `--request-review` flag or comment `/gemini review` for re-review after significant changes
- **Important**: `--request-review` is only needed for subsequent reviews, not for newly created PRs

### Gemini Code Assist Review Structure
- **CRITICAL**: Gemini posts multiple reviews on initial PRs:
  1. **Summary**: Starts with `## Summary of Changes` (NOT a code review)
  2. **Code Review**: Starts with `## Code Review` (actual review with threads)
- **Both have state "COMMENTED"**: Cannot distinguish by state alone
- **Essential verification steps**:
  ```bash
  # After reviews arrive, ALWAYS check for threads
  ./bin/gh-helper reviews fetch <PR> --list-threads
  
  # If output is NOT empty, unresolved threads exist
  # Get full details
  ./bin/gh-helper reviews fetch <PR>
  ```
- **Common mistakes to avoid**:
  - Assuming Summary completion means review is done
  - Judging by preview text alone
  - Skipping `--list-threads` verification
- **Correct workflow**:
  1. Wait for reviews with `reviews wait`
  2. Check full review data with `reviews fetch <PR>`
  3. Verify `reviewThreads.unresolvedCount` is 0
  4. Or use `--list-threads` to list all unresolved threads
  5. Address all threads before proceeding

### Pull Request Merging
- **ALWAYS use Squash and Merge**: Never use regular merge or rebase merge
- **Merge body content**: In the squash commit body, provide a summary of all changes from the branch HEAD
- **Format**: Use clear bullet points summarizing what changed
- **Example workflow**:
  ```bash
  # After all reviews are resolved and checks pass
  gh pr merge <PR> --squash --body "$(cat <<'EOF'
  Summary of changes:
  - Removed deprecated reviews check command
  - Implemented async mode in reviews wait command
  - Fixed guidance messages to use correct command syntax
  - Enhanced error logging with slog
  - Improved documentation in CLAUDE.md
  EOF
  )"
  ```

## Installation and Usage

### Development
```bash
git clone https://github.com/apstndb/gh-dev-tools.git
cd gh-dev-tools
make build
```

### Production
```bash
go install github.com/apstndb/gh-dev-tools/gh-helper@latest
```

## Key Features

- **Unified Review Analysis**: Comprehensive feedback analysis preventing missed critical issues
- **Structured Output**: YAML/JSON output optimized for AI assistant consumption
- **Batch Operations**: Minimize GitHub API calls through unified GraphQL queries
- **Thread Management**: Complete review thread lifecycle management

## Important Notes

- **GitHub API**: Uses GitHub GraphQL API directly for optimal performance
- **Output Formats**: Supports both YAML (default) and JSON output
- **AI-First Design**: Interface optimized for AI assistant workflows
- **Minimal Dependencies**: Uses goccy/go-yaml, spf13/cobra, and golang.org/x/net

## GitHub GraphQL API Handling

### Core Principles

- **Null responses**: When PRs are deleted or permissions are missing, GraphQL returns null
- **Pointer types required**: Use pointer structs to handle null responses properly
- **@include directives**: Use for conditional field inclusion to optimize queries
- **Fragment reuse**: Define fragments for commonly used field sets to avoid repetition

### Examples

**Handling null responses**:
```go
// Correct: pointer allows nil check
PullRequest *struct {
    Number int `json:"number"`
    // ...
} `json:"pullRequest"`

if pr == nil {
    // Handle missing PR
    continue
}
```

**Using @include directives for conditional queries**:
```graphql
query($includeSubIssues: Boolean!, $includeDetails: Boolean!) {
  repository(owner: $owner, name: $repo) {
    issue(number: $number) {
      ...IssueFields
      trackedIssues(first: 100) @include(if: $includeSubIssues) {
        nodes {
          ...IssueFields
          body @include(if: $includeDetails)
          createdAt @include(if: $includeDetails)
          author @include(if: $includeDetails) { login }
        }
      }
    }
  }
}

fragment IssueFields on Issue {
  number
  title
  state
  url
}
```

**Benefits of this approach**:
- Single query definition handles multiple use cases
- Reduces network payload when features aren't needed
- Improves performance by fetching only required data
- Simplifies Go code by avoiding multiple query strings

### Using GraphQL Fragments

**Define reusable fragments in `graphql_fragments.go`**:
```go
const (
    IssueFieldsFragment = `
fragment IssueFields on Issue {
  number
  title
  state
  body
  url
  createdAt
  updatedAt
  labels(first: 20) {
    nodes {
      name
    }
  }
  assignees(first: 10) {
    nodes {
      login
    }
  }
}`

    SubIssueFieldsFragment = `
fragment SubIssueFields on Issue {
  id
  number
  title
  state
  closed
}`
)
```

**Use fragments in queries**:
```go
query := AllIssueFragments + `
query($owner: String!, $repo: String!, $number: Int!, $includeSub: Boolean!) {
    repository(owner: $owner, name: $repo) {
        issue(number: $number) {
            ...IssueFields
            subIssues(first: 100) @include(if: $includeSub) {
                totalCount
                nodes {
                    ...SubIssueFields
                }
            }
        }
    }
}`
```

**Benefits of fragments**:
- Reduces code duplication across queries
- Ensures consistent field selection
- Makes queries more maintainable
- Simplifies updates when field requirements change

## Additional Commands

### Release Notes Analysis
Analyze PRs for proper release notes categorization:
```bash
# Analyze by milestone
./bin/gh-helper releases analyze --milestone v0.19.0

# Analyze by date range
./bin/gh-helper releases analyze --since 2024-01-01 --until 2024-01-31

# Analyze by PR number range
./bin/gh-helper releases analyze --pr-range 10-20

# Output in markdown format
./bin/gh-helper releases analyze --milestone v0.19.0 --format markdown
```

This command helps identify:
- PRs missing classification labels (bug, enhancement, feature)
- PRs that should have 'ignore-for-release' label
- Inconsistent labeling patterns

### Issue Management
Create and manage issues with advanced features:
```bash
# Create issue with labels and assignee
./bin/gh-helper issues create --title "Add feature X" --body "Description" --label enhancement --assignee @me

# Create sub-issue linked to parent
./bin/gh-helper issues create --title "Subtask: Implement Y" --body "Details" --parent 123

# View issue with sub-issues
./bin/gh-helper issues show 248 --include-sub
./bin/gh-helper issues show 248 --include-sub --detailed  # Includes details for each sub-issue

# Manage parent-child relationships
./bin/gh-helper issues edit 456 --parent 123              # Add as sub-issue
./bin/gh-helper issues edit 456 --parent 789 --overwrite  # Move to different parent
./bin/gh-helper issues link-parent 456 --parent 123       # Deprecated: use edit command
```

This provides comprehensive issue management including:
- Viewing issues with sub-issue hierarchies and completion statistics
- Creating parent-child relationships between issues
- Moving sub-issues between different parents
- Tracking completion percentages for project management

For detailed usage examples and API documentation, see the README.md file.