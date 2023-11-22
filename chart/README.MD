# ToCloud9 Helm chart

This chart will deploy a ToCloud9 cluster using **AzerothCore** as backend.

> **WARNING:_** 
Pre-built images in this chart values won't be actively maintained, build your own ones if you want to be up to date with AzerothCore releases.

## How to use

1. Check the values to set a storage class for persistency and load-balancer domain.
2. Deploy the chart
3. Most services won't start as long as the databases tables aren't initialized with the [AzerothCore sql scripts](https://github.com/azerothcore/azerothcore-wotlk/tree/master/data/sql/base) and [ToCloud9 modifications](../doc/RunNonDockerWinWSLAzerothCore.md#setup--run). Databases and user are already created during mysql deployment.
4. Gameserver will require its `/data` volume to be provisionned with files to be [downloaded](https://www.azerothcore.org/wiki/server-setup#download) or [extracted](https://www.azerothcore.org/wiki/server-setup#extractors).
5. If you don't localhost as "domain" and port forwarding to both auth server and lb, you need to update the auth database realmlist table with propre informations, ie :
```sql
UPDATE realmlist
SET
  name = 'myawesomerealmname',
  address = 'wow.myawesome.domain',
  localAddress = '192.168.1.X',
  localSubnetMask = '255.255.255.0',
  port = 8085
WHERE id=1;
```