# MySQL Cross Realm Reverse Proxy

A specialized cross-realm MySQL reverse proxy for routing prepared statement requests to the appropriate realm characters database.

## Overview

In the current architecture of AzerothCore/TrinityCore, each realm has its own characters database. When implementing cross-realm functionality, it's necessary to connect to all character databases and determine the correct connection based on the character's realm. This proxy was created to offload this logic from the worldserver (gameserver).

## How It Works

1. **Routing Logic**: The reverse proxy intercepts SQL requests, primarily prepared statements, from the worldserver.
2. **Character GUID Analysis**: It analyzes the request to locate the character GUID.
3. **Realm ID Extraction**: The proxy extracts the realm ID from the character GUID.
4. **Request Routing**: Using the realm ID, the proxy routes the request to the correct characters database.
5. **GUID Manipulation**: The proxy also handles any necessary GUID transformations for the request.

By directing the `CharacterDatabaseInfo` parameter in the worldserver to point to this proxy, the proxy handles routing and processing, simplifying the cross-realm logic within AzerothCore/TrinityCore.

![](https://raw.githubusercontent.com/walkline/ToCloud9/master/.github/images/cross-realm-mysql.svg "mysql crossrealm reverse proxy")
