.PHONY: all test-color lint format build clean

all:
	go mod tidy
	$(MAKE) format
	$(MAKE) test-color
	$(MAKE) lint
	$(MAKE) build
	$(MAKE) clean

test-color:
	# go install github.com/haunt98/go-test-color@latest
	go-test-color -race -failfast ./...

lint:
	golangci-lint run ./...

format:
	# go install github.com/haunt98/gofimports/cmd/gofimports@latest
	# go install mvdan.cc/gofumpt@latest
	gofimports -w --company github.com/make-go-great,github.com/haunt98 .
	gofumpt -w -extra .

build:
	$(MAKE) clean
	go build ./cmd/update-go-mod

clean:
	rm -rf update-go-mod
