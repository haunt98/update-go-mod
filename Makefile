.PHONY: all test-color lint

all: test-color lint
	go mod tidy

test-color:
	go install github.com/haunt98/go-test-color@latest
	go-test-color -race -failfast ./...

lint:
	golangci-lint run ./...
