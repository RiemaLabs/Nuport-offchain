proto-gen:
	@echo "Generating proto files..."
	@protoc --go_out=. --go_opt=paths=source_relative \
	--go-grpc_out=. --go-grpc_opt=paths=source_relative \
	proto/*.proto

#? fmt: Ensure consistent code formatting.
fmt:
	gofmt -s -w $(shell find . -name "*.go")

build:
	go build -o nubit-server cmd/*.go

run: 
	go run cmd/*.go
