# gh-dev-tools Gemini Code Review Style Guide

## Repository Context

gh-dev-tools is a Go repository for GitHub development tooling. Prefer findings
that identify concrete correctness, security, reliability, maintainability, or
workflow consistency risks in this repository.

## Review Calibration

- Do not flag a dependency or tool version as invalid solely because it is newer
  than the model's training data. Modern tools may have releases that postdate
  the model's knowledge cutoff.
- If a version is specified in repository tooling such as `mise.toml`, treat the
  repository's validation commands as authoritative when they prove the version
  resolves and runs. For this repository, `make check` runs `mise install` and
  `mise exec -- golangci-lint`.
- Do not recommend downgrading `golangci-lint` from a valid v2 release to an
  older v1 release unless repository validation fails or the change is directly
  required by compatibility evidence in the PR.
- Avoid comments that only restate generic style preferences when the existing
  code follows a deliberate local pattern and has focused test coverage.

## Tooling Expectations

- Local and CI linting should use the same repo-pinned toolchain. Prefer
  `make lint`/`make check` over separate ad hoc `golangci-lint` invocations.
- `mise.toml` is the source of truth for developer tool versions used by
  `make lint`.
- `gh-helper` may use both REST and GraphQL GitHub APIs. Prefer feedback that
  preserves explicit backend choice, rate-limit awareness, and structured output.

## GitHub Review Workflow

- For `/gemini review` slash-command triggers, a top-level PR conversation
  comment created through the REST issue-comments API is acceptable and preferred
  when it avoids unnecessary GraphQL budget.
- For thread reply and resolve operations, GraphQL is expected because GitHub's
  review-thread operations are GraphQL-oriented.
