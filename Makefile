.PHONY: test build vet clean

GO_BUILD_FLAGS := -trimpath -buildvcs=false
VERSION ?= v0.1.0
COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= unknown
LDFLAGS := -X github.com/yuanboshe/llm-relay/internal/cmd.Version=$(VERSION) -X github.com/yuanboshe/llm-relay/internal/cmd.Commit=$(COMMIT) -X github.com/yuanboshe/llm-relay/internal/cmd.BuildDate=$(BUILD_DATE)
HOST_GOOS := $(shell go env GOOS)
ifeq ($(HOST_GOOS),windows)
BINARY := llmrelay.exe
MKDIR_DIST := if not exist dist mkdir dist
else
BINARY := llmrelay
MKDIR_DIST := mkdir -p dist
endif

test:
	go test ./...

vet:
	go vet ./...

build:
	$(MKDIR_DIST)
	go build $(GO_BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o ./dist/$(BINARY) ./cmd/llmrelay

clean:
	rm -rf ./dist coverage.out
