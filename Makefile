CC=go

.PHONY: test
test:
	$(CC) test -cover ./...

.PHONY: lint
lint:
	@golangci-lint run --timeout 5m -D structcheck,unused -E bodyclose,exhaustive,exportloopref,gosec,misspell,rowserrcheck,unconvert,unparam --out-format tab --sort-results --tests=false

stats:
	scc --exclude-dir 'vendor,node_modules,data,.git,docker/etcdkeeper,utils' --wide
