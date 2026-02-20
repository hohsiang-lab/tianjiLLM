TAILWIND := ./bin/tailwindcss

.PHONY: build test lint generate check docker run clean templ-generate tailwind-build ui ui-dev tools

tools:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/templui/templui/cmd/templui@latest

templ-generate:
	templ generate ./internal/ui/...

tailwind-build:
	$(TAILWIND) -i internal/ui/input.css -o internal/ui/assets/css/output.css --minify

ui: templ-generate tailwind-build

build: ui
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
	rm -rf bin/tianji coverage.out

ui-dev:
	templ generate --watch --proxy="http://localhost:4000" &
	$(TAILWIND) -i internal/ui/input.css -o internal/ui/assets/css/output.css --watch &
