APP := acli-plus
VERSION ?= 0.1.0

.PHONY: build install release test fmt vet clean

## build: build the binary for this machine into ./bin
build:
	CGO_ENABLED=0 go build -trimpath \
		-ldflags "-s -w -X acli-plus/internal/cmd.version=$(VERSION)" \
		-o bin/$(APP) .

## install: install onto PATH (prebuilt binary if present, else builds)
install:
	./install.sh

## release: cross-compile all platforms into ./dist
release:
	VERSION=$(VERSION) ./scripts/build.sh

## test: run the test suite
test:
	go test ./...

## fmt: format all Go files
fmt:
	gofmt -w .

## vet: run go vet
vet:
	go vet ./...

## clean: remove build output
clean:
	rm -rf bin dist
