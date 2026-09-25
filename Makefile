TAILWIND := ./bin/tailwindcss
CODEX_CLIENT_VERSION := $(shell tr -d '[:space:]' < .codex-client-version)
CODEX_VERSION_LDFLAGS := -X github.com/praxisllmlab/tianjiLLM/internal/config.DefaultOpenAICodexBackendClientVersion=$(CODEX_CLIENT_VERSION)

.PHONY: build test lint generate check docker run clean templ-generate tailwind-build ui ui-dev tools dev e2e e2e-headed playwright-install hooks

hooks:
	@go tool lefthook install

tailwind-install:
	@mkdir -p bin
	@if [ ! -f $(TAILWIND) ]; then \
		ARCH=$$(uname -m); OS=$$(uname -s); \
		case "$$OS" in Darwin) OS=macos;; Linux) OS=linux;; esac; \
		case "$$ARCH" in arm64|aarch64) ARCH=arm64;; x86_64|amd64) ARCH=x64;; esac; \
		URL="https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-$$OS-$$ARCH"; \
		echo "Downloading tailwindcss from $$URL"; \
		curl -sL "$$URL" -o $(TAILWIND) && chmod +x $(TAILWIND); \
	fi

tools: tailwind-install
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/templui/templui/cmd/templui@latest

templ-generate:
	templ generate ./internal/ui/...

tailwind-build:
	$(TAILWIND) -i internal/ui/input.css -o internal/ui/assets/css/output.css --minify

ui: templ-generate tailwind-build

build: hooks ui
	go build -ldflags "$(CODEX_VERSION_LDFLAGS)" -o bin/tianji ./cmd/tianji

test:
	go test -ldflags "$(CODEX_VERSION_LDFLAGS)" -race -cover ./...

lint:
	go tool golangci-lint run

generate:
	sqlc generate

check: hooks lint test build

docker:
	docker build -t tianjiLLM .

run:
	go run -ldflags "$(CODEX_VERSION_LDFLAGS)" ./cmd/tianji --config proxy_config.yaml

clean:
	rm -rf bin/tianji coverage.out

dev:
	wgo -file .go -file .templ -file .css -xfile _templ.go -xfile .sql.go -xdir test -xdir vendor -xdir specs -xdir .git \
		templ generate ./internal/ui/... \
		:: ./bin/tailwindcss -i internal/ui/input.css -o internal/ui/assets/css/output.css --minify \
		:: go run -ldflags "$(CODEX_VERSION_LDFLAGS)" ./cmd/tianji --config proxy_config.yaml

ui-dev:
	templ generate --watch --proxy="http://localhost:4000" &
	$(TAILWIND) -i internal/ui/input.css -o internal/ui/assets/css/output.css --watch &

playwright-install:
	go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6100.0 install --with-deps chromium

e2e: ui playwright-install
	E2E_DATABASE_URL="postgres://tianji:tianji@localhost:5433/tianji_e2e?sslmode=disable" \
		go test -ldflags "$(CODEX_VERSION_LDFLAGS)" -tags e2e -count=1 -v -timeout 5m ./test/e2e/...

e2e-headed: ui playwright-install
	E2E_HEADLESS=false E2E_DATABASE_URL="postgres://tianji:tianji@localhost:5433/tianji_e2e?sslmode=disable" \
		go test -ldflags "$(CODEX_VERSION_LDFLAGS)" -tags e2e -count=1 -v -timeout 5m ./test/e2e/...
