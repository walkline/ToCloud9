package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
	socketMocks "github.com/walkline/ToCloud9/apps/gateway/sockets/socketmock"
)

func testCorpseSnapshot(id uint64, revision uint32, mapID uint32, carrier uint64, expiresAt uint64, operation uint8) *packet.Packet {
	p := packet.NewWriter(packet.TC9SMsgCorpseSnapshot).
		Uint8(corpseSnapshotProtocolVersion).
		Uint8(operation).
		Uint64(id).
		Uint32(revision).
		Uint32(mapID).
		Uint32(0).
		Uint64(carrier).
		Uint64(expiresAt).
		Uint32(12345).
		ToPacket()
	p.Source = packet.SourceWorldServer
	return p
}

func TestCorpseSnapshotCacheRejectsStaleRevisionAndRemovesTombstone(t *testing.T) {
	now := uint64(time.Now().Unix())
	s := &GameSession{character: &LoggedInCharacter{GUID: 42, Map: 1}}

	require.NoError(t, s.InterceptCorpseSnapshot(context.Background(), testCorpseSnapshot(7, 2, 1, 42, now+60, corpseSnapshotUpsert)))
	require.NoError(t, s.InterceptCorpseSnapshot(context.Background(), testCorpseSnapshot(7, 1, 1, 42, now+60, corpseSnapshotUpsert)))
	assert.Equal(t, uint32(2), s.corpseSnapshots[7].revision)
	require.NoError(t, s.InterceptCorpseSnapshot(context.Background(), testCorpseSnapshot(7, 1, 1, 42, now+60, corpseSnapshotRemove)))
	assert.Contains(t, s.corpseSnapshots, uint64(7))

	require.NoError(t, s.InterceptCorpseSnapshot(context.Background(), testCorpseSnapshot(7, 3, 1, 42, now+60, corpseSnapshotRemove)))
	assert.Equal(t, corpseSnapshotRemove, s.corpseSnapshots[7].operation)
	require.NoError(t, s.InterceptCorpseSnapshot(context.Background(), testCorpseSnapshot(7, 2, 1, 42, now+60, corpseSnapshotUpsert)))
	assert.Equal(t, corpseSnapshotRemove, s.corpseSnapshots[7].operation)
}

func TestCorpseSnapshotCacheOnlyAcceptsWorldserverAndCarrier(t *testing.T) {
	now := uint64(time.Now().Unix())
	s := &GameSession{character: &LoggedInCharacter{GUID: 42, Map: 1}}

	wrongCarrier := testCorpseSnapshot(7, 1, 1, 99, now+60, corpseSnapshotUpsert)
	require.NoError(t, s.InterceptCorpseSnapshot(context.Background(), wrongCarrier))

	fromClient := testCorpseSnapshot(8, 1, 1, 42, now+60, corpseSnapshotUpsert)
	fromClient.Source = packet.SourceGameClient
	require.NoError(t, s.InterceptCorpseSnapshot(context.Background(), fromClient))
	assert.Empty(t, s.corpseSnapshots)
}

func TestCorpseSnapshotCacheAllowsExistingEntryUpdateAtCapacity(t *testing.T) {
	now := uint64(time.Now().Unix())
	s := &GameSession{character: &LoggedInCharacter{GUID: 42, Map: 1}}
	for id := uint64(1); id <= maxCorpseSnapshotsPerSession; id++ {
		require.NoError(t, s.InterceptCorpseSnapshot(context.Background(), testCorpseSnapshot(id, 1, 1, 42, now+60, corpseSnapshotUpsert)))
	}

	require.NoError(t, s.InterceptCorpseSnapshot(context.Background(), testCorpseSnapshot(1, 2, 1, 42, now+60, corpseSnapshotUpsert)))
	assert.Equal(t, uint32(2), s.corpseSnapshots[1].revision)
}

func TestCorpseTransferOpcodesReachInternalHandlers(t *testing.T) {
	for _, opcode := range []packet.Opcode{packet.TC9CMsgPrepareForRedirect, packet.TC9SMsgCorpseSnapshot, packet.TC9CMsgRestoreCorpse} {
		assert.False(t, OpcodeBlacklist[opcode], "%s must not be dropped before its internal handler runs", opcode)
		assert.Contains(t, HandleMap, opcode)
	}
}

func TestReadyForRedirectIgnoresClientPacket(t *testing.T) {
	p := packet.NewWriter(packet.TC9SMsgReadyForRedirect).ToPacket()
	p.Source = packet.SourceGameClient
	require.NoError(t, (&GameSession{}).HandleReadyForRedirectRequest(context.Background(), p))
}

