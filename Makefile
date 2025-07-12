.PHONY: build test lint clean install-tools

# Default target
all: lint test build

build: todoscript

todoscript: *.go
	go build -o todoscript

test:
	go test -v -race ./...

lint:
	golangci-lint run

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

clean:
	rm -f todoscript

# Run with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Tidy dependencies
tidy:
	go mod tidy
	go mod verify
