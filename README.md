# ToCloud9

**ToCloud9** is an attempt to make [TrinityCore](https://github.com/TrinityCore/TrinityCore) and its forks scalable and cloud-native. 
The project is at the beginning of development and has limited functionality. 

The current architecture described (in simplified form) in the image bellow:
![](.github/images/tc9.svg "architecture")
Since project is at beginning of development more components would be added and modified.

At the moment, it supports 3.3.5 client and has the next applications:
* __authserver__ authorizes players, provides realmlist and connects a game client to the "smart" game load balancer with the least active connections;
* __game-load-balancer__ holds game client TCP connection, offloads encryption, reads packets and routes requests to other services.
For every character creates connection to the game server (TrinityCore) that Servers Registry provides. 
Also intercepts some packets and uses information from them to sync some states between services. 
* __servers-registry__ holds information about every running instance of Game Load Balancer and Game Server (TrinityCore world server).
  Makes health checks and collects necessary metrics (active connections at the moment). Assigns maps to Game Server instances. 
* __chatserver__ at the moments holds characters online and handles "whisper" messages;
* __charserver__ provides information to handle SMsgCharEnum opcode. Holds information about connected players. Handles Who opcode.
* __gameserver__ is modified TrinityCore world server with `sidecar` library, that registers GameServer in Servers Registry and handles health checks.
* __guildserver__ handles some guild opcodes. Still misses guild creation and guildbank functionality.
* __guidserver__ provides pool of guids of items and characters to the gameservers.
* __mailserver__ handles mail opcodes.

## Deployment

### Kubernetes Cluster

Utilize the [helm chart](chart/) to seamlessly deploy the solution in your Kubernetes cluster. Kudos to [@2o1o0](https://github.com/2o1o0) for the solution.

### Docker-Compose

__Prerequisites:__
* Database for TrinityCore or AzerothCore;
* TrinityCore or AzerothCore data folder (dbc, vmaps, mmaps) and config (ect folder).
* [Docker & docker-compose](https://www.docker.com/products/docker-desktop) (for 'Docker-compose' approach);

__Steps:__
1. Fill in `.env` file with relevant data.
2. Apply migrations to the characters DB from this folder - sql/characters/mysql/*
3. 
```
# For TrinityCore:
$ docker-compose --profile tc up -d

# For AzerothCore:
$ docker-compose --profile ac up -d
```
### Without Docker/Orchestration

For Windows & AzerothCore [use this guide](doc/RunNonDockerWinWSLAzerothCore.md).

For Linux and Mac - TBD.

## Community

We have the next [Discord channel](https://discord.gg/QxfBD9uGbN) where you can ask any questions and share your feedback.

## License

See [LICENSE](LICENSE).