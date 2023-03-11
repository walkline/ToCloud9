INSTALL_PATH?=bin

generate:
	go generate ./...
	mockery --dir=gen/servers-registry/pb --output=gen/servers-registry/pb/mocks --name=ServersRegistryServiceClient --structname=ServersRegistryServiceClient
	protoc --proto_path=api/proto/v1/characters --go_out=plugins=grpc:. characters.proto
	protoc --proto_path=api/proto/v1/servers-registry --go_out=plugins=grpc:. registry.proto
	protoc --proto_path=api/proto/v1/chat --go_out=plugins=grpc:. chat.proto
	protoc --proto_path=api/proto/v1/guilds --go_out=plugins=grpc:. guilds.proto
	protoc --proto_path=api/proto/v1/guid --go_out=plugins=grpc:. guid.proto
	protoc --proto_path=api/proto/v1/worldserver --go_out=plugins=grpc:. worldserver.proto

migrate-characters:
	migrate -database "mysql://trinity:trinity@tcp(localhost:3306)/characters" -path sql/characters/mysql up

test:
	go test ./... -race -coverprofile=coverage.out -covermode=atomic

build-authserver:
	go build -o $(INSTALL_PATH)/authserver apps/authserver/cmd/authserver/main.go

build-charserver:
	go build -o $(INSTALL_PATH)/charserver apps/charserver/cmd/charserver/main.go

build-chatserver:
	go build -o $(INSTALL_PATH)/chatserver apps/chatserver/cmd/chatserver/main.go

build-game-load-balancer:
	go build -o $(INSTALL_PATH)/game-load-balancer apps/game-load-balancer/cmd/game-load-balancer/main.go

build-servers-registry:
	go build -o $(INSTALL_PATH)/servers-registry apps/servers-registry/cmd/servers-registry/main.go

build-sidecar:
	go build -o $(INSTALL_PATH)/libsidecar.dylib -buildmode=c-shared ./game-server/libsidecar/

build-guidserver:
	go build -o $(INSTALL_PATH)/guidserver apps/guidserver/cmd/guidserver/main.go

build-guildserver:
	go build -o $(INSTALL_PATH)/guildserver apps/guildserver/cmd/guildserver/main.go

compose-rebuild-lb:
	docker-compose up -d --build --no-deps game-load-balancer

compose-rebuild-gs:
	docker-compose up -d --build --no-deps guildserver

compose-rebuild-chars:
	docker-compose up -d --build --no-deps charserver

compose-rebuild-sr:
	docker-compose up -d --build --no-deps servers-registry

compose-rebuild-authserver:
	docker-compose up -d --build --no-deps authserver

compose-rebuild-guidserver:
	docker-compose up -d --build --no-deps guidserver

install: build-authserver build-charserver build-chatserver build-game-load-balancer build-servers-registry build-sidecar build-guidserver build-guildserver
