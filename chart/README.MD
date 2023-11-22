# ToCloud9 Helm chart

This chart will deploy a ToCloud9 cluster using **AzerothCore** as backend.

## How to use

1. Check the values to set a storage class for persistency.
2. Deploy the chart
3. Most services won't start as long as the databases tables aren't initialized with the [AzerothCore sql scripts](https://github.com/azerothcore/azerothcore-wotlk/tree/master/data/sql/base) and [ToCloud9 modifications](../doc/RunNonDockerWinWSLAzerothCore.md#setup--run). Databases and user are already created during mysql deployment.
4. Gameserver will require its `/data` volume to be provisionned with files to be [downloaded](https://www.azerothcore.org/wiki/server-setup#download) or [extracted](https://www.azerothcore.org/wiki/server-setup#extractors).