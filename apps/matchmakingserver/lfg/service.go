package lfg

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	ErrInvalidEntry        = errors.New("invalid lfg entry")
	ErrPlayerAlreadyQueued = errors.New("player already queued for lfg")
	ErrPlayerNotQueued     = errors.New("player is not queued for lfg")
)

const candidateWindow = 32

type Service struct {
	mut sync.RWMutex

	instanceID string
	policy     MatchPolicy
	now        func() time.Time

	queues           map[uint32][]*Entry
	entries          map[string]*Entry
	players          map[PlayerKey]*Entry
	proposals        map[uint64]*Proposal
	proposalByPlayer map[PlayerKey]*Proposal
	nextProposalID   uint64
}

func NewService(instanceID string, policy MatchPolicy) *Service {
	if policy == nil {
		policy = BlizzlikePolicy{}
	}
	return &Service{
		instanceID:       instanceID,
		policy:           policy,
		now:              time.Now,
		queues:           make(map[uint32][]*Entry),
		entries:          make(map[string]*Entry),
		players:          make(map[PlayerKey]*Entry),
		proposals:        make(map[uint64]*Proposal),
		proposalByPlayer: make(map[PlayerKey]*Proposal),
	}
}

func (s *Service) InstanceID() string { return s.instanceID }

func (s *Service) Join(entry *Entry) (*Proposal, error) {
	if err := validateEntry(entry); err != nil {
		return nil, err
	}

	s.mut.Lock()
	defer s.mut.Unlock()

	if existing := s.entries[entry.RequestID]; existing != nil {
		if proposal := s.proposalForEntry(existing); proposal != nil {
			return cloneProposal(proposal), nil
		}
		return nil, nil
	}
	for _, member := range entry.Members {
		if s.players[member.PlayerKey] != nil || s.proposalByPlayer[member.PlayerKey] != nil {
			return nil, ErrPlayerAlreadyQueued
		}
	}

	entry = cloneEntry(entry)
	if entry.QueuedAt.IsZero() || entry.QueuedAt.After(s.now()) {
		entry.QueuedAt = s.now()
	}
	s.entries[entry.RequestID] = entry
	for _, member := range entry.Members {
		s.players[member.PlayerKey] = entry
	}
	s.queues[entry.BattlegroupID] = append(s.queues[entry.BattlegroupID], entry)
	s.sortQueue(entry.BattlegroupID)

	return s.match(entry.BattlegroupID), nil
}

func (s *Service) Leave(player PlayerKey) error {
	s.mut.Lock()
	defer s.mut.Unlock()

	entry := s.players[player]
	if entry == nil {
		return ErrPlayerNotQueued
	}
	s.removeEntry(entry)
	return nil
}

func (s *Service) Status(player PlayerKey) PlayerStatus {
	s.mut.RLock()
	defer s.mut.RUnlock()

	if proposal := s.proposalByPlayer[player]; proposal != nil {
		status := PlayerStatus{Status: StatusProposed, ProposalID: proposal.ID, DungeonID: proposal.DungeonID}
		for _, assignment := range proposal.Assignments {
			if assignment.Player == player {
				status.AssignedRole = assignment.Role
				break
			}
		}
		for _, entry := range proposal.Entries {
			for _, member := range entry.Members {
				if member.PlayerKey == player {
					status.QueuedAt = entry.QueuedAt
				}
			}
		}
		return status
	}
	if entry := s.players[player]; entry != nil {
		return PlayerStatus{Status: StatusQueued, QueuedAt: entry.QueuedAt}
	}
	return PlayerStatus{}
}

