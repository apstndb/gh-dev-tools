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
./bin/gh-helper --help  # Show available commands
./bin/gh-helper reviews analyze <PR>  # Complete review analysis
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

For detailed usage examples and API documentation, see the README.md file.