.PHONY: test
test:
	go test -race -timeout 3m -coverprofile cp.out ./...

.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run

.PHONY: clean
clean:
	rm cp.out golangci-lint.out golangci-lint.out.html
