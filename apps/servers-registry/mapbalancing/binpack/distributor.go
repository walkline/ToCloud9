package binpack

import (
	"math"
	"sort"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/servers-registry/mapbalancing"
	"github.com/walkline/ToCloud9/apps/servers-registry/repo"
)

type binpackBalancer struct {
	weights MapsWeight
}

func NewBinPackBalancer(weights MapsWeight) mapbalancing.MapDistributor {
	return &binpackBalancer{
		weights: weights,
	}
}

func (k *binpackBalancer) Distribute(servers []repo.GameServer) []repo.GameServer {
	weightsCopy := make(MapsWeight, len(k.weights))
	for key, v := range k.weights {
		weightsCopy[key] = v
	}

	// cleanup prev maps distribution
	for i := range servers {
		servers[i].AssignedMapsToHandle = []uint32{}
	}

	serversToBalance := []repo.GameServer{}
	readyServers := []repo.GameServer{}
	for _, server := range servers {
		if server.IsAllMapsAvailable() {
			serversToBalance = append(serversToBalance, server)
			continue
		}

		server.AssignedMapsToHandle = server.AvailableMaps
		readyServers = append(readyServers, server)

		for _, availableMap := range server.AvailableMaps {
			delete(weightsCopy, availableMap)
		}
	}

	if len(serversToBalance) == 0 {
		return readyServers
	}

	if len(serversToBalance) == 1 {
		for key := range weightsCopy {
			serversToBalance[0].AssignedMapsToHandle = append(serversToBalance[0].AssignedMapsToHandle, key)
		}

		sort.Slice(serversToBalance[0].AssignedMapsToHandle, func(ii, jj int) bool {
			return serversToBalance[0].AssignedMapsToHandle[ii] < serversToBalance[0].AssignedMapsToHandle[jj]
		})

		return append(readyServers, serversToBalance...)
	}

	return append(readyServers, k.greedyBinPackBalancer(weightsCopy, serversToBalance)...)
}

func (k *binpackBalancer) greedyBinPackBalancer(weights MapsWeight, servers []repo.GameServer) []repo.GameServer {
	totalWeight := uint32(0)
	for _, v := range weights {
		totalWeight += v
	}

	weightPerServer := uint32(math.Ceil(float64(totalWeight) / float64(len(servers))))

	// Check if there is map with higher weight than weightPerServer.
	for mapID, weight := range weights {
		if weight > weightPerServer {
			log.Warn().
				Uint32("mapID", mapID).
				Uint32("maxPossibleWeight", weightPerServer).
				Msg("Map has bigger height than one node can handle, setting maxPossibleWeight for this map. Maps distribution can be not accurate.")
			weights[mapID] = weightPerServer
		}
	}

	mw := make([]mapIDWeight, 0, len(weights))
	for mapID, weight := range weights {
		mw = append(mw, mapIDWeight{
			mapID:  mapID,
			weight: weight,
		})
	}

	sort.Slice(mw, func(i, j int) bool {
		if mw[i].weight == mw[j].weight {
			return mw[i].mapID < mw[j].mapID
		}
		return mw[i].weight > mw[j].weight
	})

	binStates := []uint32{}
	packing := [][]mapIDWeight{}

	for _, mapWeight := range mw {
		if i := k.firstAvailableBin(binStates, mapWeight.weight); i >= 0 {
			// The i-th bin has enough space for this item.
			binStates[i] -= mapWeight.weight
			packing[i] = append(packing[i], mapWeight)
		} else {
			// A new bin is required.
			binStates = append(binStates, weightPerServer-mapWeight.weight)
			packing = append(packing, []mapIDWeight{mapWeight})
		}
	}

	for i := range packing {
		serverIndex := i
		if serverIndex >= len(servers) {
			serverIndex = i % len(servers)
		}

		for j := range packing[i] {
			servers[serverIndex].AssignedMapsToHandle = append(servers[serverIndex].AssignedMapsToHandle, packing[i][j].mapID)
		}
	}

	for i := range servers {
		sort.Slice(servers[i].AssignedMapsToHandle, func(ii, jj int) bool {
			return servers[i].AssignedMapsToHandle[ii] < servers[i].AssignedMapsToHandle[jj]
		})
	}

	return servers
}

func (k *binpackBalancer) firstAvailableBin(binStates []uint32, mapWeight uint32) int {
	for i, availableBinSize := range binStates {
		if availableBinSize >= mapWeight {
			return i
		}
	}
	return -1
}

type mapIDWeight struct {
	mapID  uint32
	weight uint32
}
