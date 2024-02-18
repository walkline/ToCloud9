package repo

import "strings"

type SupportedSchemaType string

const (
	SupportedSchemaTypeTrinityCore SupportedSchemaType = "tc"
	SupportedSchemaTypeAzerothCore SupportedSchemaType = "ac"
	SupportedSchemaTypeCMaNGOS     SupportedSchemaType = "cm"
)

func ParseSchemaType(s string) SupportedSchemaType {
	switch strings.ToLower(s) {
	case "tc", "trinity", "trinitycore":
		return SupportedSchemaTypeTrinityCore
	case "ac", "acore", "azeroth", "azerothcore":
		return SupportedSchemaTypeAzerothCore
	case "cm", "cmangos", "continued mangos":
		return SupportedSchemaTypeCMaNGOS
	}
	return SupportedSchemaTypeTrinityCore
}
