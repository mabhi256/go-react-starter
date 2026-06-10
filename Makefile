## XYZMed Makefile: common dev tasks

.PHONY: up down migrate new-migration api worker test openapi frontend

# Start full dev stack (postgres, valkey, mailhog, lgtm + api + worker + frontend)
up:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build -d

# Stop all containers
down:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down

# Apply database migrations
migrate:
	cd backend && go run ./cmd/migrate

# Create a new blank migration file: make new-migration NAME=add_foo
new-migration:
	@if [ -z "$(NAME)" ]; then echo "Usage: make new-migration NAME=description"; exit 1; fi
	@last=$$(ls backend/db/migrations/*.sql 2>/dev/null | sort | tail -1); \
	if [ -z "$$last" ]; then num=0; else num=$$(basename "$$last" | cut -c1-3 | sed 's/^0*//'); fi; \
	next=$$(printf "%03d" $$((num + 1))); \
	file="backend/db/migrations/$${next}_$(NAME).sql"; \
	printf '-- write your up migration here\n\n---- create above / drop below ----\n\n-- write your down migration here\n' > "$$file"; \
	echo "Created $$file"

# Run API server locally (without docker)
api:
	cd backend && go run ./cmd/api

# Run worker locally (without docker)
worker:
	cd backend && go run ./cmd/worker

# Run integration tests (requires docker for testcontainers)
test:
	cd backend && go test ./...

# Dump generated OpenAPI spec to backend/api/openapi.yaml
# Builds the server, calls /openapi.yaml, saves to file.
openapi:
	cd backend && go run ./cmd/api &; sleep 2; \
	curl -s http://localhost:8080/openapi.yaml > api/openapi.yaml; \
	kill %1

# Install frontend deps + run dev server
frontend:
	cd frontend && bun install && bun run dev
