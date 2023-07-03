package guid

type HighGuid uint16

const (
	Item          HighGuid = 0x4000
	Container     HighGuid = 0x4000
	Player        HighGuid = 0x0000
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

func NewObjectGuid() ObjectGuid {
	return ObjectGuid{}
}

func NewObjectGuidFromUint64(guid uint64) ObjectGuid {
	return ObjectGuid{guid: guid}
}

func NewObjectGuidFromValues(hi HighGuid, entry uint32, counter LowType) ObjectGuid {
	var guid uint64
	if counter != 0 {
		guid = uint64(counter) | (uint64(entry) << 24) | (uint64(hi) << 48)
	}
	return ObjectGuid{guid: guid}
}

func NewObjectGuidFromCounter(hi HighGuid, counter LowType) ObjectGuid {
	var guid uint64
	if counter != 0 {
		guid = uint64(counter) | (uint64(hi) << 48)
	}
	return ObjectGuid{guid: guid}
}

func (og ObjectGuid) GetRawValue() uint64 {
	return og.guid
}

func (og ObjectGuid) GetHigh() HighGuid {
	return HighGuid((og.guid >> 48) & 0x0000FFFF)
}

func (og ObjectGuid) GetEntry() uint32 {
	if og.HasEntry(og.GetHigh()) {
		return uint32((og.guid >> 24) & 0x0000000000FFFFFF)
	}
	return 0
}

func (og ObjectGuid) GetCounter() LowType {
	if og.HasEntry(og.GetHigh()) {
		return LowType(og.guid & 0x0000000000FFFFFF)
	}
	return LowType(og.guid & 0x00000000FFFFFFFF)
}

func (og ObjectGuid) GetMaxCounter(high HighGuid) LowType {
	if og.HasEntry(high) {
		return LowType(0x00FFFFFF)
	}
	return LowType(0xFFFFFFFF)
}

func (og ObjectGuid) HasEntry(high HighGuid) bool {
	return high != Player
}
