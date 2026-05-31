package wow

const (
	ClassWarrior     uint8 = 1
	ClassRogue       uint8 = 4
	ClassDeathKnight uint8 = 6

	PowerTypeMana       uint8 = 0
	PowerTypeRage       uint8 = 1
	PowerTypeEnergy     uint8 = 3
	PowerTypeRunicPower uint8 = 6
)

func FixedPrimaryPowerTypeForClass(classID uint8) (uint8, bool) {
	switch classID {
	case ClassWarrior:
		return PowerTypeRage, true
	case ClassRogue:
		return PowerTypeEnergy, true
	case ClassDeathKnight:
		return PowerTypeRunicPower, true
	default:
		return PowerTypeMana, false
	}
}

func DefaultMaxPowerForClass(classID uint8) uint32 {
	switch classID {
	case ClassWarrior, ClassDeathKnight:
		return 1000
	case ClassRogue:
		return 100
	default:
		return 0
	}
}

func IsFixedClassInactivePowerType(classID, powerType uint8) bool {
	expectedPowerType, fixed := FixedPrimaryPowerTypeForClass(classID)
	if !fixed || powerType == expectedPowerType {
		return false
	}

	switch powerType {
	case PowerTypeRage, PowerTypeEnergy, PowerTypeRunicPower:
		return true
	default:
		return false
	}
}
