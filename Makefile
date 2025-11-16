.PHONY: build test clean lint default yaegi_test install-hooks

export GO111MODULE=on

default: lint test

build:
	@echo "Building traefik-hawkeye..."
	@go build -v ./...

test:
	@echo "Running all tests..."
	@go test -v -cover -timeout 30s ./...

clean:
	@echo "Cleaning..."
	@go clean ./...

lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	@golangci-lint run

yaegi_test:
	@echo "Testing yaegi compatibility..."
	@which yaegi > /dev/null || (echo "yaegi not found. Install with: go install github.com/traefik/yaegi/cmd/yaegi@latest" && exit 1)
	@yaegi test -v .

install-hooks:
	@echo "Installing git hooks..."
	@mkdir -p .git/hooks
	@cp .githooks/pre-push .git/hooks/pre-push
	@chmod +x .git/hooks/pre-push
	@echo "✅ Pre-push hook installed successfully!"
	@echo "   Hook will run 'make default' (lint + test) and 'make yaegi_test' before pushing to main branch."
