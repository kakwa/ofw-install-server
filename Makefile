APP_NAME := ofw-install-server

.PHONY: build run tidy fmt vet clean test

build:
	@go build -o $(APP_NAME) .

run:
	@go run .

tidy:
	@go mod tidy

fmt:
	@go fmt ./...

vet:
	@go vet ./...

clean:
	@rm -rf $(APP_NAME)
	@rm -f coverage.out
	@rm -f coverage.html

test:
	@go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
