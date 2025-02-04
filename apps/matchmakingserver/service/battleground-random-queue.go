package service

import (
	"context"

	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/repo"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type BattlegroundRandomQueue struct {
	realQ *GenericBattlegroundQueue

	service BattleGroundService
}

func NewBattlegroundRandomQueue(service BattleGroundService, battleGroundCreator BattlegroundCreator, template repo.BattlegroundTemplate, realmID, battlegroupID uint32, bracketID uint8) *BattlegroundRandomQueue {
	randomQ := &BattlegroundRandomQueue{
		service: service,
	}
	realQ := NewGenericBattlegroundQueue(service, randomQ, template, realmID, battlegroupID, bracketID)
	randomQ.realQ = realQ
	realQ.QueueTypeID = battleground.QueueTypeIDRandomBattleground
	return randomQ
}

func (b BattlegroundRandomQueue) AddQueuedGroup(g *QueuedGroup) error {
	g.IsRandomQueue = true
	return b.realQ.AddQueuedGroup(g)
}

func (b BattlegroundRandomQueue) RemoveQueuedGroup(player guid.PlayerUnwrapped) error {
	return b.realQ.RemoveQueuedGroup(player)
}

func (b BattlegroundRandomQueue) RemoveAllQueuedGroups() {
	b.realQ.RemoveAllQueuedGroups()
}

func (b BattlegroundRandomQueue) GetAllQueuedGroups() []QueuedGroup {
	return b.realQ.GetAllQueuedGroups()
}

func (b BattlegroundRandomQueue) QueuedGroupByPlayer(player guid.PlayerUnwrapped) *QueuedGroup {
	return b.realQ.QueuedGroupByPlayer(player)
}

func (b BattlegroundRandomQueue) GetQueueTypeID() battleground.QueueTypeID {
	return battleground.QueueTypeIDRandomBattleground
}

func (b BattlegroundRandomQueue) CreateBattleground(
	ctx context.Context,
	template repo.BattlegroundTemplate,
	queueType battleground.QueueTypeID,
	bracketID BracketID,
	realmID, battlegroupID uint32,
	allianceGroups, hordeGroups []QueuedGroup,
) error {
	// Generate a new bg
	b.realQ.BattlegroundTypeID = battleground.TypeID(b.service.TemplateForQueueTypeID(ctx, battleground.QueueTypeIDRandomBattleground).TypeID)
	return b.service.CreateBattleground(ctx, template, queueType, bracketID, realmID, battlegroupID, allianceGroups, hordeGroups)
}
