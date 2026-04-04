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

ci: lint test build-image

.PHONY: test build clean lint build-image compose-up compose-down ci
