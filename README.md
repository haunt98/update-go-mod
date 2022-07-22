# update-go-mod

Only upgrade specific subset not all modules.

## Install

Should use Go version `>= 1.16`:

```sh
go install github.com/haunt98/update-go-mod/cmd/update-go-mod@latest
```

## Usage

Create local file `.deps` or use url depends on your use case:

```sh
# Default read from .deps
update-go-mod

# Read from URL
update-go-mod --deps-url "https://example.txt"

# Don't do anythin
update-go-mod --dry-run
```

## Thanks

- [Managing dependencies](https://go.dev/doc/modules/managing-dependencies)
