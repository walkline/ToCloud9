go mod download
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.32
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3
go install golang.org/x/tools/cmd/stringer@latest
export PATH="$PATH:$(go env GOPATH)/bin"