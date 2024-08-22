all: tidy format test-color lint build clean

tidy:
    go mod tidy

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
    go build ./cmd/update-go-mod

clean:
    rm -rf update-go-mod
