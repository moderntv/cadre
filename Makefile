CC=go

.PHONY: test
test:
	$(CC) test -cover ./...

.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run

.PHONY: tools-install
tools-install:
	cat tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -tI % go install %

stats:
	scc --exclude-dir 'vendor,node_modules,data,.git,docker/etcdkeeper,utils' --wide
