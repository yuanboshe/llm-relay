.PHONY: test build vet clean

BINARY := llm-relay
GO_BUILD_FLAGS := -trimpath -buildvcs=false

test:
	go test ./...

vet:
	go vet ./...

build:
	go build $(GO_BUILD_FLAGS) -o ./dist/$(BINARY) ./cmd/llm-relay

clean:
	rm -rf ./dist coverage.out

