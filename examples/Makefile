.PHONY: proto
proto:
	protoc --proto_path=. -I ./proto -I $$GOPATH/src --gogo_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. proto/greeter/*.proto

