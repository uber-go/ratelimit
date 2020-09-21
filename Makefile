#Directory to put `go install`ed binaries in.
export GOBIN ?= $(shell pwd)/bin

GO_FILES := $(shell \
	find . '(' -path '*/.*' -o -path './vendor' ')' -prune \
	-o -name '*.go' -print | cut -b3-)

.PHONY: bench
bench:
	go test -bench=. ./...

.PHONY: build
build:
	go build ./...

.PHONY: cover
cover:
	go test -coverprofile=cover.out -coverpkg=./... -v ./...
	go tool cover -html=cover.out -o cover.html

.PHONY: gofmt
gofmt:
	$(eval FMT_LOG := $(shell mktemp -t gofmt.XXXXX))
	@gofmt -e -s -l $(GO_FILES) > $(FMT_LOG) || true
	@[ ! -s "$(FMT_LOG)" ] || (echo "gofmt failed:" | cat - $(FMT_LOG) && false)

.PHONY: golint
golint:
	@cd tools && go install golang.org/x/lint/golint
	@$(GOBIN)/golint ./...

.PHONY: lint
lint: gofmt golint staticcheck

.PHONY: staticcheck
staticcheck:
	@cd tools && go install honnef.co/go/tools/cmd/staticcheck
	@$(GOBIN)/staticcheck ./...

.PHONY: test
test:
	go test -race ./...
