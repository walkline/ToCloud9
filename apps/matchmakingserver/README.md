# Matchmaking Service

The Matchmaking Service is responsible for handling player queues for battlegrounds (arenas and LFG in the future). It enables players to join or leave the queue, tracks queue status, and notifies relevant systems when battlegrounds start or end.

This service primarily interacts with the worldserver to create battleground instances and serves the game gateway as its main client.