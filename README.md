# ToCloud9

**ToCloud9** provides a variety of microservices that operate alongside AzerothCore/TrinityCore and enable clustering support, making the system scalable and highly available.

## Architecture
The primary concept underlying the current architecture is to enhance the scalability of TrinityCore/AzerothCore with minimal modifications on their end.

To fulfill these objectives, a game-load-balancer microservice has been developed.
Functioning akin to an API Gateway, the game-load-balancer analyzes packets and strategically routes them to the Gameserver or generates requests to other services for handling.

The simplified architecture described below.

![](.github/images/tc9.svg "architecture")

If you'd like to read more, you can take a look at the pillars that form the foundation of ToCloud9 **[here](https://github.com/azerothcore/azerothcore-wotlk/discussions/16748)**.

## Current state
Currently, it is possible to play the game, but some functionalities still do not support a distributed architecture (clustering). Here is a list of features/tasks that, once completed, will enable it to replace the widely used unscalable monolith (vanilla TrinityCore/AzerothCore). The status is relevant for integration with AzerothCore.

| Feature/Task 	                                                | Status	 |                                                                                                            Comment 	                                                                                                             |
|---------------------------------------------------------------|---------|:--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------:|
| Gameservers and other services discovery                      | ✅       |                                                                                                                	                                                                                                                 |
| Services communication with NATS and gRPC                     | ✅	      |                                                                                                                	                                                                                                                 |
| Redirect players from one gameserver to another on map change | ✅	      |                                                                                                                	                                                                                                                 |
| Gameservers crash recovery                                    | ✅	      |                                                                              Players would be redirected <br/>to the another available gameserver	                                                                               |
| Automatic load balancing maps between gameservers             | ✅	      |                                                                                                                	                                                                                                                 |
| Shared pool of GUIDs                                          | ✅	      |                                                                                             Sharing Players, Items, Instance GUIDs	                                                                                              |
| "Who" opcode handling                                         | ✅	      |                                                                                                                	                                                                                                                 |
| Whispering in cluster support                                 | ✅	      |                                                                                                                	                                                                                                                 |
| Guilds in cluster support                                     | 90%	    |                                                                              Guild creation functionality is missing                              	                                                                              |
| Guild bank in cluster support                                 | 0%	     |                                                                                                                	                                                                                                                 |
| Mail in cluster support                                       | ✅	      |                                                                                                                	                                                                                                                 |
| Auction house in cluster support                              | 0%	     |                                                                                                                	                                                                                                                 |
| Friends list in cluster support                               | 0%	     |                                                                                                                	                                                                                                                 |
| Global channels in cluster support                            | 0%	     |                                                                                                                	                                                                                                                 |
| Parties and raids in cluster support                          | 80%	    | **Not implemented:** <br/>ready checks, instances reset on player request, <br/>prolonging instance bind, <br/>moving raid members between groups, <br/>and updating group members state <br/>like health when on different maps |
| Battlegrounds in cluster support                              | ✅	      |                                                                                                                	                                                                                                                 |
| Battlegrounds cross-realm support                             | ✅	      |                                                                                                                	                                                                                                                 |
| Arenas in cluster support                                     | 0%	     |                                                                                                                	                                                                                                                 |
| LFG in cluster support                                        | 0%	     |                                                                                                                	                                                                                                                 |
| Sync transports between gameservers                           | ✅	      |                                                                                                                	                                                                                                                 |
| Helm chart support                                            | ✅	      |                                                                                                                	                                                                                                                 |

## Deployment

### Kubernetes Cluster

Utilize the [helm chart](chart/) to seamlessly deploy the solution in your Kubernetes cluster. Kudos to [@2o1o0](https://github.com/2o1o0) for the solution.

### Docker Compose

__Prerequisites:__
* [Docker & Docker Compose](https://www.docker.com/products/docker-desktop) (for 'Docker Compose' approach);

__Steps:__
1. Fill in `.env` file with relevant data.
2. Start the setup containers with:

```bash
$ docker compose --profile setup-ac up -d
```
3. Wait until all setup containers (except the database) have stopped, then bring everything down safely:

```bash
$ docker compose down
```
4. Start the server normally:

```bash
$ docker compose --profile ac up -d
```

[!NOTE]
There's an default admin account included (admin:admin), be sure to change the password

### Without Docker/Orchestration

For Windows & AzerothCore [use this guide](doc/RunNonDockerWinWSLAzerothCore.md).

For Linux and Mac - TBD.

You can utilise [Perun tool](https://github.com/walkline/ToCloud9/tree/master/apps/perun) to simplify managing of all apps/microservices.  

## Community

We have the next [Discord channel](https://discord.gg/QxfBD9uGbN) where you can ask any questions and share your feedback.

## License

See [LICENSE](LICENSE).