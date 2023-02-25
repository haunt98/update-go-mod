.PHONY: all test-color lint

all:
	$(MAKE) test-color
	$(MAKE) lint
	$(MAKE) format
	$(MAKE) build
	$(MAKE) clean
	go mod tidy

test-color:
	go install github.com/haunt98/go-test-color@latest
	go-test-color -race -failfast ./...

lint:
	golangci-lint run ./...

format:
	go install github.com/haunt98/gofimports/cmd/gofimports@latest
	go install mvdan.cc/gofumpt@latest
	go install mvdan.cc/sh/v3/cmd/shfmt@latest
	gofimports -w --company github.com/make-go-great,github.com/haunt98 .
	gofumpt -w -extra .

build:
	$(MAKE) clean
	go build ./cmd/update-go-mod

clean:
	rm -rf update-go-mod
