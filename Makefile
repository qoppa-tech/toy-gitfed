VERSION ?= $(shell git rev-parse --short HEAD)

test:
	@go test ./... -v -cover

build:
	@go build -o ./bin/http-api ./cmd/http-api

clean:
	@rm -f ./bin/http-api

lint:
	@go vet ./...

build-image:
	@docker build --build-arg APP_VERSION=$(VERSION) -t gitfed:$(VERSION) -t gitfed:latest .

compose-up:
	@docker compose up -d --build

compose-down:
	@docker compose down

sqlc:
	@sqlc generate

migrate-up:
	@docker compose run --rm migrate up

migrate-down:
	@docker compose run --rm migrate down 1

migrate-status:
	@docker compose run --rm migrate version

seed:
	@go run ./cmd/admin seed

test-integration:
	@docker compose -f docker-compose.test.yml up -d --wait
	@go test ./... -v -cover; ret=$$?; docker compose -f docker-compose.test.yml down; exit $$ret

ci: lint test-integration build-image

test-e2e:
	@go test ./e2e -v

test-e2e-auth:
	@go test ./e2e -v -run 'TestE2E/TestAuthSessionLifecycle'

test-e2e-git:
	@go test ./e2e -v -run 'TestE2E/TestGit'

.PHONY: test build clean lint build-image compose-up compose-down sqlctest-integration ci test-e2e test-e2e-auth test-e2e-git migrate-up migrate-down migrate-status seed
