package events_broadcaster

import (
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

type ChatChannelScope struct {
	RealmID uint32
	TeamID  uint32
	Name    string
}

func NewChatChannelScope(realmID uint32, teamID uint32, name string) ChatChannelScope {
	return ChatChannelScope{
		RealmID: realmID,
		TeamID:  teamID,
		Name:    strings.ToLower(name),
	}
}

type PlayerStreams struct {
	streams map[uint64]chan Event
	mu      sync.RWMutex
}

func NewPlayerStreams() *PlayerStreams {
	return &PlayerStreams{
		streams: make(map[uint64]chan Event),
	}
}

func (ps *PlayerStreams) GetOrCreate(playerGUID uint64) chan Event {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ch, ok := ps.streams[playerGUID]; ok {
		return ch
	}

	ch := make(chan Event, 32)
	ps.streams[playerGUID] = ch
	return ch
}

func (ps *PlayerStreams) Get(playerGUID uint64) (chan Event, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	ch, ok := ps.streams[playerGUID]
	return ch, ok
}

func (ps *PlayerStreams) Remove(playerGUID uint64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ch, ok := ps.streams[playerGUID]; ok {
		close(ch)
		delete(ps.streams, playerGUID)
	}
}

type ChatChannel struct {
	name    string
	members map[uint64]struct{}
	mu      sync.RWMutex
}

func NewChatChannel(name string) *ChatChannel {
	return &ChatChannel{
		name:    name,
		members: make(map[uint64]struct{}),
	}
}

func (c *ChatChannel) Name() string {
	return c.name
}

func (c *ChatChannel) AddMember(playerGUID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.members[playerGUID] = struct{}{}
}

func (c *ChatChannel) RemoveMember(playerGUID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.members, playerGUID)
}

func (c *ChatChannel) Members() []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]uint64, 0, len(c.members))
	for id := range c.members {
		ids = append(ids, id)
	}
	return ids
}

type ChatChannelsInMemRepo struct {
	channels   map[ChatChannelScope]*ChatChannel
	channelsMu sync.RWMutex
}

func NewChatChannelsInMemRepo() *ChatChannelsInMemRepo {
	return &ChatChannelsInMemRepo{
		channels: make(map[ChatChannelScope]*ChatChannel),
	}
}

func (r *ChatChannelsInMemRepo) GetOrCreate(scope ChatChannelScope) *ChatChannel {
	r.channelsMu.Lock()
	defer r.channelsMu.Unlock()

	if ch, ok := r.channels[scope]; ok {
		return ch
	}

	ch := NewChatChannel(scope.Name)
	r.channels[scope] = ch
	return ch
}

func (r *ChatChannelsInMemRepo) Get(scope ChatChannelScope) *ChatChannel {
	r.channelsMu.RLock()
	defer r.channelsMu.RUnlock()

	return r.channels[scope]
}

func (r *ChatChannelsInMemRepo) Remove(scope ChatChannelScope) {
	r.channelsMu.Lock()
	defer r.channelsMu.Unlock()

	delete(r.channels, scope)
}

type ChatChannelsService struct {
	repo          *ChatChannelsInMemRepo
	playerStreams *PlayerStreams

	// reverse index: player -> channels
	playerChannels map[uint64]map[ChatChannelScope]struct{}
	pcMu           sync.RWMutex
}

func NewChatChannelsService() *ChatChannelsService {
	return &ChatChannelsService{
		repo:           NewChatChannelsInMemRepo(),
		playerStreams:  NewPlayerStreams(),
		playerChannels: make(map[uint64]map[ChatChannelScope]struct{}),
	}
}

func (s *ChatChannelsService) AddPlayerToChannel(playerGUID uint64, chanName string) <-chan Event {
	return s.AddPlayerToScopedChannel(playerGUID, 0, 0, chanName)
}

func (s *ChatChannelsService) AddPlayerToScopedChannel(playerGUID uint64, realmID uint32, teamID uint32, chanName string) <-chan Event {
	scope := NewChatChannelScope(realmID, teamID, chanName)
	channel := s.repo.GetOrCreate(scope)
	channel.AddMember(playerGUID)

	s.pcMu.Lock()
	if _, ok := s.playerChannels[playerGUID]; !ok {
		s.playerChannels[playerGUID] = make(map[ChatChannelScope]struct{})
	}
	s.playerChannels[playerGUID][scope] = struct{}{}
	s.pcMu.Unlock()

	return s.playerStreams.GetOrCreate(playerGUID)
}

func (s *ChatChannelsService) RemovePlayerFromChannel(playerGUID uint64, chanName string) {
	s.RemovePlayerFromScopedChannel(playerGUID, 0, 0, chanName)
}

func (s *ChatChannelsService) RemovePlayerFromScopedChannel(playerGUID uint64, realmID uint32, teamID uint32, chanName string) {
	scope := NewChatChannelScope(realmID, teamID, chanName)
	channel := s.repo.Get(scope)
	if channel != nil {
		channel.RemoveMember(playerGUID)
	}

	s.pcMu.Lock()
	if chans, ok := s.playerChannels[playerGUID]; ok {
		delete(chans, scope)
		if len(chans) == 0 {
			delete(s.playerChannels, playerGUID)
		}
	}
	s.pcMu.Unlock()
}

func (s *ChatChannelsService) BroadcastToChannel(channelName string, event Event) {
	s.BroadcastToScopedChannel(0, 0, channelName, event)
}

func (s *ChatChannelsService) BroadcastToScopedChannel(realmID uint32, teamID uint32, channelName string, event Event) {
	scope := NewChatChannelScope(realmID, teamID, channelName)
	channel := s.repo.Get(scope)
	if channel == nil {
		return
	}

	for _, playerGUID := range channel.Members() {
		ch, ok := s.playerStreams.Get(playerGUID)
		if !ok {
			continue
		}

		select {
		case ch <- event:
		default:
			log.Warn().
				Uint64("playerGUID", playerGUID).
				Int("eventType", int(event.Type)).
				Uint32("realmID", realmID).
				Uint32("teamID", teamID).
				Str("channel", scope.Name).
				Msg("Dropped channel event because channel is full")
		}
	}
}

func (s *ChatChannelsService) DisconnectPlayer(playerGUID uint64) {
	s.pcMu.Lock()
	chans, ok := s.playerChannels[playerGUID]
	if ok {
		delete(s.playerChannels, playerGUID)
	}
	s.pcMu.Unlock()

	if ok {
		for scope := range chans {
			channel := s.repo.Get(scope)
			if channel != nil {
				channel.RemoveMember(playerGUID)
			}
		}
	}

	// 3. close stream
	s.playerStreams.Remove(playerGUID)
}
