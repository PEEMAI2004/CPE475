.PHONY: run build test simulate tidy enroll

run:
	go run ./cmd/node

build:
	go build -o bin/node ./cmd/node
	go build -o bin/enroll ./cmd/enroll

test:
	go test -v ./...

simulate:
	go run ./cmd/simulate

enroll:
	go run ./cmd/enroll

tidy:
	go mod tidy
