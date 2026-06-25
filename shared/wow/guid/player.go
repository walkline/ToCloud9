package guid

type PlayerUnwrapped struct {
	RealmID uint16
	LowGUID LowType
}

// Player GUID convention:
//   - Persistent ToCloud9 service keys use (realmID, low player DB guid).
//   - Same-realm client/AzerothCore player ObjectGuid values are serialized as
//     the low DB guid.
//   - Foreign-realm client/AzerothCore player ObjectGuid values are serialized
//     as realm-scoped player GUIDs: realmID << 32 | low DB guid.
//   - Already encoded non-player ObjectGuid values are preserved unchanged.
//
// Use these helpers at service, gateway, and libsidecar boundaries instead of
// open-coding shifts/masks. Do not normalize already serialized client packets
// before forwarding them to the owning worldserver.

func NewPlayerUnwrappedFromRawGUID(g ObjectGuid) PlayerUnwrapped {
	// TODO: we probably should validate guid here
	return PlayerUnwrapped{
		RealmID: g.GetRealmID(),
		LowGUID: g.GetCounter(),
	}
}

func NewPlayerUnwrapped(realm uint16, low uint32) PlayerUnwrapped {
	return PlayerUnwrapped{
		RealmID: realm,
		LowGUID: LowType(low),
	}
}

func (u PlayerUnwrapped) Wrap() ObjectGuid {
	return NewCrossrealmPlayerGUID(u.RealmID, u.LowGUID)
}

func PlayerRealmIDOrDefault(defaultRealmID uint32, playerGUID uint64) uint32 {
	if playerGUID == 0 || playerGUID>>48 != 0 {
		return defaultRealmID
	}
	if realmID := uint32((playerGUID >> 32) & 0xffff); realmID != 0 {
		return realmID
	}
	return defaultRealmID
}

func PlayerLowGUID(playerGUID uint64) uint64 {
	if playerGUID == 0 || playerGUID>>48 != 0 {
		return playerGUID
	}
	if playerGUID>>32 == 0 {
		return playerGUID
	}
	return playerGUID & 0xffffffff
}

func PlayerGUIDForRealm(groupRealmID, playerRealmID uint32, playerGUID uint64) uint64 {
	if playerGUID == 0 || playerGUID>>48 != 0 {
		return playerGUID
	}

	lowGUID := PlayerLowGUID(playerGUID)
	if playerRealmID == 0 {
		playerRealmID = PlayerRealmIDOrDefault(groupRealmID, playerGUID)
	}
	if playerRealmID == 0 || playerRealmID == groupRealmID {
		return lowGUID
	}

	return NewCrossrealmPlayerGUID(uint16(playerRealmID), LowType(lowGUID)).GetRawValue()
}

func NormalizePlayerGUIDForRealm(defaultRealmID uint32, playerGUID uint64) uint64 {
	if playerGUID == 0 || playerGUID>>48 != 0 {
		return playerGUID
	}
	realmID := uint32((playerGUID >> 32) & 0xffff)
	if realmID == 0 || realmID == defaultRealmID {
		return PlayerLowGUID(playerGUID)
	}
	return playerGUID
}

func SamePlayer(defaultRealmA uint32, playerA uint64, defaultRealmB uint32, playerB uint64) bool {
	if playerA == 0 || playerB == 0 {
		return false
	}
	return PlayerRealmIDOrDefault(defaultRealmA, playerA) == PlayerRealmIDOrDefault(defaultRealmB, playerB) &&
		PlayerLowGUID(playerA) == PlayerLowGUID(playerB)
}
