package guids

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/gen/guid/pb"
)

type CrossrealmMgr struct {
	realmsContainer    map[uint32]GuidProvider
	pctToTriggerUpdate float32
	desiredPoolSize    uint64
	guidType           pb.GuidType
	client             pb.GuidServiceClient
}

func NewCrossRealmMgr(client pb.GuidServiceClient, guidType pb.GuidType, desiredPoolSize uint64, pctToTriggerUpdate float32, defaultRealmID uint32) *CrossrealmMgr {
	mgr := &CrossrealmMgr{
		realmsContainer:    map[uint32]GuidProvider{},
		pctToTriggerUpdate: pctToTriggerUpdate,
		desiredPoolSize:    desiredPoolSize,
		guidType:           guidType,
		client:             client,
	}

	mgr.InitForRealm(defaultRealmID)
	
	return mgr
}

func (m *CrossrealmMgr) Next(realmID uint32) uint64 {
	provider := m.realmsContainer[realmID]
	if provider == nil {
		provider = m.InitForRealm(realmID)
	}

	return provider.Next()
}

func (m *CrossrealmMgr) InitForRealm(realmID uint32) GuidProvider {
	var err error
	m.realmsContainer[realmID], err = NewThreadUnsafeGuidProvider(
		context.Background(),
		NewGRPCDiapasonsProvider(m.client, realmID, m.guidType, m.desiredPoolSize),
		m.pctToTriggerUpdate,
	)
	if err != nil {
		log.Fatal().Err(err).Uint32("realmID", realmID).Msg("failed to create guid provider")
	}
	return m.realmsContainer[realmID]
}
