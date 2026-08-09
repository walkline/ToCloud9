package service

import (
	"context"
	"fmt"

	"github.com/walkline/ToCloud9/apps/charserver"
	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

// DefaultMinPetitionSigns matches AC MinPetitionSigns default.
const DefaultMinPetitionSigns = 9

// Petition sign results (AC PetitionSigns + gateway-mapped extras).
const (
	PetitionSignOK                    uint32 = 0
	PetitionSignAlreadySigned         uint32 = 1
	PetitionSignAlreadyInGuild        uint32 = 2
	PetitionSignCantSignOwn           uint32 = 3
	PetitionSignNotFound              uint32 = 4
	PetitionSignNotGuild              uint32 = 5
	PetitionSignMaxSignatures         uint32 = 6
	PetitionSignAlreadyInvitedToGuild uint32 = 7
)

// PetitionService is the cluster authority for guild charter petitions.
type PetitionService interface {
	GetPetition(ctx context.Context, realmID uint32, petitionItemGUID uint64) (*repo.Petition, error)
	GetSignatures(ctx context.Context, realmID uint32, petitionItemGUID uint64) (*repo.Petition, []repo.PetitionSignature, error)
	AddSignature(ctx context.Context, realmID uint32, petitionItemGUID, signerGUID uint64, signerAccountID uint32) (result uint32, ownerGUID uint64, err error)
	Rename(ctx context.Context, realmID uint32, petitionItemGUID, ownerGUID uint64, newName string) (ok bool, notGuild bool, notOwner bool, err error)
	Delete(ctx context.Context, realmID uint32, petitionItemGUID uint64) (found bool, err error)
	ValidateTurnIn(ctx context.Context, realmID uint32, petitionItemGUID, playerGUID uint64) (status TurnInStatus, name string, signatories []uint64, err error)
	UpsertGuildPetition(ctx context.Context, realmID uint32, ownerGUID, petitionItemGUID uint64, name string) error
	GuildNameExists(ctx context.Context, realmID uint32, name string) (bool, error)
}

type TurnInStatus int

const (
	TurnInOK TurnInStatus = iota
	TurnInNotFound
	TurnInNotOwner
	TurnInNotGuild
	TurnInNeedMoreSignatures
	TurnInAlreadyInGuild
)

type petitionServiceImpl struct {
	repo           repo.Petitions
	minSigns       uint32
	eventsProducer events.PetitionServiceProducer
}

func NewPetitionService(petitions repo.Petitions, minSigns uint32, eventsProducer events.PetitionServiceProducer) PetitionService {
	if minSigns == 0 {
		minSigns = DefaultMinPetitionSigns
	}
	if minSigns > 9 {
		minSigns = 9
	}
	return &petitionServiceImpl{
		repo:           petitions,
		minSigns:       minSigns,
		eventsProducer: eventsProducer,
	}
}

func itemLow(petitionItemGUID uint64) uint32 {
	return uint32(guid.New(petitionItemGUID).GetCounter())
}

func playerLow(playerGUID uint64) uint32 {
	return uint32(guid.New(playerGUID).GetCounter())
}

func (s *petitionServiceImpl) loadGuildPetition(ctx context.Context, realmID uint32, petitionItemGUID uint64) (*repo.Petition, error) {
	p, err := s.repo.GetPetitionByItemLow(ctx, realmID, itemLow(petitionItemGUID))
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *petitionServiceImpl) GetPetition(ctx context.Context, realmID uint32, petitionItemGUID uint64) (*repo.Petition, error) {
	return s.loadGuildPetition(ctx, realmID, petitionItemGUID)
}

func (s *petitionServiceImpl) GetSignatures(ctx context.Context, realmID uint32, petitionItemGUID uint64) (*repo.Petition, []repo.PetitionSignature, error) {
	p, err := s.loadGuildPetition(ctx, realmID, petitionItemGUID)
	if err != nil || p == nil {
		return p, nil, err
	}
	if p.Type != repo.GuildCharterType {
		return p, nil, nil
	}
	sigs, err := s.repo.GetSignaturesByItemLow(ctx, realmID, p.ItemLow)
	if err != nil {
		return nil, nil, err
	}
	return p, sigs, nil
}

func (s *petitionServiceImpl) AddSignature(ctx context.Context, realmID uint32, petitionItemGUID, signerGUID uint64, signerAccountID uint32) (uint32, uint64, error) {
	p, err := s.loadGuildPetition(ctx, realmID, petitionItemGUID)
	if err != nil {
		return PetitionSignNotFound, 0, err
	}
	if p == nil {
		return PetitionSignNotFound, 0, nil
	}
	if p.Type != repo.GuildCharterType {
		return PetitionSignNotGuild, p.OwnerGUID, nil
	}

	signerLow := playerLow(signerGUID)
	if p.OwnerGUID == uint64(signerLow) || p.OwnerGUID == signerGUID {
		return PetitionSignCantSignOwn, p.OwnerGUID, nil
	}

	signer, err := s.repo.CharacterBriefByGUID(ctx, realmID, uint64(signerLow))
	if err != nil {
		return PetitionSignNotFound, p.OwnerGUID, err
	}
	if signer == nil {
		return PetitionSignNotFound, p.OwnerGUID, nil
	}
	if signer.GuildID != 0 {
		return PetitionSignAlreadyInGuild, p.OwnerGUID, nil
	}

	invited, err := s.repo.HasGuildInvite(ctx, realmID, uint64(signerLow))
	if err != nil {
		return PetitionSignNotFound, p.OwnerGUID, err
	}
	if invited {
		return PetitionSignAlreadyInvitedToGuild, p.OwnerGUID, nil
	}

	if signerAccountID == 0 {
		signerAccountID = signer.AccountID
	}

	// AC rejects when signature count would exceed petition type (9 for guild).
	maxSigns := uint32(p.Type)
	if maxSigns == 0 {
		maxSigns = uint32(repo.GuildCharterType)
	}
	inserted, alreadySigned, err := s.repo.AddSignatureIfUnderLimit(ctx, realmID, uint32(p.OwnerGUID), p.ItemLow, signerLow, signerAccountID, maxSigns)
	if err != nil {
		return PetitionSignNotFound, p.OwnerGUID, fmt.Errorf("add signature: %w", err)
	}
	if alreadySigned {
		s.publishSignResult(realmID, petitionItemGUID, p.OwnerGUID, uint64(signerLow), PetitionSignAlreadySigned)
		return PetitionSignAlreadySigned, p.OwnerGUID, nil
	}
	if !inserted {
		return PetitionSignMaxSignatures, p.OwnerGUID, nil
	}

	s.publishSignResult(realmID, petitionItemGUID, p.OwnerGUID, uint64(signerLow), PetitionSignOK)
	return PetitionSignOK, p.OwnerGUID, nil
}

func (s *petitionServiceImpl) publishSignResult(realmID uint32, petitionItemGUID, ownerGUID, signerGUID uint64, result uint32) {
	if s.eventsProducer == nil {
		return
	}
	_ = s.eventsProducer.SignResult(&events.PetitionEventSignResultPayload{
		ServiceID:        charserver.ServiceID,
		RealmID:          realmID,
		PetitionItemGUID: petitionItemGUID,
		OwnerGUID:        ownerGUID,
		SignerGUID:       signerGUID,
		Result:           result,
	})
}

func (s *petitionServiceImpl) Rename(ctx context.Context, realmID uint32, petitionItemGUID, ownerGUID uint64, newName string) (bool, bool, bool, error) {
	p, err := s.loadGuildPetition(ctx, realmID, petitionItemGUID)
	if err != nil {
		return false, false, false, err
	}
	if p == nil {
		return false, false, false, nil
	}
	if p.Type != repo.GuildCharterType {
		return false, true, false, nil
	}
	ownerLow := playerLow(ownerGUID)
	if p.OwnerGUID != uint64(ownerLow) && p.OwnerGUID != ownerGUID {
		return false, false, true, nil
	}
	if err = s.repo.RenamePetition(ctx, realmID, p.ItemLow, newName); err != nil {
		return false, false, false, err
	}
	return true, false, false, nil
}

func (s *petitionServiceImpl) Delete(ctx context.Context, realmID uint32, petitionItemGUID uint64) (bool, error) {
	p, err := s.loadGuildPetition(ctx, realmID, petitionItemGUID)
	if err != nil {
		return false, err
	}
	if p == nil {
		return false, nil
	}
	if err = s.repo.DeletePetition(ctx, realmID, p.ItemLow); err != nil {
		return false, err
	}
	return true, nil
}

func (s *petitionServiceImpl) UpsertGuildPetition(ctx context.Context, realmID uint32, ownerGUID, petitionItemGUID uint64, name string) error {
	return s.repo.UpsertGuildPetition(ctx, realmID, &repo.Petition{
		ItemLow:   itemLow(petitionItemGUID),
		OwnerGUID: uint64(playerLow(ownerGUID)),
		Name:      name,
		Type:      repo.GuildCharterType,
	})
}

func (s *petitionServiceImpl) GuildNameExists(ctx context.Context, realmID uint32, name string) (bool, error) {
	return s.repo.GuildNameExists(ctx, realmID, name)
}

func (s *petitionServiceImpl) ValidateTurnIn(ctx context.Context, realmID uint32, petitionItemGUID, playerGUID uint64) (TurnInStatus, string, []uint64, error) {
	p, err := s.loadGuildPetition(ctx, realmID, petitionItemGUID)
	if err != nil {
		return TurnInNotFound, "", nil, err
	}
	if p == nil {
		return TurnInNotFound, "", nil, nil
	}
	if p.Type != repo.GuildCharterType {
		return TurnInNotGuild, "", nil, nil
	}

	ownerLow := playerLow(playerGUID)
	if p.OwnerGUID != uint64(ownerLow) && p.OwnerGUID != playerGUID {
		return TurnInNotOwner, "", nil, nil
	}

	owner, err := s.repo.CharacterBriefByGUID(ctx, realmID, uint64(ownerLow))
	if err != nil {
		return TurnInNotFound, "", nil, err
	}
	if owner != nil && owner.GuildID != 0 {
		return TurnInAlreadyInGuild, "", nil, nil
	}

	sigs, err := s.repo.GetSignaturesByItemLow(ctx, realmID, p.ItemLow)
	if err != nil {
		return TurnInNotFound, "", nil, err
	}
	if uint32(len(sigs)) < s.minSigns {
		return TurnInNeedMoreSignatures, p.Name, nil, nil
	}

	signatories := make([]uint64, 0, len(sigs))
	for _, sig := range sigs {
		signatories = append(signatories, sig.PlayerGUID)
	}
	return TurnInOK, p.Name, signatories, nil
}
