package events_broadcaster

import (
	"sync"

	"github.com/rs/zerolog/log"
)

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
	channels   map[string]*ChatChannel
	channelsMu sync.RWMutex
}

func NewChatChannelsInMemRepo() *ChatChannelsInMemRepo {
	return &ChatChannelsInMemRepo{
		channels: make(map[string]*ChatChannel),
	}
}

func (r *ChatChannelsInMemRepo) GetOrCreate(name string) *ChatChannel {
	r.channelsMu.Lock()
	defer r.channelsMu.Unlock()

	if ch, ok := r.channels[name]; ok {
		return ch
	}

	ch := NewChatChannel(name)
	r.channels[name] = ch
	return ch
}

func (r *ChatChannelsInMemRepo) Get(name string) *ChatChannel {
	r.channelsMu.RLock()
	defer r.channelsMu.RUnlock()

	return r.channels[name]
}

func (r *ChatChannelsInMemRepo) Remove(name string) {
	r.channelsMu.Lock()
	defer r.channelsMu.Unlock()

	delete(r.channels, name)
}

type ChatChannelsService struct {
	repo          *ChatChannelsInMemRepo
	playerStreams *PlayerStreams

	// reverse index: player -> channels
	playerChannels map[uint64]map[string]struct{}
	pcMu           sync.RWMutex
}

func NewChatChannelsService() *ChatChannelsService {
	return &ChatChannelsService{
		repo:           NewChatChannelsInMemRepo(),
		playerStreams:  NewPlayerStreams(),
		playerChannels: make(map[uint64]map[string]struct{}),
	}
}

func (s *ChatChannelsService) AddPlayerToChannel(playerGUID uint64, chanName string) <-chan Event {
	channel := s.repo.GetOrCreate(chanName)
	channel.AddMember(playerGUID)

	s.pcMu.Lock()
	if _, ok := s.playerChannels[playerGUID]; !ok {
		s.playerChannels[playerGUID] = make(map[string]struct{})
	}
	s.playerChannels[playerGUID][chanName] = struct{}{}
	s.pcMu.Unlock()

	return s.playerStreams.GetOrCreate(playerGUID)
}

func (s *ChatChannelsService) RemovePlayerFromChannel(playerGUID uint64, chanName string) {
	channel := s.repo.Get(chanName)
	if channel != nil {
		channel.RemoveMember(playerGUID)
	}

	s.pcMu.Lock()
	if chans, ok := s.playerChannels[playerGUID]; ok {
		delete(chans, chanName)
		if len(chans) == 0 {
			delete(s.playerChannels, playerGUID)
		}
	}
	s.pcMu.Unlock()
}

func (s *ChatChannelsService) BroadcastToChannel(channelName string, event Event) {
	channel := s.repo.Get(channelName)
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
				Str("channel", channelName).
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
		for chanName := range chans {
			channel := s.repo.Get(chanName)
			if channel != nil {
				channel.RemoveMember(playerGUID)
			}
		}
	}

	// 3. close stream
	s.playerStreams.Remove(playerGUID)
}
