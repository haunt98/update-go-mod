# update-go-mod

[![Go](https://github.com/haunt98/update-go-mod/workflows/Go/badge.svg?branch=main)](https://github.com/haunt98/update-go-mod/actions)
[![Latest Version](https://img.shields.io/github/v/tag/haunt98/update-go-mod)](https://github.com/haunt98/update-go-mod/tags)

Only upgrade specific subset not all modules.

## Install

Should use Go version `>= 1.16`:

```sh
go install github.com/haunt98/update-go-mod/cmd/update-go-mod@latest
```

## Usage

Create local file `.deps` ([example](.deps)) or use url depends on your use
case:

```sh
# Default read from .deps
update-go-mod

# Read from URL
update-go-mod --deps-url "https://example.txt"

# Don't do anything
update-go-mod --dry-run

# Take a look
# Require GitHub access token in ~/.netrc
update-go-mod overlook
```

## Thanks

- [Managing dependencies](https://go.dev/doc/modules/managing-dependencies)
- [Bash one liners](https://blog.fredrb.com/2023/08/13/bash-one-liner-gomod/)
