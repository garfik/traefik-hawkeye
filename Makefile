.PHONY: build test clean lint

build:
	@echo "Building traefik-hawkeye..."
	@go build -v ./...

test:
	@echo "Running all tests..."
	@go test -v -timeout 30s ./...

clean:
	@echo "Cleaning..."
	@go clean ./...

lint:
	@echo "Running linter..."
	@go vet ./...
