GO ?= go
DB_PATH ?= data/chess.db
PORT ?= 8080

.PHONY: tools run test templ-generate tailwind-watch clean

tools: ## install templ codegen (stockfish + tailwindcss need OS package, see PROGRESS.md)
	$(GO) install github.com/a-h/templ/cmd/templ@latest

run: ## generate templates then run server
	(if command -v templ >/dev/null 2>&1; then templ generate; else echo "templ missing: run 'make tools' (adds $(HOME)/go/bin to PATH)"; fi)
	mkdir -p data
	DB_PATH=$(DB_PATH) PORT=$(PORT) $(GO) run ./cmd/server

test:
	$(GO) test ./...

templ-generate:
	templ generate

tailwind-watch: ## needs tailwindcss binary; MVP can use CDN instead
	tailwindcss -i web/static/in.css -o web/static/out.css --watch

clean:
	rm -f data/chess.db
