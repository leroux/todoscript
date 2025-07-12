.PHONY: build test lint clean install-tools ci check fmt vet

# Default target
all: lint test build

# CI target for automated builds
ci: fmt vet lint test build

build: todoscript

todoscript: *.go
	go build -o todoscript

test:
	go test -v -race ./...

lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Run all checks
check: fmt vet lint test

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

clean:
	rm -f todoscript coverage.out coverage.html
	rm -rf bin/ dist/

# Run with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Show coverage
coverage: test-coverage
	go tool cover -func=coverage.out

# Security scan
security:
	govulncheck ./...

# Tidy dependencies
tidy:
	go mod tidy
	go mod verify

# Release build for multiple platforms
release:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/todoscript-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/todoscript-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/todoscript-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/todoscript-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/todoscript-windows-amd64.exe .
