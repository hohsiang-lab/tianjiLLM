.PHONY: build test lint generate check docker run clean

build:
	go build -o bin/tianji ./cmd/tianji

test:
	go test -race -cover ./...

lint:
	golangci-lint run

generate:
	sqlc generate

check: lint test build

docker:
	docker build -t tianjiLLM .

run:
	go run ./cmd/tianji --config proxy_config.yaml

clean:
	rm -rf bin/ coverage.out
