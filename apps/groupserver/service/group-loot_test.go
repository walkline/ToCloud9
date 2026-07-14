package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/walkline/ToCloud9/apps/groupserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

type noopGroupProducer struct{}

func (noopGroupProducer) InviteCreated(*events.GroupEventInviteCreatedPayload) error { return nil }
func (noopGroupProducer) GroupCreated(*events.GroupEventGroupCreatedPayload) error   { return nil }
func (noopGroupProducer) GroupMemberOnlineStatusChanged(*events.GroupEventGroupMemberOnlineStatusChangedPayload) error {
	return nil
}
func (noopGroupProducer) GroupMemberLeft(*events.GroupEventGroupMemberLeftPayload) error { return nil }
func (noopGroupProducer) GroupDisband(*events.GroupEventGroupDisbandPayload) error       { return nil }
func (noopGroupProducer) MemberAdded(*events.GroupEventGroupMemberAddedPayload) error    { return nil }
func (noopGroupProducer) LeaderChanged(*events.GroupEventGroupLeaderChangedPayload) error {
	return nil
}
func (noopGroupProducer) LootTypeChanged(*events.GroupEventGroupLootTypeChangedPayload) error {
	return nil
}
func (noopGroupProducer) ConvertedToRaid(*events.GroupEventGroupConvertedToRaidPayload) error {
	return nil
}
func (noopGroupProducer) TargetIconUpdated(*events.GroupEventNewTargetIconPayload) error { return nil }
func (noopGroupProducer) GroupDifficultyChanged(*events.GroupEventGroupDifficultyChangedPayload) error {
	return nil
}
func (noopGroupProducer) SendChatMessage(*events.GroupEventNewMessagePayload) error { return nil }
func (noopGroupProducer) MembersUpdated(*events.GroupEventGroupMembersUpdatedPayload) error {
	return nil
}

// The stock 3.3.5 client sends CMSG_LOOT_METHOD with threshold 0 when only the
// method changes; unvalidated it turns every grey/white item into a roll on
// each world server. The method change must apply with the threshold kept.
func TestGroupsServiceSetLootMethodKeepsThresholdOnStockPacket(t *testing.T) {
	cache := newWarmedUpCache(t)
	ctx := context.Background()

	group := newTwoMembersGroup()
	group.LootThreshold = uint8(repo.ItemQualityRare)
	assert.NoError(t, cache.Create(ctx, 1, group))

	s := NewGroupsService(cache, nil, noopGroupProducer{})

	assert.NoError(t, s.SetLootMethod(ctx, 1, 1, uint8(repo.LootTypeGroupLoot), 0, 0))

	updated, err := s.GroupByID(ctx, 1, 1)
	assert.NoError(t, err)
	assert.Equal(t, uint8(repo.LootTypeGroupLoot), updated.LootMethod)
	assert.Equal(t, uint8(repo.ItemQualityRare), updated.LootThreshold)
}

func TestGroupsServiceSetLootMethodValidation(t *testing.T) {
	cache := newWarmedUpCache(t)
	ctx := context.Background()

	assert.NoError(t, cache.Create(ctx, 1, newTwoMembersGroup()))

	s := NewGroupsService(cache, nil, noopGroupProducer{})

	// Unknown loot method.
	assert.Error(t, s.SetLootMethod(ctx, 1, 1, 42, 0, uint8(repo.ItemQualityUncommon)))

	// Master looter must be a group member.
	assert.Error(t, s.SetLootMethod(ctx, 1, 1, uint8(repo.LootTypeMasterLoot), 777, uint8(repo.ItemQualityUncommon)))

	// Valid change applies both fields.
	assert.NoError(t, s.SetLootMethod(ctx, 1, 1, uint8(repo.LootTypeMasterLoot), 2, uint8(repo.ItemQualityEpic)))
	updated, err := s.GroupByID(ctx, 1, 1)
	assert.NoError(t, err)
	assert.Equal(t, uint8(repo.LootTypeMasterLoot), updated.LootMethod)
	assert.Equal(t, uint8(repo.ItemQualityEpic), updated.LootThreshold)
	assert.Equal(t, uint64(2), updated.LooterGUID)
}
