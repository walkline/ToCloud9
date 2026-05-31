package session

import (
	"context"
	"fmt"
	"strings"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	"github.com/walkline/ToCloud9/shared/authidentity"
	"github.com/walkline/ToCloud9/shared/events"
)

// FriendsResult enum values matching AzerothCore protocol
const (
	FriendResultDBError        = 0x00
	FriendResultListFull       = 0x01
	FriendResultOnline         = 0x02
	FriendResultOffline        = 0x03
	FriendResultNotFound       = 0x04
	FriendResultRemoved        = 0x05
	FriendResultAddedOnline    = 0x06
	FriendResultAddedOffline   = 0x07
	FriendResultAlready        = 0x08
	FriendResultSelf           = 0x09
	FriendResultEnemy          = 0x0A
	FriendResultIgnoreSelf     = 0x0B
	FriendResultIgnoreNotFound = 0x0C
	FriendResultIgnoreAlready  = 0x0D
	FriendResultIgnoreAdded    = 0x0E
	FriendResultIgnoreRemoved  = 0x0F
	FriendResultIgnoreFull     = 0x10
)

// HandleContactList handles CMsgContactList (0x066)
func (s *GameSession) HandleContactList(ctx context.Context, p *packet.Packet) error {
	s.logger.Debug().Msg("Handling contact list request")

	flags := p.Reader().Uint32()

	resp, err := s.charServiceClient.GetFriendsList(ctx, &pbChar.GetFriendsListRequest{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
	})
	if err != nil {
		return fmt.Errorf("failed to get friends list: %w", err)
	}

	w := packet.NewWriter(packet.SMsgContactList)
	w.Uint32(flags)

	// Friends and Ignored (combined count)
	totalCount := uint32(len(resp.Friends) + len(resp.Ignored))
	w.Uint32(totalCount)

	// Friends
	for _, friend := range resp.Friends {
		w.Uint64(friend.Guid)
		w.Uint32(0x01)                // SOCIAL_FLAG_FRIEND
		w.String(friend.Note)         // note comes BEFORE status!
		w.Uint8(uint8(friend.Status)) // 0=offline, 1=online
		if friend.Status > 0 {        // if online
			w.Uint32(friend.Area)
			w.Uint32(friend.Level)
			w.Uint32(friend.ClassID)
		}
	}

	// Ignored
	for _, ignored := range resp.Ignored {
		w.Uint64(ignored.Guid)
		w.Uint32(0x02) // SOCIAL_FLAG_IGNORED
		w.String("")   // empty note for ignored players
		// No status/area/level/class for ignored players
	}

	s.gameSocket.Send(w)

	return nil
}

// HandleAddFriend handles CMsgAddFriend (0x069)
func (s *GameSession) HandleAddFriend(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	friendName := r.String()
	note := r.String()

	s.logger.Debug().Str("friendName", friendName).Msg("Handling add friend")

	if strings.Contains(friendName, "@") {
		if !authidentity.IsValidEmail(friendName) {
			s.SendFriendStatus(FriendResultNotFound, 0, "", 0, 0, 0, 0)
			return nil
		}
		friendName = authidentity.NormalizeLoginIdentity(friendName)
		friendResp, err := s.charServiceClient.AddRealIDFriendByEmail(ctx, &pbChar.AddRealIDFriendByEmailRequest{
			Api:        root.Ver,
			RealmID:    root.RealmID,
			PlayerGUID: s.character.GUID,
			AccountID:  s.character.AccountID,
			Email:      friendName,
			Note:       note,
		})
		if err != nil {
			return fmt.Errorf("failed to add real id friend: %w", err)
		}

		s.SendFriendStatus(
			friendResp.Result,
			friendResp.FriendGUID,
			note,
			uint8(friendResp.Status),
			friendResp.Area,
			friendResp.Level,
			friendResp.ClassID,
		)
		return nil
	}

	// Resolve friend name to GUID
	charResp, err := s.charServiceClient.CharacterByName(ctx, &pbChar.CharacterByNameRequest{
		Api:           root.Ver,
		RealmID:       root.RealmID,
		CharacterName: friendName,
	})
	if err != nil {
		return fmt.Errorf("failed to lookup character: %w", err)
	}

	if charResp.Character == nil {
		s.SendFriendStatus(FriendResultNotFound, 0, "", 0, 0, 0, 0)
		return nil
	}

	friendResp, err := s.charServiceClient.AddFriend(ctx, &pbChar.AddFriendRequest{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		FriendGUID: charResp.Character.CharGUID,
		FriendName: friendName,
		Note:       note,
	})
	if err != nil {
		return fmt.Errorf("failed to add friend: %w", err)
	}

	// Send friend status with note and online info
	s.SendFriendStatus(
		friendResp.Result,
		charResp.Character.CharGUID,
		note,
		uint8(friendResp.Status),
		friendResp.Area,
		friendResp.Level,
		friendResp.ClassID,
	)
	return nil
}

