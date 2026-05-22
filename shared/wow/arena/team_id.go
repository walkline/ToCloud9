package arena

const (
	teamIDRealmShift = 24
	teamIDLowMask    = uint32(0x00FFFFFF)
)

func NewCrossrealmTeamID(realmID uint16, teamID uint32) uint32 {
	if realmID == 0 || teamID == 0 {
		return teamID
	}
	return uint32(realmID)<<teamIDRealmShift | (teamID & teamIDLowMask)
}

func TeamIDRealmID(teamID uint32) uint32 {
	return teamID >> teamIDRealmShift
}

func TeamIDCounter(teamID uint32) uint32 {
	if TeamIDRealmID(teamID) == 0 {
		return teamID
	}
	return teamID & teamIDLowMask
}
