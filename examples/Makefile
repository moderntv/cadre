.PHONY: proto
proto:
	protoc --proto_path=. \
		--go_out=paths=source_relative:. \
		--go-grpc_out=paths=source_relative:. \
		proto/greeter/*.proto

.PHONY: build
build:
	go build ./...

.PHONY: vet
vet:
	go vet ./...
