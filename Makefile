
FILES = $(shell find . -type f -name '*.go' -not -path './tmp/*')

build:

test:
	go test -v -race ./...

lint:
	linter ./...
	go vet ./...
	go install ./...

gofmt:
	@gofmt -s -w $(FILES)
	@gofmt -r '&α{} -> new(α)' -w $(FILES)
	@impsort . -p github.com/altipla-consulting/linter
