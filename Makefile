.PHONY: test
test:
	go test ./... -coverprofile=coverage.out

.PHONY: lint
lint:
	golangci-lint run
