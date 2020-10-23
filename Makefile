CC=go

.PHONY: test
test:
	$(CC) test -cover ./...

stats:
	scc --exclude-dir 'vendor,node_modules,data,.git,docker/etcdkeeper,utils' --wide
