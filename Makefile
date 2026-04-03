test:
	@go test ./... -v -cover

build:
	@go build -o ./bin/http-api ./cmd/http-api

clean:
	@rm -f ./bin/http-api

sqlc:
	@sqlc generate

migrate-up:
	@echo "Run: psql -d gitfed -f migrations/schema/001_users.sql"
	@echo "Run: psql -d gitfed -f migrations/schema/002_organizations.sql"
	@echo "Run: psql -d gitfed -f migrations/schema/003_sessions.sql"
	@echo "Run: psql -d gitfed -f migrations/schema/004_sso.sql"

.PHONY: test build clean sqlc migrate-up
