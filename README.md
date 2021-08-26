# ToCloud9
___

**ToCloud9** is an attempt to make [TrinityCore](https://github.com/TrinityCore/TrinityCore) and its forks scalable and cloud-native. 
The project is at the beginning of development and has limited functionality. 
Right now, it can be used for deploying without downtime and for recovery from crashes on the TrinityCore side. 
Small demo video available [here](https://www.youtube.com/watch?v=jt1I0JmarUw).

The original idea to make it scalable described (in simplified form) in the image bellow:
![](.github/images/tc9.svg "architecture")

At the moment, it supports 3.3.5 client and has the next applications:
* __authserver__ authorizes players, provides realmlist and connects a game client to the "smart" game load balancer with the least active connections;
* __game-load-balancer__ holds game client TCP connection, offloads encryption, reads packets and routes requests to other services.
For every character creates connection to the game server (TrinityCore) that Servers Registry provides.
* __servers-registry__ holds information about every running instance of Game Load Balancer and Game Server (TrinityCore world server).
  Makes health checks and collects necessary metrics (active connections at the moment). 
* __chatserver__ at the moments holds characters online and handles "whisper" messages;
* __charserver__ at the moments only provides information to handle SMsgCharEnum opcode.
* __gameserver__ is modified TrinityCore world server with `sidecar` library, that registers GameServer in Servers Registry and handles health checks.

## Run

__Prerequisites:__
* Database for TrinityCore;
* TrinityCore data folder (dbc, vmaps, mmaps).
* [Docker & docker-compose](https://www.docker.com/products/docker-desktop) (for 'Docker-compose' approach);
* [Golang](https://golang.org/dl/) (for 'Without Docker' approach).

#### Docker-compose

1. Fill in `.env` file with relevant data.
2. `$ docker-compose up -d`

#### Without Docker

1. Run `$ make install`.
2. Apply `game-server/trinitycore/31ea74b96e.diff` patch on TrinityCore (should be compatible with [this rev](https://github.com/TrinityCore/TrinityCore/commit/31ea74b96e)).
3. Place `bin/libsidecar.dylib` & `bin/libsidecar.h` files in `$TRINITY_CORE_SRC_PATH/dep/libsidecar` folder and in folder with `worldserver` executable.
4. Build patched TrinityCore.
5. Install and run [NATS](https://docs.nats.io/nats-server/installation).
6. Run everything.
```bash
export AUTH_DB_CONNECTION=trinity:trinity@tcp(127.0.0.1:3306)/auth
export CHAR_DB_CONNECTION=trinity:trinity@tcp(127.0.0.1:3306)/characters
export WORLD_DB_CONNECTION=trinity:trinity@tcp(127.0.0.1:3306)/world
export NATS_URL=nats://nats:4222

./bin/servers-registry
./bin/charserver
./bin/chatserver

# this port will be used in realmlist.wtf
PORT=3724 ./bin/authserver

# PREFERRED_HOSTNAME & PORT address that game client will connect to after auth
REALM_ID=1 \
PREFERRED_HOSTNAME=domain-or-ip.com \
PORT=9876 ./bin/game-load-balancer

# run TrinityCore worldserver as always but add HEALTH_CHECK_PORT env variable
HEALTH_CHECK_PORT=9898 ./worldserver

```

## License

See [LICENSE](LICENSE).