// HandleDelFriend handles CMsgDelFriend (0x06A)
func (s *GameSession) HandleDelFriend(ctx context.Context, p *packet.Packet) error {
	friendGUID := p.Reader().Uint64()

	s.logger.Debug().Uint64("friendGUID", friendGUID).Msg("Handling delete friend")

	_, err := s.charServiceClient.RemoveFriend(ctx, &pbChar.RemoveFriendRequest{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		FriendGUID: friendGUID,
	})
	if err != nil {
		return fmt.Errorf("failed to remove friend: %w", err)
	}

	s.SendFriendStatus(FriendResultRemoved, friendGUID, "", 0, 0, 0, 0)
	return nil
}

// HandleSetContactNotes handles CMsgSetContactNotes (0x06B)
func (s *GameSession) HandleSetContactNotes(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	friendGUID := r.Uint64()
	note := r.String()

	s.logger.Debug().Uint64("friendGUID", friendGUID).Msg("Handling set contact notes")

	_, err := s.charServiceClient.SetFriendNote(ctx, &pbChar.SetFriendNoteRequest{
		Api:        root.Ver,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		FriendGUID: friendGUID,
		Note:       note,
	})
	if err != nil {
		return fmt.Errorf("failed to set friend note: %w", err)
	}

	return nil
}

// HandleAddIgnore handles CMsgAddIgnore (0x06C)
func (s *GameSession) HandleAddIgnore(ctx context.Context, p *packet.Packet) error {
	ignoreName := p.Reader().String()

	s.logger.Debug().Str("ignoreName", ignoreName).Msg("Handling add ignore")

	charResp, err := s.charServiceClient.CharacterByName(ctx, &pbChar.CharacterByNameRequest{
		Api:           root.Ver,
		RealmID:       root.RealmID,
		CharacterName: ignoreName,
	})
	if err != nil {
		return fmt.Errorf("failed to lookup character: %w", err)
	}

	if charResp.Character == nil {
		s.SendFriendStatus(FriendResultIgnoreNotFound, 0, "", 0, 0, 0, 0)
		return nil
	}

	ignoreResp, err := s.charServiceClient.AddIgnore(ctx, &pbChar.AddIgnoreRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		PlayerGUID:  s.character.GUID,
		IgnoredGUID: charResp.Character.CharGUID,
	})
	if err != nil {
		return fmt.Errorf("failed to add ignore: %w", err)
	}

	s.SendFriendStatus(ignoreResp.Result, charResp.Character.CharGUID, "", 0, 0, 0, 0)
	return nil
}

// HandleDelIgnore handles CMsgDelIgnore (0x06D)
func (s *GameSession) HandleDelIgnore(ctx context.Context, p *packet.Packet) error {
	ignoredGUID := p.Reader().Uint64()

	s.logger.Debug().Uint64("ignoredGUID", ignoredGUID).Msg("Handling delete ignore")

	_, err := s.charServiceClient.RemoveIgnore(ctx, &pbChar.RemoveIgnoreRequest{
		Api:         root.Ver,
		RealmID:     root.RealmID,
		PlayerGUID:  s.character.GUID,
		IgnoredGUID: ignoredGUID,
	})
	if err != nil {
		return fmt.Errorf("failed to remove ignore: %w", err)
	}

	s.SendFriendStatus(FriendResultIgnoreRemoved, ignoredGUID, "", 0, 0, 0, 0)
	return nil
}

