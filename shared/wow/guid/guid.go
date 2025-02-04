package guid

type HighGuid uint16

const (
	Player        HighGuid = 0x0000
	Item          HighGuid = 0x4000
	Container     HighGuid = 0x4000
	GameObject    HighGuid = 0xF110
	Transport     HighGuid = 0xF120
	Unit          HighGuid = 0xF130
	Pet           HighGuid = 0xF140
	Vehicle       HighGuid = 0xF150
	DynamicObject HighGuid = 0xF100
	Corpse        HighGuid = 0xF101
	MoTransport   HighGuid = 0x1FC0
	Instance      HighGuid = 0x1F40
	Group         HighGuid = 0x1F50
)

type ObjectGuid struct {
	guid uint64
}

type LowType uint32

func New(raw uint64) ObjectGuid {
	return ObjectGuid{
		guid: raw,
	}
}

func NewFromCounter(hi HighGuid, counter LowType) ObjectGuid {
	var guid uint64
	if counter != 0 {
		guid = uint64(counter) | (uint64(hi) << 48)
	}
	return ObjectGuid{guid: guid}
}

func NewCrossrealmPlayerGUID(realmID uint16, counter LowType) ObjectGuid {
	var guid uint64
	if counter != 0 {
		guid = uint64(counter) | (uint64(realmID) << 32) | (uint64(Player) << 48)
	}
	return ObjectGuid{guid: guid}
}

func NewFromEntryAndCounter(hi HighGuid, entry uint32, counter LowType) ObjectGuid {
	var guid uint64
	if counter != 0 {
		guid = uint64(counter) | (uint64(entry) << 24) | (uint64(hi) << 48)
	}
	return ObjectGuid{guid: guid}
}

func (g ObjectGuid) GetRawValue() uint64 {
	return g.guid
}

func (g ObjectGuid) GetHigh() HighGuid {
	return HighGuid((g.guid >> 48) & 0x0000FFFF)
}

func (g ObjectGuid) GetEntry() uint32 {
	if g.HasEntry(g.GetHigh()) {
		return uint32((g.guid >> 24) & 0x0000000000FFFFFF)
	}
	return 0
}

func (g ObjectGuid) GetCounter() LowType {
	if g.HasEntry(g.GetHigh()) {
		return LowType(g.guid & 0x0000000000FFFFFF)
	}
	return LowType(g.guid & 0x00000000FFFFFFFF)
}

func (g ObjectGuid) GetRealmID() uint16 {
	return uint16((g.guid >> 32) & 0xFFFF)
}

func (g ObjectGuid) GetMaxCounter(high HighGuid) LowType {
	if g.HasEntry(high) {
		return LowType(0x00FFFFFF)
	}
	return LowType(0xFFFFFFFF)
}

func (g ObjectGuid) HasEntry(high HighGuid) bool {
	return high != Player
}
