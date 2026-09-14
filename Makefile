SHELL := /bin/sh

GO ?= go
BINARY ?= macos-health
DIST_DIR ?= dist
DARWIN_MIN_VERSION ?= 12.0
GOVULNCHECK_VERSION ?= v1.6.0
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.DEFAULT_GOAL := check

.PHONY: build build-all build-arm64 build-amd64 check clean fmt fmt-check install release-check snapshot test test-race uninstall vet vuln

install: build
	@mkdir -p "$(DESTDIR)$(BINDIR)"
	install -m 0755 "$(DIST_DIR)/$(BINARY)" "$(DESTDIR)$(BINDIR)/$(BINARY)"

uninstall:
	rm -f "$(DESTDIR)$(BINDIR)/$(BINARY)"


build:
	@mkdir -p "$(DIST_DIR)"
	CGO_ENABLED=1 GOOS=darwin MACOSX_DEPLOYMENT_TARGET="$(DARWIN_MIN_VERSION)" \
		$(GO) build -trimpath -o "$(DIST_DIR)/$(BINARY)" .

build-all: build-arm64 build-amd64

build-arm64:
	@mkdir -p "$(DIST_DIR)"
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 CC=clang \
		MACOSX_DEPLOYMENT_TARGET="$(DARWIN_MIN_VERSION)" \
		CGO_CFLAGS="-arch arm64 -mmacosx-version-min=$(DARWIN_MIN_VERSION)" \
		CGO_LDFLAGS="-arch arm64 -mmacosx-version-min=$(DARWIN_MIN_VERSION)" \
		$(GO) build -trimpath -o "$(DIST_DIR)/$(BINARY)_darwin_arm64" .

build-amd64:
	@mkdir -p "$(DIST_DIR)"
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 CC=clang \
		MACOSX_DEPLOYMENT_TARGET="$(DARWIN_MIN_VERSION)" \
		CGO_CFLAGS="-arch x86_64 -mmacosx-version-min=$(DARWIN_MIN_VERSION)" \
		CGO_LDFLAGS="-arch x86_64 -mmacosx-version-min=$(DARWIN_MIN_VERSION)" \
		$(GO) build -trimpath -o "$(DIST_DIR)/$(BINARY)_darwin_amd64" .

fmt:
	$(GO) fmt ./...

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "The following Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	CGO_ENABLED=1 $(GO) vet ./...

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

test:
	CGO_ENABLED=1 $(GO) test -count=1 ./...

test-race:
	CGO_ENABLED=1 $(GO) test -race -count=1 ./...

check: fmt-check vet test-race build

release-check:
	goreleaser check

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf "$(DIST_DIR)"

.PHONY: bench demo
bench:
	$(GO) test ./internal/ui -run '^$$' -bench . -benchmem

demo: build
	./dist/$(BINARY) --demo
