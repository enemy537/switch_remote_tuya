GO ?= go

.PHONY: build test run

build:
	rm -rf build && mkdir -p build
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build -tags lambda.norpc -trimpath -ldflags "-s -w" -o build/bootstrap ./cmd/api
	cd build && zip -q -X api.zip bootstrap && rm bootstrap

test:
	$(GO) vet ./...
	$(GO) test ./...

run:
	$(GO) run ./cmd/api
