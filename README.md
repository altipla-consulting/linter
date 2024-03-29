
# linter

Opinionated linter for Go source code.

> **WARNING:** It uses [revive](https://github.com/mgechev/revive) underneath. We recommend using revive directly instead of our internal customization on top of it.


## Install

```shell
go install github.com/altipla-consulting/linter@latest
```


## Usage

Add it to the Makefile `lint` rule:

```makefile
lint:
	go install ./...
	go vet ./...
	linter ./...
```


## Contributing

You can make pull requests or create issues in GitHub. Any code you send should be formatted using `make gofmt`.


## License

[MIT License](LICENSE)
