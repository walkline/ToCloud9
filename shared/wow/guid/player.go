package guid

type PlayerUnwrapped struct {
	RealmID uint16
	LowGUID LowType
}

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
