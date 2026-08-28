package service

import (
	"testing"

	"github.com/walkline/ToCloud9/shared/events"
)

func TestAccountSessionOwnerRoundTrip(t *testing.T) {
	want := events.AccountSessionRequest{RealmID: 7, GatewayID: "gateway-1", GatewayInstanceID: "0123456789abcdef0123456789abcdef", Token: "token-1"}
	got, err := parseAccountSessionOwner(formatAccountSessionOwner(want))
	if err != nil {
		t.Fatal(err)
	}
	if got.RealmID != want.RealmID || got.GatewayID != want.GatewayID || got.GatewayInstanceID != want.GatewayInstanceID || got.Token != want.Token {
		t.Fatalf("parsed owner = %+v, want %+v", got, want)
	}
}

func TestAccountSessionKeysShareRedisClusterSlot(t *testing.T) {
	owner := accountSessionKey(42)
	pending := owner + ":pending"
	if owner != "{account-session:42}:owner" || pending != "{account-session:42}:owner:pending" {
		t.Fatalf("unexpected account session keys: %q, %q", owner, pending)
	}
}

func TestParseAccountSessionOwnerRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{
		"", "realm|gateway|instance", "x|gateway|0123456789abcdef0123456789abcdef|token",
		"1||0123456789abcdef0123456789abcdef|token", "1|gateway||token",
		"1|gateway|instance|token", "1|gateway|0123456789ABCDEF0123456789ABCDEF|token",
		"1|gateway|0123456789abcdef0123456789abcdef|",
	} {
		if _, err := parseAccountSessionOwner(value); err == nil {
			t.Fatalf("invalid owner %q was accepted", value)
		}
	}
}

func TestValidGatewayInstanceID(t *testing.T) {
	if !validGatewayInstanceID("0123456789abcdef0123456789abcdef") {
		t.Fatal("generated instance ID format was rejected")
	}
	for _, value := range []string{"", ">", "0123456789abcdef0123456789abcde", "0123456789abcdef0123456789abcdeg", "0123456789ABCDEF0123456789ABCDEF"} {
		if validGatewayInstanceID(value) {
			t.Fatalf("invalid gateway instance ID %q was accepted", value)
		}
	}
}

func TestParseRedisRunID(t *testing.T) {
	runID, err := parseRedisRunID("# Server\r\nredis_version:7.4.0\r\nrun_id:abc123\r\n")
	if err != nil || runID != "abc123" {
		t.Fatalf("run id = %q, err = %v", runID, err)
	}
	if _, err = parseRedisRunID("# Server\r\nredis_version:7.4.0\r\n"); err == nil {
		t.Fatal("missing run_id was accepted")
	}
}
