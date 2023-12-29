# ToCloud9 Helm chart

This chart will deploy a ToCloud9 cluster using **AzerothCore** as backend.

## Databases

By default this chart deploy a really simple mysql dependency chart and will create the required user and databases. However, it won't create the required tables. You need to create them with the [AzerothCore sql scripts](https://github.com/azerothcore/azerothcore-wotlk/tree/master/data/sql/base) and [ToCloud9 modifications](../doc/RunNonDockerWinWSLAzerothCore.md#setup--run).

### BYOD (Brin Your Own Database)

You can also use a preexisting database by setting the `databases.external_db_hostname` variable.

## How to use

1. Check the [values](values.yaml) to set a storage class for persistency and game-load-balancer domain.

2. Deploy the chart

3. Most services won't start as long as the databases tables aren't initialized. See Databases for more details.

4. Gameserver will require its `/data` volume to be provisionned with files. Do this, you can :
  - Use the gameserver init container to download the required file if your PV is empty. Check `gameserver_ac.initcontainer.download_url` for zip download origin.
  - Provision the files on your own once the PV is created, if `gameserver_ac.initcontainer.download_enabled` is disabled.

### Example

Here is an example of how to install the chart with an external database and the `my_domain_or_ip.com` domain.
Make sure that the `game_lb` service is reachable at the specified address.

```
cd $TO_CLOUD9_PROJECT_ROOT_DIR
kubectl create ns tocloud9
helm install cloud9 ./chart -n tocloud9 \
   --set gameserver_ac.storageClass=standard \
   --set mysql.enabled=false \
   --set databases.db_user=db_user \
   --set databases.db_password=db_pass \
   --set databases.external_db_hostname=192.168.65.2 \
   --set game_lb.hostname=my_domain_or_ip.com \
   --set game_lb.service.type=ClusterIP
```

More parameters specified in the [values](values.yaml) file.