INSTALL_PATH?=bin

test:
	go test ./... -v

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
	go build -o $(INSTALL_PATH)/libsidecar.dylib -buildmode=c-shared game-server/libsidecar/lib.go

compose-rebuild-lb:
	docker-compose up -d --build --no-deps game-load-balancer

compose-rebuild-sr:
	docker-compose up -d --build --no-deps servers-registry

install: build-authserver build-charserver build-chatserver build-game-load-balancer build-servers-registry build-sidecar
