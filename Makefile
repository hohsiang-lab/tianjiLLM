TAILWIND := ./bin/tailwindcss

.PHONY: build test lint generate check docker run clean templ-generate tailwind-build ui ui-dev tools dev e2e e2e-headed playwright-install hooks

hooks:
	@command -v lefthook >/dev/null 2>&1 || go install github.com/evilmartians/lefthook@latest
	@lefthook install

tools:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/templui/templui/cmd/templui@latest

templ-generate:
	templ generate ./internal/ui/...

tailwind-build:
	$(TAILWIND) -i internal/ui/input.css -o internal/ui/assets/css/output.css --minify

ui: templ-generate tailwind-build

build: hooks ui
	go build -o bin/tianji ./cmd/tianji

test:
	go test -race -cover ./...

lint:
	golangci-lint run

generate:
	sqlc generate

check: hooks lint test build

docker:
	docker build -t tianjiLLM .

run:
	go run ./cmd/tianji --config proxy_config.yaml

clean:
	rm -rf bin/tianji coverage.out

dev:
	wgo -file .go -file .templ -file .css -xfile _templ.go -xfile .sql.go -xdir test -xdir vendor -xdir specs -xdir .git \
		templ generate ./internal/ui/... \
		:: ./bin/tailwindcss -i internal/ui/input.css -o internal/ui/assets/css/output.css --minify \
		:: go run ./cmd/tianji --config proxy_config.yaml

ui-dev:
	templ generate --watch --proxy="http://localhost:4000" &
	$(TAILWIND) -i internal/ui/input.css -o internal/ui/assets/css/output.css --watch &

playwright-install:
	go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium

e2e: ui playwright-install
	E2E_DATABASE_URL="postgres://tianji:tianji@localhost:5433/tianji_e2e?sslmode=disable" \
		go test -tags e2e -count=1 -v -timeout 5m ./test/e2e/...

e2e-headed: ui playwright-install
	E2E_HEADLESS=false E2E_DATABASE_URL="postgres://tianji:tianji@localhost:5433/tianji_e2e?sslmode=disable" \
		go test -tags e2e -count=1 -v -timeout 5m ./test/e2e/...
