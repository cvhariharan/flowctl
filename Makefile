BINARY_NAME := flowctl
VERSION := $(shell git describe --tags --exact-match 2>/dev/null || echo "v0.0.0-dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X 'github.com/cvhariharan/flowctl/cmd.version=$(VERSION)' -X 'github.com/cvhariharan/flowctl/cmd.commit=$(COMMIT)' -X 'github.com/cvhariharan/flowctl/cmd.date=$(DATE)'

GO_FILES := $(shell find . -name '*.go' -type f)
SITE_SRC := $(shell find site/src site/static -type f 2>/dev/null) site/package.json site/svelte.config.js site/vite.config.ts site/tsconfig.json

.PHONY: build clean build-site

build: $(BINARY_NAME)

$(BINARY_NAME): $(GO_FILES) go.mod go.sum site/build
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)

build-site: site/build

site/build: $(SITE_SRC) site/node_modules
	cd site && npm run build

site/node_modules: site/package.json site/package-lock.json
	cd site && npm install

clean:
	rm -rf site/build
	rm -f $(BINARY_NAME)
