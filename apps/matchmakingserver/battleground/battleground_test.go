package battleground

import (
	"testing"

	"github.com/walkline/ToCloud9/shared/wow/guid"
)

func bgWithCounts(minPerTeam, maxPerTeam uint8, alliance, horde int) *Battleground {
	bg := &Battleground{
		MinPlayersPerTeam: minPerTeam,
		MaxPlayersPerTeam: maxPerTeam,
	}
	for i := 0; i < alliance; i++ {
		bg.ActivePlayersPerTeam[TeamAlliance] = append(bg.ActivePlayersPerTeam[TeamAlliance], guid.PlayerUnwrapped{RealmID: 1, LowGUID: guid.LowType(i + 1)})
	}
	for i := 0; i < horde; i++ {
		bg.ActivePlayersPerTeam[TeamHorde] = append(bg.ActivePlayersPerTeam[TeamHorde], guid.PlayerUnwrapped{RealmID: 1, LowGUID: guid.LowType(i + 100)})
	}
	return bg
}

func TestBackfillSlotsForTeam(t *testing.T) {
	cases := []struct {
		name             string
		alliance, horde  int
		expectedAlliance uint8
		expectedHorde    uint8
	}{
		{"match at min keeps filling to max", 5, 5, 5, 5},
		{"short team refills toward max", 4, 5, 6, 5},
		{"both short refill toward max", 3, 4, 7, 6},
		{"unbalanced still fills both to max", 5, 8, 5, 2},
		{"never past max", 5, 10, 5, 0},
		{"balanced above min keeps filling", 8, 8, 2, 2},
		{"empty bg refills to max", 0, 0, 10, 10},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bg := bgWithCounts(5, 10, c.alliance, c.horde)
			if got := bg.BackfillSlotsForTeam(TeamAlliance); got != c.expectedAlliance {
				t.Errorf("alliance slots = %d, want %d", got, c.expectedAlliance)
			}
			if got := bg.BackfillSlotsForTeam(TeamHorde); got != c.expectedHorde {
				t.Errorf("horde slots = %d, want %d", got, c.expectedHorde)
			}
		})
	}
}

func TestBackfillSlotsForTeamCountsInvitedPlayers(t *testing.T) {
	bg := bgWithCounts(5, 10, 9, 5)
	bg.InvitedPlayersPerTeam[TeamAlliance] = append(bg.InvitedPlayersPerTeam[TeamAlliance], InvitedPlayer{GUID: guid.PlayerUnwrapped{RealmID: 1, LowGUID: 50}})

	if got := bg.BackfillSlotsForTeam(TeamAlliance); got != 0 {
		t.Errorf("alliance slots = %d, want 0 (invited player holds the slot)", got)
	}
}
