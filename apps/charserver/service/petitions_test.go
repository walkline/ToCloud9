package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type petitionsRepoMock struct {
	petition   *repo.Petition
	signatures []repo.PetitionSignature
	chars      map[uint64]*repo.CharacterBrief
	invites    map[uint64]bool

	addCalls int
	lastAdd  struct {
		ownerLow, itemLow, playerLow, accountID uint32
	}
	deletedID uint32
	renamed   struct {
		id   uint32
		name string
	}
}

func (m *petitionsRepoMock) GetPetitionByItemLow(_ context.Context, _, itemLow uint32) (*repo.Petition, error) {
	if m.petition == nil || m.petition.ItemLow != itemLow {
		return nil, nil
	}
	return m.petition, nil
}

func (m *petitionsRepoMock) GetSignaturesByItemLow(_ context.Context, _, itemLow uint32) ([]repo.PetitionSignature, error) {
	if m.petition == nil || m.petition.ItemLow != itemLow {
		return nil, nil
	}
	return append([]repo.PetitionSignature(nil), m.signatures...), nil
}

func (m *petitionsRepoMock) RenamePetition(_ context.Context, _, itemLow uint32, name string) error {
	m.renamed.id = itemLow
	m.renamed.name = name
	if m.petition != nil {
		m.petition.Name = name
	}
	return nil
}

func (m *petitionsRepoMock) DeletePetition(_ context.Context, _, itemLow uint32) error {
	m.deletedID = itemLow
	return nil
}

func (m *petitionsRepoMock) CharacterBriefByGUID(_ context.Context, _ uint32, charGUID uint64) (*repo.CharacterBrief, error) {
	if m.chars == nil {
		return nil, nil
	}
	return m.chars[charGUID], nil
}

func (m *petitionsRepoMock) HasGuildInvite(_ context.Context, _ uint32, charGUID uint64) (bool, error) {
	if m.invites == nil {
		return false, nil
	}
	return m.invites[charGUID], nil
}

func (m *petitionsRepoMock) UpsertGuildPetition(_ context.Context, _ uint32, p *repo.Petition) error {
	m.petition = p
	return nil
}

func (m *petitionsRepoMock) GuildNameExists(_ context.Context, _ uint32, _ string) (bool, error) {
	return false, nil
}

func (m *petitionsRepoMock) AddSignatureIfUnderLimit(_ context.Context, _ uint32, ownerLow, itemLow, playerLow, accountID, maxSigns uint32) (bool, bool, error) {
	for _, sig := range m.signatures {
		if sig.PlayerGUID == uint64(playerLow) || sig.AccountID == accountID {
			return false, true, nil
		}
	}
	if uint32(len(m.signatures))+1 > maxSigns {
		return false, false, nil
	}
	m.addCalls++
	m.lastAdd.ownerLow = ownerLow
	m.lastAdd.itemLow = itemLow
	m.lastAdd.playerLow = playerLow
	m.lastAdd.accountID = accountID
	m.signatures = append(m.signatures, repo.PetitionSignature{
		PlayerGUID: uint64(playerLow),
		AccountID:  accountID,
	})
	return true, false, nil
}

func itemGUID(low uint32) uint64 {
	return guid.NewFromCounter(guid.Item, guid.LowType(low)).GetRawValue()
}

func guildPetitionFixture() *repo.Petition {
	return &repo.Petition{
		ItemLow:   100,
		OwnerGUID: 1,
		Name:      "TestGuild",
		Type:      repo.GuildCharterType,
	}
}

func TestAddSignatureOK(t *testing.T) {
	repoMock := &petitionsRepoMock{
		petition: guildPetitionFixture(),
		chars: map[uint64]*repo.CharacterBrief{
			2: {GUID: 2, AccountID: 20, Race: 1},
		},
	}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	result, owner, err := svc.AddSignature(context.Background(), 1, itemGUID(100), 2, 20)
	require.NoError(t, err)
	assert.Equal(t, PetitionSignOK, result)
	assert.Equal(t, uint64(1), owner)
	assert.Equal(t, 1, repoMock.addCalls)
	assert.Equal(t, uint32(100), repoMock.lastAdd.itemLow)
	assert.Equal(t, uint32(2), repoMock.lastAdd.playerLow)
	assert.Equal(t, uint32(20), repoMock.lastAdd.accountID)
}

func TestAddSignatureRejectsOwner(t *testing.T) {
	repoMock := &petitionsRepoMock{petition: guildPetitionFixture()}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	result, _, err := svc.AddSignature(context.Background(), 1, itemGUID(100), 1, 1)
	require.NoError(t, err)
	assert.Equal(t, PetitionSignCantSignOwn, result)
	assert.Equal(t, 0, repoMock.addCalls)
}

