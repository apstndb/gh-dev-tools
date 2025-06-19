# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**IMPORTANT**: This file must be written entirely in English. Do not use Japanese or any other languages in CLAUDE.md.

## Project Overview

gh-dev-tools provides generic GitHub development tools optimized for AI assistants. This is a standalone repository extracted from spanner-mycli dev-tools to create reusable GitHub operations tools.

## 🚨 CRITICAL REQUIREMENTS

**Before ANY push to the repository**:
1. **Always run `make check`** - runs test && lint (required for quality assurance)
2. **Resolve conflicts with origin/main** - ensure branch can merge cleanly to avoid integration issues
3. **Never push directly to main branch** - always use Pull Requests
4. **Never commit directly to main branch** - always use feature branches

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
```

## Core Architecture

### gh-helper
- **Main package**: All functionality merged into single main package
- **Unified output**: JSON/YAML using goccy/go-yaml library
- **GitHub GraphQL**: Direct use of GitHub API types without conversion
- **AI-optimized**: Structured output for assistant workflows

## Development Workflow

### Branch Management
- **Feature branches**: Always create feature branches for development
- **Pull Requests**: All changes must go through PR review
- **No direct commits**: Never commit directly to main branch

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
- **CRITICAL: Push commits BEFORE replying to threads** - GitHub needs the commit to exist for proper linking
- **For threads requiring code changes**:
  1. Make the necessary changes and commit
  2. **Push the commit to GitHub first**: `git push origin HEAD`
  3. Reply with commit hash and resolve: `./bin/gh-helper threads reply <THREAD_ID> --commit-hash <HASH> --message "Fixed as suggested" --resolve`
- **For threads not requiring changes**:
  1. Reply with explanation and resolve: `./bin/gh-helper threads reply <THREAD_ID> --message "Explanation here" --resolve`
- **Batch resolve multiple threads**: `./bin/gh-helper threads resolve <THREAD_ID1> <THREAD_ID2> <THREAD_ID3>`

```bash
# Example: Address feedback with commit reference
git commit -m "fix: address review feedback"
./bin/gh-helper threads reply PRRT_kwDONC6gMM5SU-GH --commit-hash $(git rev-parse HEAD) --message "Fixed as suggested" --resolve

# Example: Reply without code changes
./bin/gh-helper threads reply PRRT_kwDONC6gMM5SU-GH --message "This is intentional behavior for compatibility" --resolve
```

### Automated Review Management
- **Gemini Code Assist**: Provides automatic initial review but requires explicit request for follow-up reviews
- **Request additional reviews**: Comment `/gemini review` on PR for re-review after significant changes
- **Initial review only**: After first automated review, no additional reviews come automatically

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

For detailed usage examples and API documentation, see the README.md file.