// SendFriendStatus sends SMsgFriendStatus (0x068)
func (s *GameSession) SendFriendStatus(result uint32, guid uint64, note string, status uint8, area, level, classID uint32) {
	w := packet.NewWriter(packet.SMsgFriendStatus)
	w.Uint8(uint8(result))
	w.Uint64(guid)

	// For FRIEND_ADDED_OFFLINE and FRIEND_ADDED_ONLINE, send note
	if result == FriendResultAddedOffline || result == FriendResultAddedOnline {
		w.String(note)
	}

	// For FRIEND_ADDED_ONLINE and FRIEND_ONLINE, send status/area/level/class
	if result == FriendResultAddedOnline || result == FriendResultOnline {
		w.Uint8(status) // 1 for online, 0 for offline
		w.Uint32(area)
		w.Uint32(level)
		w.Uint32(classID)
	}

	s.gameSocket.Send(w)
}

// Friend event handlers

func (s *GameSession) HandleEventFriendStatusChange(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.FriendEventStatusChangePayload)

	// Send friend status packet to notify about online/offline status
	w := packet.NewWriter(packet.SMsgFriendStatus)
	if eventData.Status == 1 {
		w.Uint8(FriendResultOnline)
	} else {
		w.Uint8(FriendResultOffline)
	}
	w.Uint64(eventData.PlayerGUID)

	// For FRIEND_ONLINE, send status (uint8) + area/level/class
	if eventData.Status == 1 {
		w.Uint8(eventData.Status) // 1 for online
		w.Uint32(eventData.Area)
		w.Uint32(eventData.Level)
		w.Uint32(eventData.ClassID)
	}

	s.gameSocket.Send(w)

	return nil
}

func (s *GameSession) HandleEventFriendAdded(ctx context.Context, e *eBroadcaster.Event) error {
	// Friend added event - not typically sent to client as immediate notification
	// The client will see the friend in their list on next request
	return nil
}

func (s *GameSession) HandleEventFriendRemoved(ctx context.Context, e *eBroadcaster.Event) error {
	// Friend removed event - not typically sent to client as immediate notification
	return nil
}

func (s *GameSession) HandleEventFriendNoteUpdate(ctx context.Context, e *eBroadcaster.Event) error {
	// Note update - not typically sent to client as immediate notification
	return nil
}

func (s *GameSession) HandleWho(ctx context.Context, p *packet.Packet) error {
	s.logger.Debug().Msg("Handling who")

	r := p.Reader()
	lvlMin := r.Uint32()
	lvlMax := r.Uint32()

	playerName := r.String()
	guildName := r.String()

	raceMask := r.Uint32()
	classMask := r.Uint32()

	zonesCount := r.Uint32()
	if zonesCount > 10 {
		return fmt.Errorf("zoneCount is invalid - %d, should be <= 10", zonesCount)
	}

	zones := make([]uint32, zonesCount)
	for i := uint32(0); i < zonesCount; i++ {
		zones[i] = r.Uint32()
	}

	strCount := r.Uint32()
	if strCount > 4 {
		return fmt.Errorf("strCount is invalid - %d, should be <= 4", strCount)
	}

	strs := make([]string, strCount)
	for i := uint32(0); i < strCount; i++ {
		strs[i] = r.String()
	}

	resp, err := s.charServiceClient.WhoQuery(ctx, &pbChar.WhoQueryRequest{
		Api:           root.Ver,
		CharacterGUID: s.character.GUID,
		RealmID:       root.RealmID,
		LvlMin:        lvlMin,
		LvlMax:        lvlMax,
		PlayerName:    playerName,
		GuildName:     guildName,
		RaceMask:      raceMask,
		ClassMask:     classMask,
		Zones:         zones,
		Strings:       strs,
	})
	if err != nil {
		return err
	}

	w := packet.NewWriter(packet.SMsgWho)
	w.Uint32(uint32(len(resp.ItemsToDisplay)))
	w.Uint32(resp.TotalFound)
	for _, item := range resp.ItemsToDisplay {
		w.String(item.Name)
		w.String(item.Guild)
		w.Uint32(item.Lvl)
		w.Uint32(item.Class)
		w.Uint32(item.Race)
		w.Uint8(uint8(item.Race))
		w.Uint32(item.ZoneID)
	}

	s.gameSocket.Send(w)

	return nil
}
