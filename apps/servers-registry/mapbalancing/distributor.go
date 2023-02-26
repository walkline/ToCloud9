package mapbalancing

import "github.com/walkline/ToCloud9/apps/servers-registry/repo"

type MapDistributor interface {
	Distribute(servers []repo.GameServer) []repo.GameServer
}
