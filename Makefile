BINARY := gbdata
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)
GO ?= go

.PHONY: build test vet mod-verify fmt-check diff-check check clean

build:
	mkdir -p bin
	GOTOOLCHAIN=auto $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/gbdata

test:
	GOTOOLCHAIN=auto $(GO) test -race -shuffle=on ./...

vet:
	GOTOOLCHAIN=auto $(GO) vet ./...

mod-verify:
	GOTOOLCHAIN=auto $(GO) mod verify

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -type f))" || \
		(echo "Go files need formatting; run gofmt" >&2; gofmt -l $$(find . -name '*.go' -type f); exit 1)

diff-check:
	git diff --check

check: mod-verify fmt-check test vet diff-check

clean:
	rm -rf bin dist
