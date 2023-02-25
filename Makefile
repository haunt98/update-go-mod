.PHONY: all test-color lint

all:
	$(MAKE) test-color
	$(MAKE) lint
	go mod tidy

test-color:
	go install github.com/haunt98/go-test-color@latest
	go-test-color -race -failfast ./...

lint:
	golangci-lint run ./...
