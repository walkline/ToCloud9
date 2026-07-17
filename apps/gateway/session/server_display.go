package session

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	panel "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

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

func (s *GameSession) sendLayerSwitchStarted(server *panel.Server) {
	label := friendlyGameServer(server)
	if s.showSensitiveServerInformation {
		serverLabel := strings.TrimSuffix(label, fmt.Sprintf(" layer %d", server.LayerID))
		s.SendSysMessage(fmt.Sprintf("Switching to layer %d on %s (%s).", server.LayerID, serverLabel, server.Address))
		return
	}
	s.SendSysMessage(fmt.Sprintf("Switching to layer %d.", server.LayerID))
}

func (s *GameSession) sendLayerSwitchCompleted(server *panel.Server) {
	label := friendlyGameServer(server)
	if s.showSensitiveServerInformation {
		s.SendSysMessage(fmt.Sprintf("Movement resumed on %s (%s).", label, server.Address))
		return
	}
	s.SendSysMessage(fmt.Sprintf("Switched to layer %d.", server.LayerID))
}
