build:
	go mod tidy
	go build -mod=readonly -o bin/crawler ./cmd/crawler

run:
	go run ./cmd/crawler

clean:
	rm -rf bin/

ci-check:
	go mod tidy
	go fmt ./...
	go vet ./...

lint:
	golangci-lint run

check: ci-check lint
