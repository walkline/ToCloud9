package service

import (
	"context"
	"testing"

	"github.com/walkline/ToCloud9/shared/events"
)

type loginLocksRecorder struct {
	releasedAccount uint32
	releasedChar    uint64
	releasedGateway string
	crashedGateway  string
}

func (*loginLocksRecorder) Acquire(context.Context, uint32, uint32, uint64, string) (bool, error) {
	return true, nil
}

func (r *loginLocksRecorder) Release(_ context.Context, _ uint32, accountID uint32, characterGUID uint64, gatewayID string) error {
	r.releasedAccount, r.releasedChar, r.releasedGateway = accountID, characterGUID, gatewayID
	return nil
}

func (r *loginLocksRecorder) ReleaseByGateway(_ context.Context, _ uint32, gatewayID string) error {
	r.crashedGateway = gatewayID
	return nil
}

func TestCharacterLoginLockEventsReleaseLogoutAndGateway(t *testing.T) {
	recorder := &loginLocksRecorder{}
	handler := NewCharacterLoginLockEvents(recorder)
	if err := handler.HandleCharacterLoggedOut(events.GWEventCharacterLoggedOutPayload{
		RealmID: 1, AccountID: 7, CharGUID: 9, GatewayID: "gateway-a",
	}); err != nil {
		t.Fatal(err)
	}
	if err := handler.ReleaseGateway(context.Background(), 1, "gateway-b"); err != nil {
		t.Fatal(err)
	}
	if recorder.releasedAccount != 7 || recorder.releasedChar != 9 || recorder.releasedGateway != "gateway-a" || recorder.crashedGateway != "gateway-b" {
		t.Fatalf("unexpected releases: %+v", recorder)
	}
}
