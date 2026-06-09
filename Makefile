.PHONY: test build vet clean install

BINARY := llmrelay
GO_BUILD_FLAGS := -trimpath -buildvcs=false
GOBIN ?= $(HOME)/.local/bin

test:
	go test ./...

vet:
	go vet ./...

build:
	go build $(GO_BUILD_FLAGS) -o ./dist/$(BINARY) ./cmd/llmrelay

install:
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(GOBIN)' | Out-Null; go build $(GO_BUILD_FLAGS) -o '$(GOBIN)\$(BINARY).exe' ./cmd/llmrelay"

clean:
	rm -rf ./dist coverage.out
