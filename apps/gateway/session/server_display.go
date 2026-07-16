package session

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	root "github.com/walkline/ToCloud9/apps/gateway"
	panel "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

func (s *GameSession) accountGMLevel(ctx context.Context) (uint32, error) {
	if s.authDB == nil {
		return 0, nil
	}
	var level uint32
	err := s.authDB.QueryRowContext(ctx, "SELECT COALESCE(MAX(gmlevel), 0) FROM account_access WHERE id = ? AND (RealmID = -1 OR RealmID = ?)", s.accountID, root.RealmID).Scan(&level)
	return level, err
}

func friendlyGameServer(server *panel.Server) string {
	if server == nil {
		return "another server"
	}
	number := "1"
	parts := strings.FieldsFunc(server.ID, func(r rune) bool { return !unicode.IsDigit(r) })
	if len(parts) > 0 {
		if _, err := strconv.ParseUint(parts[len(parts)-1], 10, 32); err == nil {
			number = parts[len(parts)-1]
		}
	}
	if server.LayerID > 0 {
		return fmt.Sprintf("server %s layer %d", number, server.LayerID)
	}
	return fmt.Sprintf("server %s", number)
}
