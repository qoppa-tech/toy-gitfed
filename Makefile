test:
	@go test ./... -v -cover

build:
	@go build -o ./bin/main ./cmd/main.go

clean:
	@rm -f ./bin/main

.PHONY: test build clean
