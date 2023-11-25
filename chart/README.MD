# ToCloud9 Helm chart

This chart will deploy a ToCloud9 cluster using **AzerothCore** as backend.

> **WARNING:** 
Pre-built images in this chart values won't be actively maintained, build your own ones if you want to be up to date with AzerothCore releases.

## Databases

By default this chart deploy a really simple mysql dependency chart and will create the required user and databases. However, it won't create the required tables. You need to create them with the [AzerothCore sql scripts](https://github.com/azerothcore/azerothcore-wotlk/tree/master/data/sql/base) and [ToCloud9 modifications](../doc/RunNonDockerWinWSLAzerothCore.md#setup--run).

### BYOD (Brin Your Own Database)

You can also use a preexisting database by setting the `databases.external_db_hostname` variable.

## How to use

1. Check the values to set a storage class for persistency and load-balancer domain.

2. Deploy the chart

3. Most services won't start as long as the databases tables aren't initialized. See Databases for more details.

4. Gameserver will require its `/data` volume to be provisionned with files. Do this, you can :
  - Use the gameserver init container to download the required file if your PV is empty. Check `gameserver_ac.initcontainer.download_url` for zip download origin.
  - Provision the files on your own once the PV is created, if `gameserver_ac.initcontainer.download_enabled` is disabled.

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