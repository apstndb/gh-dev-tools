# gh-dev-tools

Generic GitHub development tools optimized for AI assistants.

## Requirements

- Go 1.24 or later

## Installation

```bash
go install github.com/apstndb/gh-dev-tools/gh-helper@latest
```

## Tools

### gh-helper

GitHub operations with structured YAML/JSON output optimized for AI assistant workflows.

```bash
gh-helper --help
```

**Key Features:**
- Unified review and thread analysis
- Structured YAML/JSON output
- Batch operations to minimize API calls
- AI-friendly interface design

**Common Usage:**
```bash
# Complete review analysis
gh-helper reviews analyze <PR>

# Fetch review data with threads
gh-helper reviews fetch <PR>

# Thread operations
gh-helper threads reply <THREAD_ID> --message "Fixed as suggested"

# Issue management with sub-issues
gh-helper issues show 248 --include-sub --detailed
gh-helper issues edit 456 --parent 123
gh-helper issues create --title "Subtask" --body "Details" --parent 123

# Get GraphQL node IDs
gh-helper node-id issue 248
gh-helper node-id pr 312
gh-helper node-id --batch "issue:123,pr:456,issue:789"
```

## Development

This repository follows the same development practices as spanner-mycli:
- **Never push directly to main branch** - always use Pull Requests
- **Never commit directly to main branch** - always use feature branches
- All changes must go through PR review process

See [spanner-mycli CLAUDE.md](https://github.com/apstndb/spanner-mycli/blob/main/CLAUDE.md) for detailed development guidelines.

## Design Philosophy

- **AI-First**: Structured output and clear patterns for AI assistant integration
- **GitHub GraphQL**: Direct use of GitHub GraphQL API types without conversion
- **Unified Output**: Single library (goccy/go-yaml) for both JSON and YAML formats
- **Batch Operations**: Minimize API calls through unified GraphQL queries