BINARY_NAME := disco
BUILD_DIR := dist
GO := go

.PHONY: all build test build-all clean lint

all: build

build:
	mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/disco

test:
	$(GO) test ./... -v

lint:
	$(GO) vet ./...

build-all:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/disco
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/disco
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/disco
	GOOS=linux GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/disco
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/disco

clean:
	rm -rf $(BUILD_DIR)
