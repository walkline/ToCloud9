INSTALL_PATH?=bin

generate:
	go generate ./...
	mockery --dir=gen/servers-registry/pb --output=gen/servers-registry/pb/mocks --name=ServersRegistryServiceClient --structname=ServersRegistryServiceClient
	mockery --dir=gen/characters/pb --output=gen/characters/pb/mocks --name=CharactersServiceClient --structname=CharactersServiceClient
	mockery --dir=gen/worldserver/pb --output=gen/worldserver/pb/mocks --name=WorldServerServiceClient --structname=WorldServerServiceClient
	mockery --dir=gen/mail/pb --output=gen/mail/pb/mocks --name=MailServiceClient --structname=MailServiceClient
	mockery --dir=gen/group/pb --output=gen/group/pb/mocks --name=GroupServiceClient --structname=GroupServiceClient
	# Preferred protobuf versions:
	# 	brew install protobuf@3
	# 	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.32
	# 	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.60.1
	protoc --proto_path=api/proto/v1/characters --go-grpc_out=. --go_out=. characters.proto
	protoc --proto_path=api/proto/v1/servers-registry --go-grpc_out=. --go_out=. registry.proto
	protoc --proto_path=api/proto/v1/chat --go-grpc_out=. --go_out=. chat.proto
	protoc --proto_path=api/proto/v1/guilds --go-grpc_out=. --go_out=. guilds.proto
	protoc --proto_path=api/proto/v1/guid --go-grpc_out=. --go_out=. guid.proto
	protoc --proto_path=api/proto/v1/mail --go-grpc_out=. --go_out=. mail.proto
	protoc --proto_path=api/proto/v1/worldserver --go-grpc_out=. --go_out=. worldserver.proto
	protoc --proto_path=api/proto/v1/group --go-grpc_out=. --go_out=. group.proto

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
	go build -ldflags=-w -o $(INSTALL_PATH)/libsidecar.so -buildmode=c-shared ./game-server/libsidecar/

build-guidserver:
	go build -o $(INSTALL_PATH)/guidserver apps/guidserver/cmd/guidserver/main.go

build-guildserver:
	go build -o $(INSTALL_PATH)/guildserver apps/guildserver/cmd/guildserver/main.go

build-mailserver:
	go build -o $(INSTALL_PATH)/mailserver apps/mailserver/cmd/mailserver/main.go

build-groupserver:
	go build -o $(INSTALL_PATH)/groupserver apps/groupserver/cmd/groupserver/main.go

compose-rebuild-lb:
	docker-compose up -d --build --no-deps game-load-balancer

compose-rebuild-lb2:
	docker-compose up -d --build --no-deps game-load-balancer-second

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

compose-rebuild-mailserver:
	docker-compose up -d --build --no-deps mailserver

compose-rebuild-groupserver:
	docker-compose up -d --build --no-deps groupserver

compose-rebuild-gameserver:
	docker-compose up -d --build --no-deps gameserver

install: build-authserver build-charserver build-chatserver build-game-load-balancer build-servers-registry build-sidecar build-guidserver build-guildserver build-mailserver build-groupserver