func TestRestoreCorpseSnapshotsOnlyReplaysCurrentMapAndPrunesExpired(t *testing.T) {
	now := uint64(time.Now().Unix())
	worldSocket := socketMocks.NewSocket(t)
	worldSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		return w.Opcode == packet.TC9CMsgRestoreCorpse && assert.ObjectsAreEqual(testCorpseSnapshot(1, 1, 1, 42, now+60, corpseSnapshotUpsert).Data, w.Payload.Bytes())
	})).Return().Once()

	s := &GameSession{
		character:   &LoggedInCharacter{GUID: 42, Map: 1},
		worldSocket: worldSocket,
		corpseSnapshots: map[uint64]corpseSnapshot{
			1: mustDecodeTestSnapshot(t, testCorpseSnapshot(1, 1, 1, 42, now+60, corpseSnapshotUpsert)),
			2: mustDecodeTestSnapshot(t, testCorpseSnapshot(2, 1, 530, 42, now+60, corpseSnapshotUpsert)),
			3: mustDecodeTestSnapshot(t, testCorpseSnapshot(3, 1, 1, 42, now-1, corpseSnapshotUpsert)),
		},
	}

	s.restoreCorpseSnapshots()
	assert.NotContains(t, s.corpseSnapshots, uint64(3))
}

func TestTimeSyncRestoresCorpseSnapshotsOnlyOnWorldEntry(t *testing.T) {
	now := uint64(time.Now().Unix())
	gameSocket := socketMocks.NewSocket(t)
	worldSocket := socketMocks.NewSocket(t)
	timeSync := packet.NewWriter(packet.SMsgTimeSyncReq).ToPacket()
	gameSocket.On("SendPacket", timeSync).Return().Twice()
	worldSocket.On("Send", mock.MatchedBy(func(w *packet.Writer) bool {
		return w.Opcode == packet.TC9CMsgRestoreCorpse
	})).Return().Once()

	s := &GameSession{
		character:         &LoggedInCharacter{GUID: 42, Map: 1},
		gameSocket:        gameSocket,
		worldSocket:       worldSocket,
		worldEntryPending: true,
		corpseSnapshots: map[uint64]corpseSnapshot{
			1: mustDecodeTestSnapshot(t, testCorpseSnapshot(1, 1, 1, 42, now+60, corpseSnapshotUpsert)),
		},
	}

	require.NoError(t, s.InterceptSMsgTimeSyncReq(context.Background(), timeSync))
	require.NoError(t, s.InterceptSMsgTimeSyncReq(context.Background(), timeSync))
}

func TestCrashRecoveryStartsNewWorldEntry(t *testing.T) {
	oldSocket := socketMocks.NewSocket(t)
	recoveredSocket := socketMocks.NewSocket(t)
	s := &GameSession{
		character:         &LoggedInCharacter{GUID: 42, Map: 1},
		worldSocket:       oldSocket,
		worldEntryPending: false,
	}

	s.installRecoveredWorldSocket(recoveredSocket)

	assert.Same(t, recoveredSocket, s.worldSocket)
	assert.True(t, s.worldEntryPending)
}

func TestPrepareForLayerRedirectCarriesOnlyCurrentMapSnapshotIDs(t *testing.T) {
	now := uint64(time.Now().Unix())
	s := &GameSession{
		character: &LoggedInCharacter{GUID: 42, Map: 1},
		corpseSnapshots: map[uint64]corpseSnapshot{
			9: mustDecodeTestSnapshot(t, testCorpseSnapshot(9, 1, 1, 42, now+60, corpseSnapshotUpsert)),
			4: mustDecodeTestSnapshot(t, testCorpseSnapshot(4, 1, 1, 42, now+60, corpseSnapshotUpsert)),
			7: mustDecodeTestSnapshot(t, testCorpseSnapshot(7, 1, 530, 42, now+60, corpseSnapshotUpsert)),
		},
	}

	r := s.prepareForRedirectPacket(true).ToPacket().Reader()
	assert.Equal(t, corpseSnapshotProtocolVersion, r.Uint8())
	assert.Equal(t, uint16(2), r.Uint16())
	assert.Equal(t, uint64(4), r.Uint64())
	assert.Equal(t, uint64(9), r.Uint64())
	assert.Equal(t, 0, r.Left())
	assert.NoError(t, r.Error())
}

func mustDecodeTestSnapshot(t *testing.T, p *packet.Packet) corpseSnapshot {
	t.Helper()
	_, snapshot, err := decodeCorpseSnapshotEnvelope(p)
	require.NoError(t, err)
	return snapshot
}
