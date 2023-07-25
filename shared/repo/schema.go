package repo

import "strings"

type SupportedSchemaType string

const (
	SupportedSchemaTypeTrinityCore SupportedSchemaType = "tc"
	SupportedSchemaTypeAzerothCore SupportedSchemaType = "ac"
)

func ParseSchemaType(s string) SupportedSchemaType {
	switch strings.ToLower(s) {
	case "tc", "trinity", "trinitycore":
		return SupportedSchemaTypeTrinityCore
	case "ac", "acore", "azeroth", "azerothcore":
		return SupportedSchemaTypeAzerothCore
	}
	return SupportedSchemaTypeTrinityCore
}
