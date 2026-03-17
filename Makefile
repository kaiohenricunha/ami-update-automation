.PHONY: build test test-integration test-e2e test-security test-cover lint clean zip

BINARY := bootstrap
LAMBDA_DIR := cmd/lambda
GOOS_LINUX := GOOS=linux
GOARCH_AMD64 := GOARCH=amd64
GOARCH_ARM64 := GOARCH=arm64
LDFLAGS := -ldflags="-s -w"
TAG ?= dev

build:
	$(GOOS_LINUX) $(GOARCH_AMD64) go build $(LDFLAGS) -o $(BINARY) ./$(LAMBDA_DIR)/
	@echo "Built $(BINARY) (linux/amd64)"

build-arm64:
	$(GOOS_LINUX) $(GOARCH_ARM64) go build $(LDFLAGS) -o $(BINARY)-arm64 ./$(LAMBDA_DIR)/
	@echo "Built $(BINARY)-arm64 (linux/arm64)"

test:
	go test -race ./internal/... ./pkg/... -count=1

test-integration:
	go test -race -tags=integration ./test/integration/... -count=1 -v

test-e2e:
	go test -race -tags=e2e ./test/e2e/... -count=1 -v

test-security:
	go test -race -tags=security ./test/security/... -count=1 -v

test-cover:
	go test -race -coverprofile=coverage.out ./internal/... ./pkg/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run

clean:
	rm -f $(BINARY) $(BINARY)-arm64 lambda.zip lambda-arm64.zip coverage.out coverage.html

zip: build
	zip lambda.zip $(BINARY)
	@echo "Created lambda.zip"

zip-arm64: build-arm64
	zip lambda-arm64.zip $(BINARY)-arm64
	@echo "Created lambda-arm64.zip"