func (s *Service) match(battlegroupID uint32) *Proposal {
	queue := s.queues[battlegroupID]
	if len(queue) == 0 {
		return nil
	}
	window := len(queue)
	if window > candidateWindow {
		window = candidateWindow
	}
	candidates := queue[:window]

	var found *Proposal
	var search func(candidates []*Entry, index, members int, selected []*Entry)
	search = func(candidates []*Entry, index, members int, selected []*Entry) {
		if found != nil || members > 5 {
			return
		}
		if members == 5 {
			proposal, err := s.policy.Match(selected)
			if err == nil {
				found = proposal
			}
			return
		}
		for i := index; i < len(candidates); i++ {
			search(candidates, i+1, members+len(candidates[i].Members), append(selected, candidates[i]))
		}
	}
	// Try anchors from oldest to newest. This gives the oldest compatible entry
	// priority without allowing an currently-unmatchable entry to block the
	// entire battlegroup queue.
	for anchor := 0; anchor < len(candidates) && found == nil; anchor++ {
		search(candidates, anchor+1, len(candidates[anchor].Members), []*Entry{candidates[anchor]})
	}
	if found == nil {
		return nil
	}

	s.nextProposalID++
	found.ID = s.nextProposalID
	found.CreatedAt = s.now()
	s.proposals[found.ID] = found
	for _, entry := range found.Entries {
		s.removeEntry(entry)
		// Keep the request index while the proposal is active so a gateway retry
		// with the same request ID is idempotent and returns that proposal.
		s.entries[entry.RequestID] = entry
		for _, member := range entry.Members {
			s.proposalByPlayer[member.PlayerKey] = found
		}
	}
	return cloneProposal(found)
}

func (s *Service) removeEntry(entry *Entry) {
	delete(s.entries, entry.RequestID)
	for _, member := range entry.Members {
		delete(s.players, member.PlayerKey)
	}
	queue := s.queues[entry.BattlegroupID]
	for i, queued := range queue {
		if queued == entry {
			s.queues[entry.BattlegroupID] = append(queue[:i], queue[i+1:]...)
			break
		}
	}
}

func (s *Service) proposalForEntry(entry *Entry) *Proposal {
	for _, member := range entry.Members {
		if proposal := s.proposalByPlayer[member.PlayerKey]; proposal != nil {
			return proposal
		}
	}
	return nil
}

func (s *Service) sortQueue(battlegroupID uint32) {
	sort.SliceStable(s.queues[battlegroupID], func(i, j int) bool {
		return s.queues[battlegroupID][i].QueuedAt.Before(s.queues[battlegroupID][j].QueuedAt)
	})
}

func validateEntry(entry *Entry) error {
	if entry == nil || entry.RequestID == "" || entry.Leader.GUID == 0 || len(entry.Members) == 0 || len(entry.Members) > 5 || len(entry.SelectedDungeons) == 0 {
		return ErrInvalidEntry
	}
	foundLeader := false
	seen := make(map[PlayerKey]struct{}, len(entry.Members))
	for _, member := range entry.Members {
		if member.RealmID == 0 || member.GUID == 0 || member.Roles == 0 || len(member.EligibleDungeons) == 0 {
			return ErrInvalidEntry
		}
		if _, found := seen[member.PlayerKey]; found {
			return ErrInvalidEntry
		}
		seen[member.PlayerKey] = struct{}{}
		foundLeader = foundLeader || member.PlayerKey == entry.Leader
	}
	if !foundLeader {
		return ErrInvalidEntry
	}
	return nil
}

func cloneEntry(entry *Entry) *Entry {
	copyEntry := *entry
	copyEntry.Members = append([]Member(nil), entry.Members...)
	for i := range copyEntry.Members {
		copyEntry.Members[i].EligibleDungeons = cloneSet(entry.Members[i].EligibleDungeons)
	}
	copyEntry.SelectedDungeons = cloneSet(entry.SelectedDungeons)
	return &copyEntry
}

func cloneSet(values map[uint32]struct{}) map[uint32]struct{} {
	result := make(map[uint32]struct{}, len(values))
	for value := range values {
		result[value] = struct{}{}
	}
	return result
}

func cloneProposal(proposal *Proposal) *Proposal {
	copyProposal := *proposal
	copyProposal.Entries = append([]*Entry(nil), proposal.Entries...)
	copyProposal.Assignments = append([]Assignment(nil), proposal.Assignments...)
	return &copyProposal
}
