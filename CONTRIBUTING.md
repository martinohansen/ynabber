# Contributing

Thank you for considering contributing to Ynabber. This is a Go project optional
Docker image as release method.

## Development Workflow

Use these commands before opening a pull request:

```sh
go test -v ./...
go build ./cmd/ynabber
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Format any touched Go files with `gofmt`. For example:

```sh
gofmt -w cmd/ynabber/main.go ynabber.go
```

If you change configuration structs or envconfig tags, regenerate
`CONFIGURATION.md`:

```sh
go generate ./...
```

`CONFIGURATION.md` is generated. Do not edit it manually.

## Go

If you are new to Go make sure to follow [Effective
Go](https://go.dev/doc/effective_go) and [Go Style
Guide](https://google.github.io/styleguide/go/guide) if you want to dig even
deeper.
