package session

// localeNames holds the locales supported by the game client, indexed by the
// locale constant the world server stores in the account row.
var localeNames = [...]string{
	"enUS",
	"koKR",
	"frFR",
	"deDE",
	"zhCN",
	"zhTW",
	"esES",
	"esMX",
	"ruRU",
}

// localeEnUS is the fallback used for clients that report an unknown locale,
// enGB included.
const localeEnUS uint8 = 0

// localeByName returns the locale constant of the given locale name, or
// localeEnUS if the name is unknown.
func localeByName(name string) uint8 {
	for i := range localeNames {
		if localeNames[i] == name {
			return uint8(i)
		}
	}

	return localeEnUS
}

// localeFromChallengeCountry returns the locale constant for the country field
// of the logon challenge. The client sends it reversed, e.g. "SUne" for "enUS".
func localeFromChallengeCountry(country [4]byte) uint8 {
	name := []byte{country[3], country[2], country[1], country[0]}
	return localeByName(string(name))
}
