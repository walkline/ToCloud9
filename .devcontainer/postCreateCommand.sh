go mod download
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.27.1
go install golang.org/x/tools/cmd/stringer@latest
export PATH="$PATH:$(go env GOPATH)/bin"