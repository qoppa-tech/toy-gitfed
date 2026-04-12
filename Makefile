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

test-e2e:
	@go test ./e2e -v

test-e2e-auth:
	@go test ./e2e -v -run 'TestE2E/TestAuthSessionLifecycle'

test-e2e-git:
	@go test ./e2e -v -run 'TestE2E/TestGit'

.PHONY: test build clean lint build-image compose-up compose-down ci migrate-up test-e2e test-e2e-auth test-e2e-git
