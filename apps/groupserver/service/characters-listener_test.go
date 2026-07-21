package service

import (
	"testing"

	"github.com/walkline/ToCloud9/shared/events"
)

type loggedOutHandlerRecorder struct {
	payloads []events.GWEventCharacterLoggedOutPayload
}

func (r *loggedOutHandlerRecorder) HandleCharacterLoggedOut(p events.GWEventCharacterLoggedOutPayload) error {
	r.payloads = append(r.payloads, p)
	return nil
}

func TestCharactersListenerFansOutDisconnectedChars(t *testing.T) {
	h1 := &loggedOutHandlerRecorder{}
	h2 := &loggedOutHandlerRecorder{}
	l := NewCharactersListener(nil, h1, h2)

	l.handleDisconnectedChars(&events.CharEventCharsDisconnectedUnhealthyGWPayload{
		RealmID:        1,
		GatewayID:      "gw-1",
		CharactersGUID: []uint64{11, 22},
	})

	for _, h := range []*loggedOutHandlerRecorder{h1, h2} {
		if len(h.payloads) != 2 {
			t.Fatalf("expected 2 payloads, got %d", len(h.payloads))
		}
		if h.payloads[0].CharGUID != 11 || h.payloads[1].CharGUID != 22 || h.payloads[0].RealmID != 1 {
			t.Fatalf("unexpected payloads: %+v", h.payloads)
		}
	}
}