func TestAddSignatureRejectsDuplicateAccount(t *testing.T) {
	repoMock := &petitionsRepoMock{
		petition: guildPetitionFixture(),
		signatures: []repo.PetitionSignature{
			{PlayerGUID: 9, AccountID: 20},
		},
		chars: map[uint64]*repo.CharacterBrief{
			2: {GUID: 2, AccountID: 20, Race: 1},
		},
	}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	result, _, err := svc.AddSignature(context.Background(), 1, itemGUID(100), 2, 20)
	require.NoError(t, err)
	assert.Equal(t, PetitionSignAlreadySigned, result)
	assert.Equal(t, 0, repoMock.addCalls)
}

func TestAddSignatureRejectsAlreadyInGuild(t *testing.T) {
	repoMock := &petitionsRepoMock{
		petition: guildPetitionFixture(),
		chars: map[uint64]*repo.CharacterBrief{
			2: {GUID: 2, AccountID: 20, GuildID: 5},
		},
	}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	result, _, err := svc.AddSignature(context.Background(), 1, itemGUID(100), 2, 20)
	require.NoError(t, err)
	assert.Equal(t, PetitionSignAlreadyInGuild, result)
}

func TestAddSignatureRejectsMaxSignatures(t *testing.T) {
	sigs := make([]repo.PetitionSignature, 9)
	for i := range sigs {
		sigs[i] = repo.PetitionSignature{PlayerGUID: uint64(10 + i), AccountID: uint32(100 + i)}
	}
	repoMock := &petitionsRepoMock{
		petition:   guildPetitionFixture(),
		signatures: sigs,
		chars: map[uint64]*repo.CharacterBrief{
			2: {GUID: 2, AccountID: 20},
		},
	}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	result, _, err := svc.AddSignature(context.Background(), 1, itemGUID(100), 2, 20)
	require.NoError(t, err)
	assert.Equal(t, PetitionSignMaxSignatures, result)
}

func TestAddSignatureRejectsArenaPetition(t *testing.T) {
	p := guildPetitionFixture()
	p.Type = 2 // arena 2v2
	repoMock := &petitionsRepoMock{petition: p}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	result, _, err := svc.AddSignature(context.Background(), 1, itemGUID(100), 2, 20)
	require.NoError(t, err)
	assert.Equal(t, PetitionSignNotGuild, result)
	assert.Equal(t, 0, repoMock.addCalls)
}

func TestGetSignatures(t *testing.T) {
	repoMock := &petitionsRepoMock{
		petition: guildPetitionFixture(),
		signatures: []repo.PetitionSignature{
			{PlayerGUID: 2, AccountID: 20},
			{PlayerGUID: 3, AccountID: 30},
		},
	}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	p, sigs, err := svc.GetSignatures(context.Background(), 1, itemGUID(100))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "TestGuild", p.Name)
	assert.Len(t, sigs, 2)
}

func TestValidateTurnInNeedMoreSignatures(t *testing.T) {
	repoMock := &petitionsRepoMock{
		petition: guildPetitionFixture(),
		signatures: []repo.PetitionSignature{
			{PlayerGUID: 2, AccountID: 20},
		},
		chars: map[uint64]*repo.CharacterBrief{
			1: {GUID: 1, AccountID: 1},
		},
	}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	status, name, sigs, err := svc.ValidateTurnIn(context.Background(), 1, itemGUID(100), 1)
	require.NoError(t, err)
	assert.Equal(t, TurnInNeedMoreSignatures, status)
	assert.Equal(t, "TestGuild", name)
	assert.Nil(t, sigs)
}

func TestValidateTurnInOK(t *testing.T) {
	sigs := make([]repo.PetitionSignature, 9)
	for i := range sigs {
		sigs[i] = repo.PetitionSignature{PlayerGUID: uint64(10 + i), AccountID: uint32(100 + i)}
	}
	repoMock := &petitionsRepoMock{
		petition:   guildPetitionFixture(),
		signatures: sigs,
		chars: map[uint64]*repo.CharacterBrief{
			1: {GUID: 1, AccountID: 1},
		},
	}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	status, name, out, err := svc.ValidateTurnIn(context.Background(), 1, itemGUID(100), 1)
	require.NoError(t, err)
	assert.Equal(t, TurnInOK, status)
	assert.Equal(t, "TestGuild", name)
	assert.Len(t, out, 9)
}

func TestValidateTurnInNotOwner(t *testing.T) {
	repoMock := &petitionsRepoMock{petition: guildPetitionFixture()}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	status, _, _, err := svc.ValidateTurnIn(context.Background(), 1, itemGUID(100), 99)
	require.NoError(t, err)
	assert.Equal(t, TurnInNotOwner, status)
}

func TestValidateTurnInArena(t *testing.T) {
	p := guildPetitionFixture()
	p.Type = 3
	repoMock := &petitionsRepoMock{petition: p}
	svc := NewPetitionService(repoMock, 9, events.NoopPetitionServiceProducer{})

	status, _, _, err := svc.ValidateTurnIn(context.Background(), 1, itemGUID(100), 1)
	require.NoError(t, err)
	assert.Equal(t, TurnInNotGuild, status)
}
