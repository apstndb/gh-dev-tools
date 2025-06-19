# gh-dev-tools Makefile
# Generic GitHub development tools optimized for AI assistants

build:
	mkdir -p bin
	go build -o bin/gh-helper ./gh-helper

clean:
	rm -rf bin tmp
	go clean -testcache

test:
	go test ./...

test-verbose:
	go test -v ./...

test-coverage:
	mkdir -p tmp
	go test ./... -coverprofile=tmp/coverage.out

test-quick:
	go test -short ./...

lint:
	golangci-lint run

# Combined test and lint check (required before push)
check: test lint

# Combined test with coverage and lint check (for CI)
check-coverage: test-coverage lint

# Install to $GOPATH/bin
install:
	go install ./gh-helper

# Show development help
help:
	@echo "🛠️  Development Commands:"
	@echo "  make build            - Build gh-helper tool"
	@echo "  make test             - Run full test suite (required before push)"
	@echo "  make test-coverage    - Run tests with coverage profile (for CI)"
	@echo "  make test-quick       - Run quick tests (go test -short)"
	@echo "  make lint             - Run linter (required before push)"
	@echo "  make check            - Run test && lint (required before push)"
	@echo "  make check-coverage   - Run test-coverage && lint (for CI)"
	@echo "  make clean            - Clean build artifacts and test cache"
	@echo "  make install          - Install gh-helper to $GOPATH/bin"
	@echo ""
	@echo "🚀 Quick Start:"
	@echo "  ./bin/gh-helper reviews analyze <PR>     # Complete review analysis"
	@echo "  ./bin/gh-helper reviews fetch <PR>       # Fetch review data"
	@echo "  ./bin/gh-helper threads reply <ID>       # Reply to review thread"

.PHONY: build clean test test-verbose test-coverage test-quick lint check check-coverage install help