.PHONY: test build vet clean

BINARY := llmrelay
GO_BUILD_FLAGS := -trimpath -buildvcs=false

test:
	go test ./...

vet:
	go vet ./...

build:
	go build $(GO_BUILD_FLAGS) -o ./dist/$(BINARY) ./cmd/llmrelay

clean:
	rm -rf ./dist coverage.out
