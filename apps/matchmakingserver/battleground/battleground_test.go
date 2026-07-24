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
		{"balanced full match accepts nobody", 5, 5, 0, 0},
		{"short team refills up to min", 4, 5, 1, 0},
		{"both short refill to min", 3, 4, 2, 1},
		{"team refills up to bigger opposite team", 5, 8, 3, 0},
		{"never past max", 5, 10, 5, 0},
		{"balanced above min accepts nobody", 8, 8, 0, 0},
		{"empty bg refills to min", 0, 0, 5, 5},
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
	bg := bgWithCounts(5, 10, 4, 5)
	bg.InvitedPlayersPerTeam[TeamAlliance] = append(bg.InvitedPlayersPerTeam[TeamAlliance], InvitedPlayer{GUID: guid.PlayerUnwrapped{RealmID: 1, LowGUID: 50}})

	if got := bg.BackfillSlotsForTeam(TeamAlliance); got != 0 {
		t.Errorf("alliance slots = %d, want 0 (invited player holds the slot)", got)
	}
}
