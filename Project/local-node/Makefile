.PHONY: run build test simulate tidy

run:
	go run ./cmd/node

build:
	go build -o bin/node ./cmd/node

test:
	go test -v ./...

simulate:
	go run ./cmd/simulate

tidy:
	go mod tidy
