package gateway

var RealmID uint32

var RetrievedGatewayID string

// AllowTwoSideInteractionGuild mirrors AzerothCore's
// AllowTwoSide.Interaction.Guild for gateway-owned guild operations.
var AllowTwoSideInteractionGuild bool

// AllowTwoSideInteractionChannel mirrors AzerothCore's
// AllowTwoSide.Interaction.Channel for gateway-owned channel operations.
var AllowTwoSideInteractionChannel bool

// AllowTwoSideInteractionArena mirrors AzerothCore's
// AllowTwoSide.Interaction.Arena for gateway-owned arena-team operations.
var AllowTwoSideInteractionArena bool

// MaxPlayerLevel mirrors AzerothCore's MaxPlayerLevel for gateway-owned checks.
var MaxPlayerLevel uint32

// ArenaCurrentSeason mirrors AzerothCore ArenaSeasonMgr for gateway-owned arena charter creation.
var ArenaCurrentSeason uint32

// LegacyArenaStartRating mirrors AzerothCore's legacy arena start rating.
var LegacyArenaStartRating uint32

// ArenaStartRating mirrors AzerothCore's current arena start rating.
var ArenaStartRating uint32

// ArenaStartPersonalRating mirrors AzerothCore's Arena.StartPersonalRating.
var ArenaStartPersonalRating uint32

// ArenaStartMatchmakerRating mirrors AzerothCore's Arena.StartMatchmakerRating.
var ArenaStartMatchmakerRating uint32

func EffectiveArenaStartRating() uint32 {
	if ArenaCurrentSeason < 6 {
		return LegacyArenaStartRating
	}
	return ArenaStartRating
}

func EffectiveArenaStartPersonalRating(teamRating uint32) uint32 {
	if ArenaStartPersonalRating > 0 {
		return ArenaStartPersonalRating
	}
	if ArenaCurrentSeason < 6 {
		return 1500
	}
	if teamRating >= 1000 {
		return 1000
	}
	return 0
}

const (
	Ver                            = "0.0.1"
	SupportedCharServiceVer        = "0.0.1"
	SupportedServerRegistryVer     = "0.0.1"
	SupportedMailServiceVer        = "0.0.1"
	SupportedGroupServiceVer       = "0.0.1"
	SupportedMatchmakingServiceVer = "0.0.1"
	SupportedGameServerVer         = "0.0.1"
	SupportedAuctionHouseVer       = "0.0.1"
)
