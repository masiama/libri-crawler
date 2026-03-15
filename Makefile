build:
	go generate ./ent
	go mod tidy
	go build -mod=readonly -o bin/crawler ./cmd/crawler

run:
	go run ./cmd/crawler

clean:
	rm -rf bin/

check:
	go mod tidy
	go fmt ./...
	go vet ./...
