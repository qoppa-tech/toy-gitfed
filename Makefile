test:
	@go test ./... -v -cover

build:
	@go build -o ./bin/main ./cmd/main.go

clean:
	@rm -f ./bin/main

lint:
	@go vet ./...

build-image:
	@docker build -t gitfed:latest .

compose-up:
	@docker compose up -d --build

compose-down:
	@docker compose down

sqlc:
	@sqlc generate

migrate-up:
	@echo "Run: psql -d gitfed -f migrations/schema/001_users.sql"
	@echo "Run: psql -d gitfed -f migrations/schema/002_organizations.sql"
	@echo "Run: psql -d gitfed -f migrations/schema/003_sessions.sql"
	@echo "Run: psql -d gitfed -f migrations/schema/004_sso.sql"

ci: lint test build-image

.PHONY: test build clean lint build-image compose-up compose-down sqlc migrate-up ci
