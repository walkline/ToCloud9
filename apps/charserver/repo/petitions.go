package repo

import "context"

// GuildCharterType is AC GUILD_CHARTER_TYPE (also the max signature count).
const GuildCharterType uint8 = 9

// Petition is a characters.petition row. ItemLow is petitionguid (item counter).
type Petition struct {
	ItemLow   uint32
	OwnerGUID uint64
	Name      string
	Type      uint8
}

// PetitionSignature is a petition_sign row.
type PetitionSignature struct {
	PlayerGUID uint64
	AccountID  uint32
}

// CharacterBrief is a minimal character row for petition checks.
type CharacterBrief struct {
	GUID      uint64
	AccountID uint32
	Race      uint8
	GuildID   uint32
	Name      string
}

// Petitions stores guild charter petitions and signatures.
type Petitions interface {
	GetPetitionByItemLow(ctx context.Context, realmID, itemLow uint32) (*Petition, error)
	GetSignaturesByItemLow(ctx context.Context, realmID, itemLow uint32) ([]PetitionSignature, error)
	RenamePetition(ctx context.Context, realmID, itemLow uint32, name string) error
	DeletePetition(ctx context.Context, realmID, itemLow uint32) error
	CharacterBriefByGUID(ctx context.Context, realmID uint32, charGUID uint64) (*CharacterBrief, error)
	// HasGuildInvite reports a pending guild invite. Unknown-table errors are not returned (fail open).
	HasGuildInvite(ctx context.Context, realmID uint32, charGUID uint64) (bool, error)
	// UpsertGuildPetition replaces the owner's guild charter row and its signatures.
	UpsertGuildPetition(ctx context.Context, realmID uint32, p *Petition) error
	GuildNameExists(ctx context.Context, realmID uint32, name string) (bool, error)
	// AddSignatureIfUnderLimit inserts under maxSigns, or reports alreadySigned / full.
	AddSignatureIfUnderLimit(ctx context.Context, realmID uint32, ownerLow, itemLow, playerLow, accountID, maxSigns uint32) (inserted bool, alreadySigned bool, err error)
}
