.PHONY: build test test-verbose test-cover bench clean install help lint

# Build the binary
build:
	go build -o lightfold ./cmd/lightfold

# Run tests
test:
	go test ./...

# Run tests with verbose output
test-verbose:
	go test -v ./...

# Run tests with coverage
test-cover:
	go test ./... -cover

# Run benchmark tests
bench:
	go test ./... -bench=. -benchmem

# Run all tests (verbose + coverage + bench)
test-all: test-verbose test-cover bench

# Clean build artifacts
clean:
	rm -f lightfold
	go clean

# Install locally
install: build
	cp lightfold /usr/local/bin/

# Format Go code
lint:
	gofmt -w .

# Show help
help:
	@echo "Available targets:"
	@echo "  build       - Build the lightfold binary"
	@echo "  test        - Run tests"
	@echo "  test-verbose- Run tests with verbose output"
	@echo "  test-cover  - Run tests with coverage analysis"
	@echo "  bench       - Run benchmark tests"
	@echo "  test-all    - Run all tests (verbose + coverage + bench)"
	@echo "  clean       - Clean build artifacts"
	@echo "  install     - Install lightfold to /usr/local/bin"
	@echo "  lint        - Format Go code using gofmt"
	@echo "  help        - Show this help message"