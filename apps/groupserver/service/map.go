package service

type MapID int

func (m MapID) IsDungeon() bool {
	_, found := dungeonMaps[int(m)]
	return found
}

func (m MapID) IsRaid() bool {
	_, found := raidMaps[int(m)]
	return found
}

// dungeonMaps dictionary with map IDs of non raid dungeons.
var dungeonMaps = map[int]struct{}{
	33:  {},
	34:  {},
	36:  {},
	43:  {},
	44:  {},
	47:  {},
	48:  {},
	70:  {},
	90:  {},
	109: {},
	129: {},
	189: {},
	209: {},
	229: {},
	230: {},
	269: {},
	289: {},
	329: {},
	349: {},
	389: {},
	429: {},
	540: {},
	542: {},
	543: {},
	545: {},
	546: {},
	547: {},
	552: {},
	553: {},
	554: {},
	555: {},
	556: {},
	557: {},
	558: {},
	560: {},
	574: {},
	575: {},
	576: {},
	578: {},
	585: {},
	595: {},
	598: {},
	599: {},
	600: {},
	601: {},
	602: {},
	604: {},
	608: {},
	619: {},
	632: {},
	650: {},
	658: {},
	668: {},
}

// raidMaps dictionary with map IDs of raid dungeons.
var raidMaps = map[int]struct{}{
	169: {},
	249: {},
	309: {},
	409: {},
	469: {},
	509: {},
	531: {},
	532: {},
	533: {},
	534: {},
	544: {},
	548: {},
	550: {},
	564: {},
	565: {},
	568: {},
	580: {},
	603: {},
	615: {},
	616: {},
	624: {},
	631: {},
	649: {},
	724: {},
}
