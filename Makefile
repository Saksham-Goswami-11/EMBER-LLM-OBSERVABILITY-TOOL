.PHONY: build web go-build run dev test lint docker up down logs rotate-key demo demo-python clean

build: web go-build ## Build the dashboard, then the Go binary that embeds it.

web: ## Build the dashboard into web/dist.
	cd web && npm install && npm run build

go-build: ## Build the ember binary (needs web/dist already built).
	go build -o bin/ember ./cmd/ember

run: build ## Build everything and run it.
	./bin/ember

dev: ## Print the two commands to run for hot-reload development.
	@echo "Terminal 1: cd web && npm run dev        (dashboard on :5173, proxies /api and /v1 to :8080)"
	@echo "Terminal 2: go run ./cmd/ember            (backend on :8080)"

test: ## Run the Go test suite.
	go test ./...

lint: ## Vet the Go code and type-check the frontend.
	go vet ./...
	cd web && npx tsc --noEmit

docker: ## Build the production Docker image.
	docker build -t ember:local .

up: ## Start Ember with docker compose (copies .env from the example if missing).
	@test -f .env || { cp .env.example .env; echo "Created .env — edit EMBER_API_KEY before exposing this."; }
	docker compose up -d --build
	@echo "Ember is at http://localhost:$${EMBER_PORT:-8080}"

down: ## Stop the compose stack (keeps the data volume).
	docker compose down

logs: ## Follow the compose logs.
	docker compose logs -f

rotate-key: ## Issue a new API key without losing existing traces.
	docker compose run --rm ember -rotate-key

demo: ## Send sample OTel GenAI traces to a running ember (needs EMBER_API_KEY).
	cd examples/demo-agent && go run .

demo-python: ## Same, from the Python example (needs EMBER_API_KEY).
	cd examples/python-agent && python3 -m pip install -qr requirements.txt && python3 agent.py

clean:
	rm -rf bin web/dist
	mkdir -p web/dist && touch web/dist/.gitkeep
