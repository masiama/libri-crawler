VERSION := $(shell echo $$(git describe --tags --abbrev=0 2>/dev/null || echo dev)-$$(git rev-parse --short HEAD 2>/dev/null))
LDFLAGS := -X main.version=$(VERSION)

build:
	go mod tidy
	go build -mod=readonly -ldflags "$(LDFLAGS)" -o bin/crawler ./cmd/crawler

run:
	go run -ldflags "$(LDFLAGS)" ./cmd/crawler

clean:
	rm -rf bin/

ci-check:
	go mod tidy
	go fmt ./...
	go vet ./...

lint:
	golangci-lint run

check: ci-check lint
