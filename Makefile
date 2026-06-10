.PHONY: test vet build build-local build-linux-amd64 build-linux-arm64 build-windows-amd64 build-darwin-amd64 build-darwin-arm64 clean

VERSION ?= v0.0.0
COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?=
BUILD_DATE_FLAG := $(if $(BUILD_DATE),-date $(BUILD_DATE),)
BUILD := go run ./scripts/build-release.go -version $(VERSION) -commit $(COMMIT) $(BUILD_DATE_FLAG)

test:
	go test ./...

vet:
	go vet ./...

build:
	$(BUILD)

build-local:
	$(BUILD) -local

build-linux-amd64:
	$(BUILD) -target linux/amd64

build-linux-arm64:
	$(BUILD) -target linux/arm64

build-windows-amd64:
	$(BUILD) -target windows/amd64

build-darwin-amd64:
	$(BUILD) -target darwin/amd64

build-darwin-arm64:
	$(BUILD) -target darwin/arm64

clean:
	go run ./scripts/build-release.go -clean
