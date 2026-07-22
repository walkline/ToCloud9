package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	charMocks "github.com/walkline/ToCloud9/gen/characters/pb/mocks"
)

func TestAccountSessionGatewayLivenessHeartbeat(t *testing.T) {
	client := &charMocks.CharactersServiceClient{}
	client.On("HeartbeatGatewaySession", mock.Anything, mock.MatchedBy(func(request *pbChar.HeartbeatGatewaySessionRequest) bool {
		return request.GatewayID == "gateway-7" && request.RealmID == 3 && request.LivenessSeconds == 30
	}), mock.Anything).Return(&pbChar.HeartbeatGatewaySessionResponse{}, nil).Once()
	logger := zerolog.Nop()
	liveness := NewAccountSessionGatewayLiveness(client, &logger, "gateway-7", 3, 30*time.Second)
	if err := liveness.Heartbeat(context.Background()); err != nil {
		t.Fatal(err)
	}
	client.AssertExpectations(t)
}

func TestNewGatewaySessionInstanceIDIsUniqueAndBounded(t *testing.T) {
	first, err := NewGatewaySessionInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewGatewaySessionInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) == 0 || len(first) > 64 {
		t.Fatalf("invalid gateway session IDs: %q %q", first, second)
	}
}

func TestAccountSessionGatewayLivenessFailsBeforeExpiry(t *testing.T) {
	client := &charMocks.CharactersServiceClient{}
	client.On("HeartbeatGatewaySession", mock.Anything, mock.Anything, mock.Anything).
		Return((*pbChar.HeartbeatGatewaySessionResponse)(nil), errors.New("database unavailable"))
	logger := zerolog.Nop()
	liveness := NewAccountSessionGatewayLiveness(client, &logger, "gateway-7", 3, 90*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := liveness.Run(ctx); err == nil {
		t.Fatal("expected liveness failure")
	}
}
