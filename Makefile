APP_NAME := sun-netinstall-server

.PHONY: build run tidy fmt vet clean

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

