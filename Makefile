test:
	@go test ./... -v -cover

build:
	@go build -o ./bin/http-api ./cmd/http-api

clean:
	@rm -f ./bin/http-api

sqlc:
	@sqlc generate

lint:
	@go vet ./...

build-image:
	@docker build -t gitfed:latest .

compose-up:
	@docker compose up -d --build

compose-down:
	@docker compose down

ci: lint test build-image

.PHONY: test build clean lint build-image compose-up compose-down ci migrate-up