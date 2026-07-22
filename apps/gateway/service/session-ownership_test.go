package service

import (
	"strings"
	"testing"
	"time"
)

func TestParseSessionOwner(t *testing.T) {
	gatewayID, token, ok := parseSessionOwner("gateway-1|token-1")
	if !ok || gatewayID != "gateway-1" || token != "token-1" {
		t.Fatalf("unexpected parsed owner: gateway=%q token=%q ok=%v", gatewayID, token, ok)
	}
	for _, invalid := range []string{"", "gateway", "|token", "gateway|"} {
		if _, _, ok := parseSessionOwner(invalid); ok {
			t.Fatalf("expected %q to be rejected", invalid)
		}
	}
}

func TestSessionOwnershipKeysAreScopedByRealmAndIdentity(t *testing.T) {
	service := NewSessionOwnershipService(nil, nil, nil, "gateway", 7, 15*time.Second)
	if got, want := service.characterKey(34), "{gateway-session:7}:owner:character:34"; got != want {
		t.Fatalf("character key = %q, want %q", got, want)
	}
	for _, key := range []string{
		service.characterKey(34), service.evictionStreamKey("other"),
		service.gatewayLivenessKey("other"), service.evictionAckKey("token", 0),
	} {
		if !strings.HasPrefix(key, "{gateway-session:7}:") {
			t.Fatalf("key %q does not share the realm Redis Cluster hash tag", key)
		}
	}
}

func TestScriptInt64(t *testing.T) {
	for _, test := range []struct {
		value any
		want  int64
	}{{int64(1), 1}, {"0", 0}} {
		got, err := scriptInt64([]any{test.value}, 0)
		if err != nil || got != test.want {
			t.Fatalf("scriptInt64(%v) = %d, %v; want %d", test.value, got, err, test.want)
		}
	}
	if _, err := scriptInt64(nil, 0); err == nil {
		t.Fatal("scriptInt64 accepted an empty response")
	}
}
