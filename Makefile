.PHONY: test
test:
	go test -race -timeout 3m -coverprofile cp.out ./...

.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run

.PHONY: tools-install
tools-install:
	cat tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -tI % go install %
