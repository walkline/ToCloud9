# Build and Run with No Docker + Windows + WSL + AzerothCore

To run ToCloud9 on Windows without docker you would need to use WSL for Redis and for AzerothCore.

### Prerequisites

* Configured [WSL](https://learn.microsoft.com/en-us/windows/wsl/install) (preferably with Ubuntu).
* Installed [Go](https://go.dev/dl/).
* Installed [Redis](https://redis.io/docs/getting-started/installation/install-redis-on-windows/) in WSL.
* Installed [NATS](https://docs.nats.io/running-a-nats-service/introduction/installation#installing-via-a-package-manager).
* Installed [MySQL Server](https://dev.mysql.com/downloads/mysql/8.0.html) preferably with already configured AzerothCore databases.

### Build

1. Download ToCloud9 sourcecode preferably with git (`git clone https://github.com/walkline/ToCloud9.git`).
2. Go to the directory with downloaded sources and open Terminal there.
3. Build apps with given commands in the Terminal:
```
go build -o bin/authserver.exe apps/authserver/cmd/authserver/main.go
go build -o bin/charserver.exe apps/charserver/cmd/charserver/main.go
go build -o bin/chatserver.exe apps/chatserver/cmd/chatserver/main.go
go build -o bin/game-load-balancer.exe apps/game-load-balancer/cmd/game-load-balancer/main.go
go build -o bin/servers-registry.exe apps/servers-registry/cmd/servers-registry/main.go
go build -o bin/guidserver.exe apps/guidserver/cmd/guidserver/main.go
go build -o bin/guildserver.exe apps/guildserver/cmd/guildserver/main.go
go build -o bin/groupserver.exe apps/groupserver/cmd/groupserver/main.go
go build -o bin/mailserver.exe apps/mailserver/cmd/mailserver/main.go
```
4. Now in `bin` directory you should see 9 .exe files. We will get back to them on Setup & Run steps.

### Build AzerothCore

You need to build & run AzerothCore inside of WSL, the reason for that is libsidecar (more details [here](https://github.com/walkline/azerothcore-wotlk/blob/af06d3c5e24f1f3f0a820eea18aba8c6e5633dd6/deps/libsidecar/CMakeLists.txt#L13)).

Let's start with building libsidecar.

1. To build libsidecar you need to have Go installed in WSL as well. So please follow *Linux* instructions described here: https://go.dev/doc/install.
2. Open ToCloud9 folder downloaded from previous steps in WSL terminal and run the next command:
```
go build -o bin/libsidecar.so -buildmode=c-shared ./game-server/libsidecar/
```
3. Download AzerothCore sources with cluster mode support using git in some folder, example:
```
cd ~/dev/
git clone --branch cluster-mode https://github.com/walkline/azerothcore-wotlk.git
```
4. Copy `bin/libsidecar.so` file from ToCloud9 directory to the AzerothCore `deps/libsidecar/` and to `/usr/lib` folder, example:
```
cd ~/dev/
cp $PATH_TOCLOUD9_DIR/bin/libsidecar.so azerothcore-wotlk/deps/libsidecar/libsidecar.so
sudo cp $PATH_TOCLOUD9_DIR/bin/libsidecar.so /usr/lib/libsidecar.so
```
5. Follow regular steps to [build and setup AzerothCore in Linux](https://www.azerothcore.org/wiki/linux-requirements) but skip cloning/downloading sources step and __ADD cmake option `-DUSE_REAL_LIBSIDECAR=ON`__.
6. On this step, you should have already configured AzerothCore in the same way as for non-cluster mode, so your worldserver can startup.

### Setup & Run

1. Apply database migrations (2 migrations to the acore_characters db at the moment of writing) from `sql/characters/mysql` folder. 

    *a*. Since there are a few of them, you can apply them manually just by executing the content of `*.up.sql` files.
    
    *b*. Or you can use tool like [this](https://github.com/golang-migrate/migrate) and execute similar command:
```
migrate -database "mysql://acore:acore@tcp(localhost:3306)/acore_characters" -path sql/characters/mysql up
```
2. On Windows go the ToCloud9 folder and copy `config.yml.example` file into `bin` subdirectory and rename it to `config.yml`.
3. Edit content of this new file by updating at least `db:` section. DB section should look like this:
```
db:
  auth: &defaultAuthDB "dbUser:dbPassword@tcp(127.0.0.1:3306)/acore_auth"
  characters: &defaultCharactersDB "dbUser:dbPassword@tcp(127.0.0.1:3306)/acore_characters"
  world: &defaultWorldDB "dbUser:dbPassword@tcp(127.0.0.1:3306)/acore_world"
  # Supported 2 schema types:
  #   tc - TrinityCore
  #   ac - AzerothCore
  schemaType: &defaultSchemaType "ac"
```
4. Make sure that Nats is running. If not, run this command in Windows terminal: `nats-server`.
5. Make sure that Redis is running. If not, run this command in *WSL* terminal: `sudo service redis-server start`.
6. Run all of 8 exe files from `bin` subdirectory of ToCloud9 folder in the next order:

At first run:
   * servers-registry.exe
   * guidserver.exe

Then run the rest of exe files:
   * authserver.exe
   * charserver.exe
   * chatserver.exe
   * game-load-balancer.exe
   * guildserver.exe
   * groupserver.exe
   * mailserver.exe

At this point, you should be able to log in to your account (if you have any in the database) and see the list of your characters. 
However, when you try to log in to the game with one of your characters, you will get the "World server is down" error in your game client.

To resolve this issue, let's proceed with the AzerothCore running instructions. 

_Note:_ at the moment ToCloud9 supports only 1 realm with ID - 1. 

To run AzerothCore worldserver follow the next steps:
1. Don't use `autherserver` from AzerothCore. ToCloud9 uses its own implementation of authserver that you already started. 
2. In WSL terminal go to the folder with `worldserver` and run it using the following command:
```
AC_CLUSTER_ENABLED=1 AC_WORLD_SERVER_PORT=9601 ./worldserver
```
3. At this point you should have working cluster with 1 worldserver, but the point of this clustering feature to have several.
Let's run one more worldserver. You need to use the same command as previous but you need to override ports with some available, example:
```
AC_CLUSTER_ENABLED=1 AC_WORLD_SERVER_PORT=9602 GRPC_PORT=9603 HEALTH_CHECK_PORT=9604 ./worldserver
```
4. Now you should have 2 worldservers in your cluster and map IDs distributed equally between them.
But let's run worldserver that will handle only Icecrown Citadel map:
```
AC_CLUSTER_AVAILABLE_MAPS=631 AC_CLUSTER_ENABLED=1 AC_WORLD_SERVER_PORT=9605 GRPC_PORT=9606 HEALTH_CHECK_PORT=9607 ./worldserver
```

Congratulations if you reached to this place without issues! :) 

Feel free to share your feedback in the [Discord channel](https://discord.gg/QxfBD9uGbN).
