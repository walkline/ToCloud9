package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/repo"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow/guid"
	"google.golang.org/grpc"
)

const (
	LFGRoleNone   uint8 = 0x00
	LFGRoleLeader uint8 = 0x01
	LFGRoleTank   uint8 = 0x02
	LFGRoleHealer uint8 = 0x04
	LFGRoleDamage uint8 = 0x08

	LFGTanksNeeded   uint8 = 1
	LFGHealersNeeded uint8 = 1
	LFGDamageNeeded  uint8 = 3
	LFGMaxGroupSize  uint8 = 5

	LFGRoleCheckTimeout = 45 * time.Second
	LFGProposalTimeout  = 40 * time.Second

	lfgDungeonIDMask uint32 = 0x00FFFFFF
)

var (
	ErrLFGAlreadyQueuedOrMatched = errors.New("lfg player already queued or matched")
	ErrLFGInvalidDungeon         = errors.New("lfg invalid dungeon")
	ErrLFGInvalidMember          = errors.New("lfg invalid member")
	ErrLFGInvalidRoles           = errors.New("lfg invalid roles")
	ErrLFGNotFound               = errors.New("lfg not found")
	ErrLFGGroupFull              = errors.New("lfg group full")
	ErrLFGNotLeader              = errors.New("lfg player is not group leader")
	ErrLFGProposalMismatch       = errors.New("lfg proposal mismatch")
	ErrLFGMultiRealm             = errors.New("lfg multi realm party is not allowed")
)

type LFGState uint8

const (
	LFGStateNone LFGState = iota
	LFGStateRolecheck
	LFGStateQueued
	LFGStateProposal
	LFGStateBoot
	LFGStateDungeon
	LFGStateFinishedDungeon
	LFGStateRaidBrowser
)

type LFGProposalState uint8

const (
	LFGProposalInitiating LFGProposalState = iota
	LFGProposalFailed
	LFGProposalSuccess
)

type LFGProposalFailure uint8

const (
	LFGProposalFailureNone LFGProposalFailure = iota
	LFGProposalFailureFailed
	LFGProposalFailureDeclined
)

type LFGPlayerKey struct {
	RealmID    uint32
	PlayerGUID uint64
}

type LFGMember struct {
	RealmID            uint32
	PlayerGUID         uint64
	Roles              uint8
	Leader             bool
	WorldserverID      string
	QueueLeaderRealmID uint32
	QueueLeaderGUID    uint64
}

type LFGProposalMember struct {
	RealmID            uint32
	PlayerGUID         uint64
	SelectedRoles      uint8
	AssignedRole       uint8
	QueueLeaderRealmID uint32
	QueueLeaderGUID    uint64
	WorldserverID      string
	Answered           bool
	Accepted           bool
}

type LFGStatus struct {
	State                  LFGState
	ProposalID             uint32
	ProposalState          LFGProposalState
	ProposalFailure        LFGProposalFailure
	DungeonEntry           uint32
	SelectedDungeons       []uint32
	QueuedMembers          []LFGMember
	ProposalMembers        []LFGProposalMember
	QueuedTimeMilliseconds uint32
	TanksNeeded            uint8
	HealersNeeded          uint8
	DamageNeeded           uint8
}

type LFGJoinData struct {
	RealmID        uint32
	LeaderGUID     uint64
	Members        []LFGMember
	DungeonEntries []uint32
	Comment        string
}

//go:generate mockery --name=LFGService
type LFGService interface {
	JoinLfg(ctx context.Context, data LFGJoinData) (*LFGStatus, error)
	LeaveLfg(ctx context.Context, realmID uint32, playerGUID uint64) error
	RemoveOfflinePlayer(ctx context.Context, realmID uint32, playerGUID uint64) error
	SetLfgRoles(ctx context.Context, realmID uint32, playerGUID uint64, roles uint8) (*LFGStatus, error)
	AnswerLfgProposal(ctx context.Context, realmID uint32, playerGUID uint64, proposalID uint32, accept bool) (*LFGStatus, error)
	FailLfgProposal(ctx context.Context, realmID uint32, proposalID uint32) error
	LfgStatus(ctx context.Context, realmID uint32, playerGUID uint64) (*LFGStatus, error)
	CompleteLfgDungeon(ctx context.Context, completedDungeonEntry uint32, selectedDungeonEntry uint32, players []LFGPlayerKey) error
	ProcessExpiredLfgProposals(ctx context.Context)
}

type lfgCrossRealmNodes interface {
	IsCrossRealmNodeAvailable() bool
}

type lfgAcceptedGroupRegistrar interface {
	RegisterAcceptedLfgGroup(ctx context.Context, in *pbGroup.RegisterAcceptedLfgGroupRequest, opts ...grpc.CallOption) (*pbGroup.RegisterAcceptedLfgGroupResponse, error)
}

type lfgQueuedGroup struct {
	realmID                uint32
	battlegroupID          uint32
	leader                 LFGPlayerKey
	members                []LFGMember
	memberRoles            map[LFGPlayerKey]uint8
	dungeonEntries         []uint32
	selectedDungeonEntries []uint32
	comment                string
	queuedAt               time.Time
}

type lfgRoleCheck struct {
	realmID                uint32
	battlegroupID          uint32
	leader                 LFGPlayerKey
	members                []LFGMember
	memberRoles            map[LFGPlayerKey]uint8
	dungeonEntries         []uint32
	selectedDungeonEntries []uint32
	comment                string
	startedAt              time.Time
}

type lfgProposal struct {
	id                     uint32
	realmID                uint32
	leaderRealmID          uint32
	battlegroupID          uint32
	crossRealm             bool
	dungeonID              uint32
	selectedDungeonEntries []uint32
	leader                 LFGPlayerKey
	worldserverID          string
	state                  LFGProposalState
	members                []LFGProposalMember
	memberIndex            map[LFGPlayerKey]int
	queuedGroups           []*lfgQueuedGroup
	createdAt              time.Time
}

type lfgService struct {
	mut              sync.RWMutex
	now              func() time.Time
	eventsProducer   events.MatchmakingServiceProducer
	battleGroupsRepo repo.BattleGroupsRepository
	crossRealmNodes  lfgCrossRealmNodes
	groupRegistrar   lfgAcceptedGroupRegistrar
	nextProposalID   uint32
	queuedGroups     map[LFGPlayerKey]*lfgQueuedGroup
	playerQueueOwner map[LFGPlayerKey]LFGPlayerKey
	roleChecks       map[LFGPlayerKey]*lfgRoleCheck
	playerRoleCheck  map[LFGPlayerKey]LFGPlayerKey
	proposals        map[uint32]*lfgProposal
	playerProposal   map[LFGPlayerKey]uint32
	playerDungeon    map[LFGPlayerKey]*LFGStatus
}

func NewLFGService(producer ...events.MatchmakingServiceProducer) LFGService {
	return NewLFGServiceWithClock(time.Now, producer...)
}

func NewLFGServiceWithClock(now func() time.Time, producer ...events.MatchmakingServiceProducer) LFGService {
	return NewLFGServiceWithClockAndBattleGroupsAndGroupRegistrar(now, nil, nil, nil, producer...)
}

func NewLFGServiceWithBattleGroups(battleGroupsRepo repo.BattleGroupsRepository, crossRealmNodes lfgCrossRealmNodes, producer ...events.MatchmakingServiceProducer) LFGService {
	return NewLFGServiceWithClockAndBattleGroupsAndGroupRegistrar(time.Now, battleGroupsRepo, crossRealmNodes, nil, producer...)
}

func NewLFGServiceWithClockAndBattleGroups(now func() time.Time, battleGroupsRepo repo.BattleGroupsRepository, crossRealmNodes lfgCrossRealmNodes, producer ...events.MatchmakingServiceProducer) LFGService {
	return NewLFGServiceWithClockAndBattleGroupsAndGroupRegistrar(now, battleGroupsRepo, crossRealmNodes, nil, producer...)
}

func NewLFGServiceWithBattleGroupsAndGroupRegistrar(battleGroupsRepo repo.BattleGroupsRepository, crossRealmNodes lfgCrossRealmNodes, groupRegistrar lfgAcceptedGroupRegistrar, producer ...events.MatchmakingServiceProducer) LFGService {
	return NewLFGServiceWithClockAndBattleGroupsAndGroupRegistrar(time.Now, battleGroupsRepo, crossRealmNodes, groupRegistrar, producer...)
}

func NewLFGServiceWithClockAndBattleGroupsAndGroupRegistrar(now func() time.Time, battleGroupsRepo repo.BattleGroupsRepository, crossRealmNodes lfgCrossRealmNodes, groupRegistrar lfgAcceptedGroupRegistrar, producer ...events.MatchmakingServiceProducer) LFGService {
	var eventsProducer events.MatchmakingServiceProducer
	if len(producer) > 0 {
		eventsProducer = producer[0]
	}

	return &lfgService{
		now:              now,
		eventsProducer:   eventsProducer,
		battleGroupsRepo: battleGroupsRepo,
		crossRealmNodes:  crossRealmNodes,
		groupRegistrar:   groupRegistrar,
		nextProposalID:   1,
		queuedGroups:     map[LFGPlayerKey]*lfgQueuedGroup{},
		playerQueueOwner: map[LFGPlayerKey]LFGPlayerKey{},
		roleChecks:       map[LFGPlayerKey]*lfgRoleCheck{},
		playerRoleCheck:  map[LFGPlayerKey]LFGPlayerKey{},
		proposals:        map[uint32]*lfgProposal{},
		playerProposal:   map[LFGPlayerKey]uint32{},
		playerDungeon:    map[LFGPlayerKey]*LFGStatus{},
	}
}

func (s *lfgService) JoinLfg(ctx context.Context, data LFGJoinData) (*LFGStatus, error) {
	group, err := s.buildQueuedGroup(ctx, data)
	if err != nil {
		return nil, err
	}

	s.mut.Lock()

	if existingGroup, ok := s.currentQueuedGroupForJoinLocked(group); ok {
		status := s.statusForQueuedGroupLocked(existingGroup)
		notifyPlayers := s.queuedStatusPlayerGUIDsLocked(existingGroup)
		s.mut.Unlock()
		s.publishStatus(notifyPlayers, status)
		return status, nil
	}

	for member := range group.memberRoles {
		if _, ok := s.playerQueueOwner[member]; ok {
			s.mut.Unlock()
			return nil, ErrLFGAlreadyQueuedOrMatched
		}
		if _, ok := s.playerRoleCheck[member]; ok {
			s.mut.Unlock()
			return nil, ErrLFGAlreadyQueuedOrMatched
		}
		if _, ok := s.playerProposal[member]; ok {
			s.mut.Unlock()
			return nil, ErrLFGAlreadyQueuedOrMatched
		}
		if dungeonStatus, ok := s.playerDungeon[member]; ok {
			if dungeonStatus != nil && dungeonStatus.State == LFGStateFinishedDungeon {
				delete(s.playerDungeon, member)
			} else {
				s.mut.Unlock()
				return nil, ErrLFGAlreadyQueuedOrMatched
			}
		}
	}

	var status *LFGStatus
	notifyPlayers := lfgMemberKeys(group.members)
	if group.needsRoleCheck() {
		roleCheck := newLfgRoleCheckFromGroup(group)
		s.roleChecks[roleCheck.leader] = roleCheck
		for member := range roleCheck.memberRoles {
			s.playerRoleCheck[member] = roleCheck.leader
		}
		status = statusForRoleCheck(roleCheck)
		s.mut.Unlock()
		s.publishStatus(notifyPlayers, status)
		return status, nil
	}

	s.queuedGroups[group.leader] = group
	for member := range group.memberRoles {
		s.playerQueueOwner[member] = group.leader
	}

	status = s.statusForQueuedGroupLocked(group)
	notifyPlayers = s.queuedStatusPlayerGUIDsLocked(group)
	proposal := s.tryCreateProposalLocked(group)
	if proposalID, ok := s.playerProposal[group.leader]; ok {
		status = s.statusForProposalLocked(proposalID, group.leader)
	}
	if proposal != nil {
		notifyPlayers = proposalPlayerKeys(proposal)
	}
	s.mut.Unlock()
	s.publishStatus(notifyPlayers, status)
	return status, nil
}

func (s *lfgService) currentQueuedGroupForJoinLocked(group *lfgQueuedGroup) (*lfgQueuedGroup, bool) {
	if group == nil || len(group.memberRoles) == 0 {
		return nil, false
	}

	for member := range group.memberRoles {
		leader, ok := s.playerQueueOwner[member]
		if !ok || leader != group.leader {
			return nil, false
		}
	}

	existingGroup := s.queuedGroups[group.leader]
	if !lfgQueuedGroupsHaveSameMembers(existingGroup, group) {
		return nil, false
	}

	return existingGroup, true
}

func (s *lfgService) LeaveLfg(ctx context.Context, realmID uint32, playerGUID uint64) error {
	return s.removePlayerFromLfg(ctx, realmID, playerGUID, true)
}

func (s *lfgService) RemoveOfflinePlayer(ctx context.Context, realmID uint32, playerGUID uint64) error {
	return s.removePlayerFromLfg(ctx, realmID, playerGUID, false)
}

func (s *lfgService) removePlayerFromLfg(_ context.Context, realmID uint32, playerGUID uint64, requireLeader bool) error {
	key := LFGPlayerKey{RealmID: realmID, PlayerGUID: playerGUID}

	s.mut.Lock()

	if leader, ok := s.playerQueueOwner[key]; ok {
		if requireLeader && leader != key {
			s.mut.Unlock()
			return ErrLFGNotLeader
		}
		group := s.queuedGroups[leader]
		notifyPlayers := []LFGPlayerKey{key}
		var remainingPlayers []LFGPlayerKey
		var remainingStatus *LFGStatus
		if group != nil {
			notifyPlayers = lfgMemberKeys(group.members)
		}
		s.removeQueuedGroupLocked(leader)
		if group != nil {
			remainingGroups := s.compatibleGroupsForQueuedGroupLocked(group)
			if len(remainingGroups) > 0 {
				remainingPlayers = lfgMemberKeysForQueuedGroups(remainingGroups)
				remainingStatus = s.statusForQueuedGroupLocked(remainingGroups[0])
			}
		}
		s.mut.Unlock()
		s.publishStatus(notifyPlayers, &LFGStatus{State: LFGStateNone})
		if len(remainingPlayers) > 0 {
			s.publishStatus(remainingPlayers, remainingStatus)
		}
		return nil
	}

	if leader, ok := s.playerRoleCheck[key]; ok {
		if requireLeader && leader != key {
			s.mut.Unlock()
			return ErrLFGNotLeader
		}
		roleCheck := s.roleChecks[leader]
		notifyPlayers := []LFGPlayerKey{key}
		if roleCheck != nil {
			notifyPlayers = lfgMemberKeys(roleCheck.members)
		}
		s.removeRoleCheckLocked(leader)
		s.mut.Unlock()
		s.publishStatus(notifyPlayers, &LFGStatus{State: LFGStateNone})
		return nil
	}

	if proposalID, ok := s.playerProposal[key]; ok {
		proposal := s.proposals[proposalID]
		if proposal == nil {
			s.mut.Unlock()
			return ErrLFGNotFound
		}
		queueLeader := proposalQueueLeaderForMember(proposal, key)
		if requireLeader && queueLeader != key {
			s.mut.Unlock()
			return ErrLFGNotLeader
		}
		if idx, ok := proposal.memberIndex[key]; ok {
			proposal.members[idx].Answered = true
			proposal.members[idx].Accepted = false
		}
		notifyPlayers := proposalPlayerKeys(proposal)
		status := statusForFailedProposal(proposal, LFGProposalFailureDeclined)
		s.failProposalLocked(proposalID, LFGProposalFailureDeclined)
		s.mut.Unlock()
		s.publishStatus(notifyPlayers, status)
		return nil
	}

	if _, ok := s.playerDungeon[key]; ok {
		delete(s.playerDungeon, key)
		s.mut.Unlock()
		s.publishStatus([]LFGPlayerKey{key}, &LFGStatus{State: LFGStateNone})
		return nil
	}

	s.mut.Unlock()
	return ErrLFGNotFound
}

func (s *lfgService) SetLfgRoles(_ context.Context, realmID uint32, playerGUID uint64, roles uint8) (*LFGStatus, error) {
	normalizedRoles := normalizeLfgRoles(roles)
	if normalizedRoles == LFGRoleNone {
		return nil, ErrLFGInvalidRoles
	}

	key := LFGPlayerKey{RealmID: realmID, PlayerGUID: playerGUID}

	s.mut.Lock()

	if leader, ok := s.playerRoleCheck[key]; ok {
		roleCheck := s.roleChecks[leader]
		if roleCheck == nil {
			s.mut.Unlock()
			return nil, ErrLFGNotFound
		}
		updateLfgMemberRoles(roleCheck.members, key, normalizedRoles)
		roleCheck.memberRoles[key] = normalizedRoles
		notifyPlayers := lfgMemberKeys(roleCheck.members)

		if roleCheck.ready() {
			group := newQueuedGroupFromRoleCheck(roleCheck, s.now())
			s.removeRoleCheckLocked(leader)
			s.queuedGroups[group.leader] = group
			for member := range group.memberRoles {
				s.playerQueueOwner[member] = group.leader
			}
			status := s.statusForQueuedGroupLocked(group)
			notifyPlayers = s.queuedStatusPlayerGUIDsLocked(group)
			proposal := s.tryCreateProposalLocked(group)
			if proposalID, ok := s.playerProposal[key]; ok {
				status = s.statusForProposalLocked(proposalID, key)
			}
			if proposal != nil {
				notifyPlayers = proposalPlayerKeys(proposal)
			}
			s.mut.Unlock()
			s.publishStatus(notifyPlayers, status)
			return status, nil
		}

		status := statusForRoleCheck(roleCheck)
		s.mut.Unlock()
		s.publishStatus(notifyPlayers, status)
		return status, nil
	}

	leader, ok := s.playerQueueOwner[key]
	if !ok {
		s.mut.Unlock()
		return nil, ErrLFGNotFound
	}

	group := s.queuedGroups[leader]
	if group == nil {
		s.mut.Unlock()
		return nil, ErrLFGNotFound
	}

	updateLfgMemberRoles(group.members, key, normalizedRoles)
	group.memberRoles[key] = normalizedRoles

	status := s.statusForQueuedGroupLocked(group)
	notifyPlayers := s.queuedStatusPlayerGUIDsLocked(group)
	proposal := s.tryCreateProposalLocked(group)
	if proposalID, ok := s.playerProposal[key]; ok {
		status = s.statusForProposalLocked(proposalID, key)
	}
	if proposal != nil {
		notifyPlayers = proposalPlayerKeys(proposal)
	}
	s.mut.Unlock()
	s.publishStatus(notifyPlayers, status)
	return status, nil
}

func (s *lfgService) AnswerLfgProposal(ctx context.Context, realmID uint32, playerGUID uint64, proposalID uint32, accept bool) (*LFGStatus, error) {
	key := LFGPlayerKey{RealmID: realmID, PlayerGUID: playerGUID}

	s.mut.Lock()

	activeProposalID, ok := s.playerProposal[key]
	if !ok {
		s.mut.Unlock()
		return nil, ErrLFGNotFound
	}
	if activeProposalID != proposalID {
		s.mut.Unlock()
		return nil, ErrLFGProposalMismatch
	}

	proposal := s.proposals[proposalID]
	if proposal == nil {
		s.mut.Unlock()
		return nil, ErrLFGNotFound
	}

	idx, ok := proposal.memberIndex[key]
	if !ok {
		s.mut.Unlock()
		return nil, ErrLFGNotFound
	}

	proposal.members[idx].Answered = true
	proposal.members[idx].Accepted = accept
	notifyPlayers := proposalPlayerKeys(proposal)

	if !accept {
		status := statusForFailedProposal(proposal, LFGProposalFailureDeclined)
		s.failProposalLocked(proposalID, LFGProposalFailureDeclined)
		s.mut.Unlock()
		s.publishStatus(notifyPlayers, status)
		return status, nil
	}

	allAccepted := true
	for _, member := range proposal.members {
		if !member.Answered || !member.Accepted {
			allAccepted = false
			break
		}
	}
	var acceptedPayload *events.MatchmakingEventLfgProposalAcceptedPayload
	var completedStatus map[LFGPlayerKey]*LFGStatus
	if allAccepted && proposal.state != LFGProposalSuccess {
		proposal.state = LFGProposalSuccess
		acceptedPayload = lfgProposalAcceptedEventPayload(proposal)
		completedStatus = playerDungeonStatusesForProposal(proposal)
	}

	status := s.statusForProposalLocked(proposalID, key)
	if acceptedPayload != nil {
		if err := s.registerAcceptedLfgGroup(ctx, acceptedPayload); err != nil {
			failedStatus := statusForFailedProposal(s.proposals[proposalID], LFGProposalFailureFailed)
			s.failProposalLocked(proposalID, LFGProposalFailureFailed)
			s.mut.Unlock()
			s.publishStatus(notifyPlayers, failedStatus)
			return failedStatus, nil
		}

		for member, dungeonStatus := range completedStatus {
			s.playerDungeon[member] = dungeonStatus
			delete(s.playerProposal, member)
		}
		delete(s.proposals, proposalID)
	}
	s.mut.Unlock()

	s.publishStatus(notifyPlayers, status)
	s.publishProposalAccepted(acceptedPayload)
	return status, nil
}

func (s *lfgService) LfgStatus(_ context.Context, realmID uint32, playerGUID uint64) (*LFGStatus, error) {
	key := LFGPlayerKey{RealmID: realmID, PlayerGUID: playerGUID}

	s.mut.RLock()
	defer s.mut.RUnlock()

	return s.statusForPlayerLocked(key), nil
}

func (s *lfgService) CompleteLfgDungeon(_ context.Context, completedDungeonEntry uint32, selectedDungeonEntry uint32, players []LFGPlayerKey) error {
	if len(players) == 0 {
		return nil
	}

	changedStatuses := map[LFGPlayerKey]*LFGStatus{}
	s.mut.Lock()
	for _, player := range players {
		if player.RealmID == 0 || player.PlayerGUID == 0 {
			continue
		}
		status, ok := s.playerDungeon[player]
		if !ok || status == nil || status.State != LFGStateDungeon {
			continue
		}
		if !lfgStatusMatchesSelectedDungeon(status, selectedDungeonEntry) {
			continue
		}

		finished := cloneLFGStatus(status)
		finished.State = LFGStateFinishedDungeon
		if completedDungeonEntry != 0 {
			finished.DungeonEntry = completedDungeonEntry
		}
		s.playerDungeon[player] = finished
		changedStatuses[player] = finished
	}
	s.mut.Unlock()

	if len(changedStatuses) > 0 {
		log.Info().
			Uint32("completedDungeonEntry", completedDungeonEntry).
			Uint32("selectedDungeonEntry", selectedDungeonEntry).
			Int("players", len(changedStatuses)).
			Msg("completed LFG dungeon")
	}
	for player, status := range changedStatuses {
		s.publishStatus([]LFGPlayerKey{player}, status)
	}
	return nil
}

func (s *lfgService) FailLfgProposal(_ context.Context, realmID uint32, proposalID uint32) error {
	s.mut.Lock()

	proposal := s.proposals[proposalID]
	if proposal == nil || proposal.realmID != realmID {
		s.mut.Unlock()
		return ErrLFGNotFound
	}

	notifyPlayers := proposalPlayerKeys(proposal)
	status := statusForFailedProposal(proposal, LFGProposalFailureFailed)
	s.failProposalLocked(proposalID, LFGProposalFailureFailed)
	s.mut.Unlock()

	s.publishStatus(notifyPlayers, status)
	return nil
}

func (s *lfgService) ProcessExpiredLfgProposals(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.expireLfgRoleChecks(now)
			s.expireLfgProposals(now)
		}
	}
}

func (s *lfgService) expireLfgRoleChecks(now time.Time) {
	type expiredEvent struct {
		players []LFGPlayerKey
	}

	expiredEvents := []expiredEvent{}
	s.mut.Lock()
	for leader, roleCheck := range s.roleChecks {
		if now.Before(roleCheck.startedAt.Add(LFGRoleCheckTimeout)) {
			continue
		}

		expiredEvents = append(expiredEvents, expiredEvent{
			players: lfgMemberKeys(roleCheck.members),
		})
		s.removeRoleCheckLocked(leader)
	}
	s.mut.Unlock()

	for _, event := range expiredEvents {
		s.publishStatus(event.players, &LFGStatus{State: LFGStateNone})
	}
}

func (s *lfgService) expireLfgProposals(now time.Time) {
	type expiredEvent struct {
		players []LFGPlayerKey
		status  *LFGStatus
	}

	expiredEvents := []expiredEvent{}
	s.mut.Lock()
	for proposalID, proposal := range s.proposals {
		if proposal.state != LFGProposalInitiating || now.Before(proposal.createdAt.Add(LFGProposalTimeout)) {
			continue
		}

		expiredEvents = append(expiredEvents, expiredEvent{
			players: proposalPlayerKeys(proposal),
			status:  statusForFailedProposal(proposal, LFGProposalFailureFailed),
		})
		s.failProposalLocked(proposalID, LFGProposalFailureFailed)
	}
	s.mut.Unlock()

	for _, event := range expiredEvents {
		s.publishStatus(event.players, event.status)
	}
}

func (s *lfgService) buildQueuedGroup(ctx context.Context, data LFGJoinData) (*lfgQueuedGroup, error) {
	if data.RealmID == 0 || data.LeaderGUID == 0 {
		return nil, ErrLFGInvalidMember
	}
	if len(data.DungeonEntries) == 0 {
		return nil, ErrLFGInvalidDungeon
	}

	dungeons, err := normalizeDungeonEntries(data.DungeonEntries)
	if err != nil {
		return nil, err
	}
	selectedDungeons, err := normalizeSelectedDungeonEntries(data.DungeonEntries)
	if err != nil {
		return nil, err
	}

	leaderKey := LFGPlayerKey{RealmID: data.RealmID, PlayerGUID: data.LeaderGUID}
	members := normalizeLfgMembers(leaderKey, data.Members)
	if len(members) == 0 {
		return nil, ErrLFGInvalidMember
	}
	if len(members) > int(LFGMaxGroupSize) {
		return nil, ErrLFGGroupFull
	}

	memberRoles := make(map[LFGPlayerKey]uint8, len(members))
	leaderPresent := false
	for i := range members {
		if members[i].RealmID == 0 {
			members[i].RealmID = data.RealmID
		}
		if members[i].PlayerGUID == 0 {
			return nil, ErrLFGInvalidMember
		}
		members[i].Roles = normalizeLfgRoles(members[i].Roles)
		memberKey := lfgMemberKey(members[i])
		if memberKey == leaderKey {
			members[i].Leader = true
			leaderPresent = true
		}

		if _, exists := memberRoles[memberKey]; exists {
			return nil, ErrLFGInvalidMember
		}
		memberRoles[memberKey] = members[i].Roles
	}
	if !leaderPresent {
		return nil, ErrLFGInvalidMember
	}
	if len(members) == 1 && members[0].Roles == LFGRoleNone {
		return nil, ErrLFGInvalidRoles
	}

	battlegroupID, err := s.battlegroupForLfgMembers(ctx, members)
	if err != nil {
		return nil, err
	}

	return &lfgQueuedGroup{
		realmID:                data.RealmID,
		battlegroupID:          battlegroupID,
		leader:                 leaderKey,
		members:                members,
		memberRoles:            memberRoles,
		dungeonEntries:         dungeons,
		selectedDungeonEntries: selectedDungeons,
		comment:                data.Comment,
		queuedAt:               s.now(),
	}, nil
}

func normalizeDungeonEntries(entries []uint32) ([]uint32, error) {
	seen := map[uint32]struct{}{}
	res := make([]uint32, 0, len(entries))
	for _, entry := range entries {
		dungeonID := entry & lfgDungeonIDMask
		if dungeonID == 0 {
			return nil, ErrLFGInvalidDungeon
		}
		if _, ok := seen[dungeonID]; ok {
			continue
		}
		seen[dungeonID] = struct{}{}
		res = append(res, dungeonID)
	}
	sort.Slice(res, func(i, j int) bool { return res[i] < res[j] })
	return res, nil
}

func normalizeSelectedDungeonEntries(entries []uint32) ([]uint32, error) {
	seen := map[uint32]struct{}{}
	res := make([]uint32, 0, len(entries))
	for _, entry := range entries {
		if entry&lfgDungeonIDMask == 0 {
			return nil, ErrLFGInvalidDungeon
		}
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		res = append(res, entry)
	}
	sort.Slice(res, func(i, j int) bool {
		leftID := res[i] & lfgDungeonIDMask
		rightID := res[j] & lfgDungeonIDMask
		if leftID != rightID {
			return leftID < rightID
		}
		return res[i] < res[j]
	})
	return res, nil
}

func normalizeLfgMembers(leader LFGPlayerKey, members []LFGMember) []LFGMember {
	res := make([]LFGMember, 0, len(members)+1)
	leaderSeen := false
	for _, member := range members {
		if member.RealmID == 0 {
			member.RealmID = leader.RealmID
		}
		if lfgMemberKey(member) == leader {
			member.Leader = true
			leaderSeen = true
		}
		res = append(res, member)
	}
	if !leaderSeen {
		res = append(res, LFGMember{
			RealmID:    leader.RealmID,
			PlayerGUID: leader.PlayerGUID,
			Leader:     true,
		})
	}
	sort.SliceStable(res, func(i, j int) bool {
		if res[i].Leader != res[j].Leader {
			return res[i].Leader
		}
		if res[i].RealmID != res[j].RealmID {
			return res[i].RealmID < res[j].RealmID
		}
		return res[i].PlayerGUID < res[j].PlayerGUID
	})
	return res
}

func normalizeLfgRoles(roles uint8) uint8 {
	return roles & (LFGRoleTank | LFGRoleHealer | LFGRoleDamage)
}

func lfgMemberKey(member LFGMember) LFGPlayerKey {
	return LFGPlayerKey{
		RealmID:    member.RealmID,
		PlayerGUID: member.PlayerGUID,
	}
}

func (s *lfgService) battlegroupForLfgMembers(ctx context.Context, members []LFGMember) (uint32, error) {
	if len(members) == 0 {
		return 0, ErrLFGInvalidMember
	}
	firstRealmID := members[0].RealmID
	multiRealm := false
	for _, member := range members {
		if member.RealmID == 0 {
			return 0, ErrLFGInvalidMember
		}
		if member.RealmID != firstRealmID {
			multiRealm = true
		}
	}

	if s.battleGroupsRepo == nil || s.crossRealmNodes == nil || !s.crossRealmNodes.IsCrossRealmNodeAvailable() {
		if multiRealm {
			return 0, ErrLFGMultiRealm
		}
		return 0, nil
	}

	var battlegroupID uint32
	for _, member := range members {
		memberBattlegroupID, err := s.battleGroupsRepo.BattleGroupIDByRealmID(ctx, member.RealmID)
		if err != nil {
			return 0, err
		}
		if memberBattlegroupID == 0 {
			if multiRealm {
				return 0, ErrLFGMultiRealm
			}
			return 0, nil
		}
		if battlegroupID == 0 {
			battlegroupID = memberBattlegroupID
			continue
		}
		if memberBattlegroupID != battlegroupID {
			return 0, ErrLFGMultiRealm
		}
	}
	return battlegroupID, nil
}

func (g *lfgQueuedGroup) needsRoleCheck() bool {
	return len(g.members) > 1
}

func (r *lfgRoleCheck) ready() bool {
	for _, roles := range r.memberRoles {
		if roles == LFGRoleNone {
			return false
		}
	}
	return true
}

func newLfgRoleCheckFromGroup(group *lfgQueuedGroup) *lfgRoleCheck {
	members := append([]LFGMember(nil), group.members...)
	memberRoles := make(map[LFGPlayerKey]uint8, len(group.memberRoles))
	for i := range members {
		key := lfgMemberKey(members[i])
		if key == group.leader {
			memberRoles[key] = members[i].Roles
			continue
		}
		members[i].Roles = LFGRoleNone
		memberRoles[key] = LFGRoleNone
	}

	return &lfgRoleCheck{
		realmID:                group.realmID,
		battlegroupID:          group.battlegroupID,
		leader:                 group.leader,
		members:                members,
		memberRoles:            memberRoles,
		dungeonEntries:         append([]uint32(nil), group.dungeonEntries...),
		selectedDungeonEntries: append([]uint32(nil), group.selectedDungeonEntries...),
		comment:                group.comment,
		startedAt:              group.queuedAt,
	}
}

func newQueuedGroupFromRoleCheck(roleCheck *lfgRoleCheck, queuedAt time.Time) *lfgQueuedGroup {
	return &lfgQueuedGroup{
		realmID:                roleCheck.realmID,
		battlegroupID:          roleCheck.battlegroupID,
		leader:                 roleCheck.leader,
		members:                append([]LFGMember(nil), roleCheck.members...),
		memberRoles:            cloneLfgMemberRoles(roleCheck.memberRoles),
		dungeonEntries:         append([]uint32(nil), roleCheck.dungeonEntries...),
		selectedDungeonEntries: append([]uint32(nil), roleCheck.selectedDungeonEntries...),
		comment:                roleCheck.comment,
		queuedAt:               queuedAt,
	}
}

func cloneLfgQueuedGroup(group *lfgQueuedGroup) *lfgQueuedGroup {
	if group == nil {
		return nil
	}

	return &lfgQueuedGroup{
		realmID:                group.realmID,
		battlegroupID:          group.battlegroupID,
		leader:                 group.leader,
		members:                append([]LFGMember(nil), group.members...),
		memberRoles:            cloneLfgMemberRoles(group.memberRoles),
		dungeonEntries:         append([]uint32(nil), group.dungeonEntries...),
		selectedDungeonEntries: append([]uint32(nil), group.selectedDungeonEntries...),
		comment:                group.comment,
		queuedAt:               group.queuedAt,
	}
}

func cloneLfgQueuedGroups(groups []*lfgQueuedGroup) []*lfgQueuedGroup {
	res := make([]*lfgQueuedGroup, 0, len(groups))
	for _, group := range groups {
		res = append(res, cloneLfgQueuedGroup(group))
	}
	return res
}

func cloneLfgMemberRoles(src map[LFGPlayerKey]uint8) map[LFGPlayerKey]uint8 {
	dst := make(map[LFGPlayerKey]uint8, len(src))
	for key, roles := range src {
		dst[key] = roles
	}
	return dst
}

func updateLfgMemberRoles(members []LFGMember, player LFGPlayerKey, roles uint8) {
	for i := range members {
		if lfgMemberKey(members[i]) == player {
			members[i].Roles = roles
			return
		}
	}
}

func (s *lfgService) tryCreateProposalLocked(newGroup *lfgQueuedGroup) *lfgProposal {
	if newGroup == nil {
		return nil
	}

	for _, dungeonID := range newGroup.dungeonEntries {
		groups := s.compatibleGroupsForDungeonLocked(newGroup, dungeonID)
		selectedGroups, selectedMembers := chooseLfgGroups(groups)
		if len(selectedGroups) == 0 {
			continue
		}

		assignments, ok := assignLfgRoles(selectedMembers)
		if !ok {
			continue
		}

		crossRealm := lfgGroupsCrossRealm(selectedGroups)
		proposalRealmID := selectedGroups[0].realmID
		if crossRealm {
			proposalRealmID = 0
		}

		proposal := &lfgProposal{
			id:                     s.nextProposalID,
			realmID:                proposalRealmID,
			leaderRealmID:          selectedGroups[0].leader.RealmID,
			battlegroupID:          selectedGroups[0].battlegroupID,
			crossRealm:             crossRealm,
			dungeonID:              dungeonID,
			selectedDungeonEntries: selectedDungeonEntriesForProposal(selectedGroups, dungeonID),
			leader:                 selectedGroups[0].leader,
			worldserverID:          selectedGroups[0].leaderWorldserverID(),
			state:                  LFGProposalInitiating,
			members:                make([]LFGProposalMember, 0, len(selectedMembers)),
			memberIndex:            map[LFGPlayerKey]int{},
			queuedGroups:           cloneLfgQueuedGroups(selectedGroups),
			createdAt:              s.now(),
		}
		s.nextProposalID++

		queueLeaderByMember := queueLeaderByMemberKey(selectedGroups)
		for _, member := range selectedMembers {
			key := lfgMemberKey(member)
			queueLeader := queueLeaderByMember[key]
			proposal.memberIndex[key] = len(proposal.members)
			proposal.members = append(proposal.members, LFGProposalMember{
				RealmID:            member.RealmID,
				PlayerGUID:         member.PlayerGUID,
				SelectedRoles:      member.Roles,
				AssignedRole:       assignments[key],
				QueueLeaderRealmID: queueLeader.RealmID,
				QueueLeaderGUID:    queueLeader.PlayerGUID,
				WorldserverID:      member.WorldserverID,
			})
			s.playerProposal[key] = proposal.id
		}

		s.proposals[proposal.id] = proposal
		for _, group := range selectedGroups {
			s.removeQueuedGroupLocked(group.leader)
		}
		return proposal
	}
	return nil
}

func (s *lfgService) compatibleGroupsForDungeonLocked(base *lfgQueuedGroup, dungeonID uint32) []*lfgQueuedGroup {
	groups := make([]*lfgQueuedGroup, 0, len(s.queuedGroups))
	for _, group := range s.queuedGroups {
		if !lfgGroupsShareQueueScope(base, group) || !containsDungeon(group.dungeonEntries, dungeonID) {
			continue
		}
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].queuedAt.Equal(groups[j].queuedAt) {
			if groups[i].leader.RealmID != groups[j].leader.RealmID {
				return groups[i].leader.RealmID < groups[j].leader.RealmID
			}
			return groups[i].leader.PlayerGUID < groups[j].leader.PlayerGUID
		}
		return groups[i].queuedAt.Before(groups[j].queuedAt)
	})
	return groups
}

func (s *lfgService) compatibleGroupsForQueuedGroupLocked(base *lfgQueuedGroup) []*lfgQueuedGroup {
	if base == nil {
		return nil
	}

	groups := make([]*lfgQueuedGroup, 0, len(s.queuedGroups))
	for _, group := range s.queuedGroups {
		if !lfgGroupsShareQueueScope(base, group) || !hasSharedDungeon(group.dungeonEntries, base.dungeonEntries) {
			continue
		}
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].queuedAt.Equal(groups[j].queuedAt) {
			if groups[i].leader.RealmID != groups[j].leader.RealmID {
				return groups[i].leader.RealmID < groups[j].leader.RealmID
			}
			return groups[i].leader.PlayerGUID < groups[j].leader.PlayerGUID
		}
		return groups[i].queuedAt.Before(groups[j].queuedAt)
	})
	return groups
}

func lfgGroupsShareQueueScope(left, right *lfgQueuedGroup) bool {
	if left == nil || right == nil {
		return false
	}
	if left.realmID == right.realmID {
		return true
	}
	if left.battlegroupID != 0 || right.battlegroupID != 0 {
		return left.battlegroupID != 0 && left.battlegroupID == right.battlegroupID
	}
	return false
}

func lfgGroupsCrossRealm(groups []*lfgQueuedGroup) bool {
	var realmID uint32
	for _, group := range groups {
		if group == nil {
			continue
		}
		for _, member := range group.members {
			if member.RealmID == 0 {
				continue
			}
			if realmID == 0 {
				realmID = member.RealmID
				continue
			}
			if member.RealmID != realmID {
				return true
			}
		}
	}
	return false
}

func containsDungeon(entries []uint32, dungeonID uint32) bool {
	for _, entry := range entries {
		if entry == dungeonID {
			return true
		}
	}
	return false
}

func hasSharedDungeon(left, right []uint32) bool {
	for _, entry := range left {
		if containsDungeon(right, entry) {
			return true
		}
	}
	return false
}

func chooseLfgGroups(groups []*lfgQueuedGroup) ([]*lfgQueuedGroup, []LFGMember) {
	selectedGroups := make([]*lfgQueuedGroup, 0, len(groups))
	selectedMembers := make([]LFGMember, 0, LFGMaxGroupSize)
	if chooseLfgGroupsRecursive(groups, 0, selectedGroups, selectedMembers, 0, &selectedGroups, &selectedMembers) {
		return selectedGroups, selectedMembers
	}
	return nil, nil
}

func chooseLfgGroupsRecursive(groups []*lfgQueuedGroup, idx int, currentGroups []*lfgQueuedGroup, currentMembers []LFGMember, currentSize int, selectedGroups *[]*lfgQueuedGroup, selectedMembers *[]LFGMember) bool {
	if currentSize == int(LFGMaxGroupSize) {
		if _, ok := assignLfgRoles(currentMembers); ok {
			*selectedGroups = append([]*lfgQueuedGroup(nil), currentGroups...)
			*selectedMembers = append([]LFGMember(nil), currentMembers...)
			return true
		}
		return false
	}
	if currentSize > int(LFGMaxGroupSize) || idx >= len(groups) {
		return false
	}

	group := groups[idx]
	if currentSize+len(group.members) <= int(LFGMaxGroupSize) {
		if chooseLfgGroupsRecursive(groups, idx+1, append(currentGroups, group), append(currentMembers, group.members...), currentSize+len(group.members), selectedGroups, selectedMembers) {
			return true
		}
	}
	return chooseLfgGroupsRecursive(groups, idx+1, currentGroups, currentMembers, currentSize, selectedGroups, selectedMembers)
}

func assignLfgRoles(members []LFGMember) (map[LFGPlayerKey]uint8, bool) {
	if len(members) != int(LFGMaxGroupSize) {
		return nil, false
	}

	assignments := map[LFGPlayerKey]uint8{}
	if assignLfgRolesRecursive(members, 0, int(LFGTanksNeeded), int(LFGHealersNeeded), int(LFGDamageNeeded), assignments) {
		return assignments, true
	}
	return nil, false
}

func assignLfgRolesRecursive(members []LFGMember, idx, tanks, healers, damage int, assignments map[LFGPlayerKey]uint8) bool {
	if idx == len(members) {
		return tanks == 0 && healers == 0 && damage == 0
	}

	member := members[idx]
	key := lfgMemberKey(member)
	for _, role := range []uint8{LFGRoleTank, LFGRoleHealer, LFGRoleDamage} {
		if member.Roles&role == 0 {
			continue
		}
		switch role {
		case LFGRoleTank:
			if tanks == 0 {
				continue
			}
			assignments[key] = role
			if assignLfgRolesRecursive(members, idx+1, tanks-1, healers, damage, assignments) {
				return true
			}
		case LFGRoleHealer:
			if healers == 0 {
				continue
			}
			assignments[key] = role
			if assignLfgRolesRecursive(members, idx+1, tanks, healers-1, damage, assignments) {
				return true
			}
		case LFGRoleDamage:
			if damage == 0 {
				continue
			}
			assignments[key] = role
			if assignLfgRolesRecursive(members, idx+1, tanks, healers, damage-1, assignments) {
				return true
			}
		}
		delete(assignments, key)
	}
	return false
}

func (s *lfgService) removeQueuedGroupLocked(leader LFGPlayerKey) {
	group := s.queuedGroups[leader]
	if group == nil {
		return
	}
	for member := range group.memberRoles {
		delete(s.playerQueueOwner, member)
	}
	delete(s.queuedGroups, leader)
}

func (s *lfgService) removeRoleCheckLocked(leader LFGPlayerKey) {
	roleCheck := s.roleChecks[leader]
	if roleCheck == nil {
		return
	}
	for member := range roleCheck.memberRoles {
		delete(s.playerRoleCheck, member)
	}
	delete(s.roleChecks, leader)
}

func (s *lfgService) failProposalLocked(proposalID uint32, failure LFGProposalFailure) {
	proposal := s.proposals[proposalID]
	if proposal == nil {
		return
	}
	proposal.state = LFGProposalFailed
	removedQueueLeaders := lfgProposalRemovedQueueLeaders(proposal, failure)
	for _, group := range proposal.queuedGroups {
		if group == nil {
			continue
		}
		if _, removed := removedQueueLeaders[group.leader]; removed {
			continue
		}
		queuedGroup := cloneLfgQueuedGroup(group)
		s.queuedGroups[queuedGroup.leader] = queuedGroup
		for member := range queuedGroup.memberRoles {
			s.playerQueueOwner[member] = queuedGroup.leader
		}
	}
	for _, member := range proposal.members {
		delete(s.playerProposal, LFGPlayerKey{RealmID: member.RealmID, PlayerGUID: member.PlayerGUID})
	}
	delete(s.proposals, proposalID)
}

func (s *lfgService) statusForPlayerLocked(key LFGPlayerKey) *LFGStatus {
	if proposalID, ok := s.playerProposal[key]; ok {
		return s.statusForProposalLocked(proposalID, key)
	}
	if status, ok := s.playerDungeon[key]; ok {
		return cloneLFGStatus(status)
	}
	if leader, ok := s.playerRoleCheck[key]; ok {
		roleCheck := s.roleChecks[leader]
		if roleCheck == nil {
			return &LFGStatus{State: LFGStateNone}
		}
		return statusForRoleCheck(roleCheck)
	}
	if leader, ok := s.playerQueueOwner[key]; ok {
		group := s.queuedGroups[leader]
		if group == nil {
			return &LFGStatus{State: LFGStateNone}
		}
		return s.statusForQueuedGroupLocked(group)
	}
	return &LFGStatus{State: LFGStateNone}
}

func statusForRoleCheck(roleCheck *lfgRoleCheck) *LFGStatus {
	return &LFGStatus{
		State:            LFGStateRolecheck,
		SelectedDungeons: append([]uint32(nil), roleCheck.selectedDungeonEntries...),
		QueuedMembers:    lfgMembersWithQueueLeader(roleCheck.members, roleCheck.leader),
		TanksNeeded:      LFGTanksNeeded,
		HealersNeeded:    LFGHealersNeeded,
		DamageNeeded:     LFGDamageNeeded,
	}
}

func (s *lfgService) statusForQueuedGroupLocked(group *lfgQueuedGroup) *LFGStatus {
	if group == nil {
		return &LFGStatus{State: LFGStateNone}
	}

	groups := s.compatibleGroupsForQueuedGroupLocked(group)
	members := lfgMembersForQueuedGroups(groups)
	queuedAt := group.queuedAt
	for _, candidate := range groups {
		if candidate.queuedAt.Before(queuedAt) {
			queuedAt = candidate.queuedAt
		}
	}

	return statusForQueuedMembers(group.selectedDungeonEntries, members, queuedAt, s.now())
}

func statusForQueuedMembers(dungeonEntries []uint32, members []LFGMember, queuedAt, now time.Time) *LFGStatus {
	status := &LFGStatus{
		State:            LFGStateQueued,
		SelectedDungeons: append([]uint32(nil), dungeonEntries...),
		QueuedMembers:    append([]LFGMember(nil), members...),
		TanksNeeded:      LFGTanksNeeded,
		HealersNeeded:    LFGHealersNeeded,
		DamageNeeded:     LFGDamageNeeded,
	}
	if now.After(queuedAt) {
		status.QueuedTimeMilliseconds = uint32(now.Sub(queuedAt).Seconds())
	}
	for _, member := range members {
		if member.Roles&LFGRoleTank != 0 && status.TanksNeeded > 0 {
			status.TanksNeeded--
		}
		if member.Roles&LFGRoleHealer != 0 && status.HealersNeeded > 0 {
			status.HealersNeeded--
		}
		if member.Roles&LFGRoleDamage != 0 && status.DamageNeeded > 0 {
			status.DamageNeeded--
		}
	}
	return status
}

func (s *lfgService) queuedStatusPlayerGUIDsLocked(group *lfgQueuedGroup) []LFGPlayerKey {
	return lfgMemberKeysForQueuedGroups(s.compatibleGroupsForQueuedGroupLocked(group))
}

func (s *lfgService) statusForProposalLocked(proposalID uint32, _ LFGPlayerKey) *LFGStatus {
	proposal := s.proposals[proposalID]
	if proposal == nil {
		return &LFGStatus{State: LFGStateNone}
	}
	state := LFGStateProposal
	if proposal.state == LFGProposalSuccess {
		state = LFGStateDungeon
	}
	status := &LFGStatus{
		State:         state,
		ProposalID:    proposal.id,
		ProposalState: proposal.state,
		DungeonEntry:  proposal.dungeonID,
		SelectedDungeons: append([]uint32(nil),
			proposal.selectedDungeonEntries...),
		QueuedMembers:   queuedMembersForProposal(proposal),
		ProposalMembers: append([]LFGProposalMember(nil), proposal.members...),
	}
	sort.Slice(status.ProposalMembers, func(i, j int) bool {
		if status.ProposalMembers[i].RealmID != status.ProposalMembers[j].RealmID {
			return status.ProposalMembers[i].RealmID < status.ProposalMembers[j].RealmID
		}
		return status.ProposalMembers[i].PlayerGUID < status.ProposalMembers[j].PlayerGUID
	})
	return status
}

func statusForFailedProposal(proposal *lfgProposal, failure LFGProposalFailure) *LFGStatus {
	if proposal == nil {
		return &LFGStatus{State: LFGStateNone}
	}

	proposalMembers := append([]LFGProposalMember(nil), proposal.members...)
	if failure == LFGProposalFailureFailed {
		for i := range proposalMembers {
			if !proposalMembers[i].Answered {
				proposalMembers[i].Answered = true
				proposalMembers[i].Accepted = false
			}
		}
	}

	status := &LFGStatus{
		State:           LFGStateProposal,
		ProposalID:      proposal.id,
		ProposalState:   LFGProposalFailed,
		ProposalFailure: failure,
		DungeonEntry:    proposal.dungeonID,
		SelectedDungeons: append([]uint32(nil),
			proposal.selectedDungeonEntries...),
		QueuedMembers:   queuedMembersForProposal(proposal),
		ProposalMembers: proposalMembers,
	}
	sort.Slice(status.ProposalMembers, func(i, j int) bool {
		if status.ProposalMembers[i].RealmID != status.ProposalMembers[j].RealmID {
			return status.ProposalMembers[i].RealmID < status.ProposalMembers[j].RealmID
		}
		return status.ProposalMembers[i].PlayerGUID < status.ProposalMembers[j].PlayerGUID
	})
	return status
}

func selectedDungeonEntriesForProposal(groups []*lfgQueuedGroup, dungeonID uint32) []uint32 {
	seen := map[uint32]struct{}{}
	res := make([]uint32, 0, len(groups))
	for _, group := range groups {
		for _, entry := range group.selectedDungeonEntries {
			if entry&lfgDungeonIDMask != dungeonID&lfgDungeonIDMask {
				continue
			}
			if _, ok := seen[entry]; ok {
				continue
			}
			seen[entry] = struct{}{}
			res = append(res, entry)
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i] < res[j] })
	if len(res) == 0 && dungeonID != 0 {
		res = append(res, dungeonID)
	}
	return res
}

func playerDungeonStatusesForProposal(proposal *lfgProposal) map[LFGPlayerKey]*LFGStatus {
	if proposal == nil {
		return nil
	}
	res := make(map[LFGPlayerKey]*LFGStatus, len(proposal.members))
	queuedMembers := queuedMembersForProposal(proposal)
	for _, member := range proposal.members {
		res[LFGPlayerKey{RealmID: member.RealmID, PlayerGUID: member.PlayerGUID}] = &LFGStatus{
			State:            LFGStateDungeon,
			ProposalState:    LFGProposalSuccess,
			DungeonEntry:     proposal.dungeonID,
			SelectedDungeons: append([]uint32(nil), proposal.selectedDungeonEntries...),
			QueuedMembers:    append([]LFGMember(nil), queuedMembers...),
		}
	}
	return res
}

func queuedMembersForProposal(proposal *lfgProposal) []LFGMember {
	if proposal == nil {
		return nil
	}

	members := make([]LFGMember, 0, len(proposal.members))
	for _, member := range proposal.members {
		members = append(members, LFGMember{
			RealmID:            member.RealmID,
			PlayerGUID:         member.PlayerGUID,
			Roles:              member.SelectedRoles,
			Leader:             member.QueueLeaderRealmID == member.RealmID && member.QueueLeaderGUID == member.PlayerGUID,
			WorldserverID:      member.WorldserverID,
			QueueLeaderRealmID: member.QueueLeaderRealmID,
			QueueLeaderGUID:    member.QueueLeaderGUID,
		})
	}
	return members
}

func proposalQueueLeaderForMember(proposal *lfgProposal, key LFGPlayerKey) LFGPlayerKey {
	if proposal == nil {
		return key
	}
	idx, ok := proposal.memberIndex[key]
	if !ok || idx < 0 || idx >= len(proposal.members) {
		return key
	}
	member := proposal.members[idx]
	if member.QueueLeaderRealmID == 0 || member.QueueLeaderGUID == 0 {
		return key
	}
	return LFGPlayerKey{RealmID: member.QueueLeaderRealmID, PlayerGUID: member.QueueLeaderGUID}
}

func lfgProposalRemovedQueueLeaders(proposal *lfgProposal, failure LFGProposalFailure) map[LFGPlayerKey]struct{} {
	res := map[LFGPlayerKey]struct{}{}
	if proposal == nil {
		return res
	}

	for _, member := range proposal.members {
		key := LFGPlayerKey{RealmID: member.RealmID, PlayerGUID: member.PlayerGUID}
		queueLeader := proposalQueueLeaderForMember(proposal, key)
		switch failure {
		case LFGProposalFailureDeclined:
			if member.Answered && !member.Accepted {
				res[queueLeader] = struct{}{}
			}
		case LFGProposalFailureFailed:
			if !member.Accepted {
				res[queueLeader] = struct{}{}
			}
		}
	}

	if failure == LFGProposalFailureFailed && len(res) == 0 {
		for _, group := range proposal.queuedGroups {
			if group != nil {
				res[group.leader] = struct{}{}
			}
		}
	}
	return res
}

func cloneLFGStatus(status *LFGStatus) *LFGStatus {
	if status == nil {
		return &LFGStatus{State: LFGStateNone}
	}
	clone := *status
	clone.SelectedDungeons = append([]uint32(nil), status.SelectedDungeons...)
	clone.QueuedMembers = append([]LFGMember(nil), status.QueuedMembers...)
	clone.ProposalMembers = append([]LFGProposalMember(nil), status.ProposalMembers...)
	return &clone
}

func lfgStatusMatchesSelectedDungeon(status *LFGStatus, dungeonEntry uint32) bool {
	if dungeonEntry == 0 {
		return true
	}
	entry := dungeonEntry & lfgDungeonIDMask
	if status.DungeonEntry&lfgDungeonIDMask == entry {
		return true
	}
	for _, selectedDungeon := range status.SelectedDungeons {
		if selectedDungeon&lfgDungeonIDMask == entry {
			return true
		}
	}
	return false
}

func lfgMemberKeys(members []LFGMember) []LFGPlayerKey {
	res := make([]LFGPlayerKey, 0, len(members))
	for _, member := range members {
		res = append(res, lfgMemberKey(member))
	}
	return res
}

func lfgMembersForQueuedGroups(groups []*lfgQueuedGroup) []LFGMember {
	var total int
	for _, group := range groups {
		total += len(group.members)
	}

	res := make([]LFGMember, 0, total)
	for _, group := range groups {
		res = append(res, lfgMembersWithQueueLeader(group.members, group.leader)...)
	}
	return res
}

func lfgMembersWithQueueLeader(members []LFGMember, leader LFGPlayerKey) []LFGMember {
	res := make([]LFGMember, 0, len(members))
	for _, member := range members {
		member.QueueLeaderRealmID = leader.RealmID
		member.QueueLeaderGUID = leader.PlayerGUID
		res = append(res, member)
	}
	return res
}

func lfgQueuedGroupsHaveSameMembers(left, right *lfgQueuedGroup) bool {
	if left == nil || right == nil || len(left.memberRoles) != len(right.memberRoles) {
		return false
	}
	for member := range right.memberRoles {
		if _, ok := left.memberRoles[member]; !ok {
			return false
		}
	}
	return true
}

func (g *lfgQueuedGroup) leaderWorldserverID() string {
	if g == nil {
		return ""
	}

	for _, member := range g.members {
		if lfgMemberKey(member) == g.leader {
			return member.WorldserverID
		}
	}
	return ""
}

func lfgMemberKeysForQueuedGroups(groups []*lfgQueuedGroup) []LFGPlayerKey {
	var total int
	for _, group := range groups {
		total += len(group.members)
	}

	res := make([]LFGPlayerKey, 0, total)
	for _, group := range groups {
		res = append(res, lfgMemberKeys(group.members)...)
	}
	return res
}

func queueLeaderByMemberKey(groups []*lfgQueuedGroup) map[LFGPlayerKey]LFGPlayerKey {
	res := map[LFGPlayerKey]LFGPlayerKey{}
	for _, group := range groups {
		if group == nil {
			continue
		}
		for _, member := range group.members {
			res[lfgMemberKey(member)] = group.leader
		}
	}
	return res
}

func proposalPlayerKeys(proposal *lfgProposal) []LFGPlayerKey {
	if proposal == nil {
		return nil
	}
	res := make([]LFGPlayerKey, 0, len(proposal.members))
	for _, member := range proposal.members {
		res = append(res, LFGPlayerKey{RealmID: member.RealmID, PlayerGUID: member.PlayerGUID})
	}
	return res
}

func (s *lfgService) publishStatus(players []LFGPlayerKey, status *LFGStatus) {
	if s.eventsProducer == nil || len(players) == 0 {
		return
	}

	playersByRealm := map[uint32][]guid.LowType{}
	for _, player := range players {
		if player.RealmID == 0 || player.PlayerGUID == 0 {
			continue
		}
		playersByRealm[player.RealmID] = append(playersByRealm[player.RealmID], guid.LowType(player.PlayerGUID))
	}
	statusPayload := lfgStatusToEventPayload(status)
	for realmID, playerGUIDs := range playersByRealm {
		payload := &events.MatchmakingEventLfgStatusChangedPayload{
			RealmID:     realmID,
			PlayersGUID: playerGUIDs,
			Status:      statusPayload,
		}
		if err := s.eventsProducer.LfgStatusChanged(payload); err != nil {
			log.Error().
				Err(err).
				Uint32("realmID", realmID).
				Int("players", len(playerGUIDs)).
				Msg("failed to publish LFG status changed event")
		}
	}
}

func (s *lfgService) publishProposalAccepted(payload *events.MatchmakingEventLfgProposalAcceptedPayload) {
	if s.eventsProducer == nil || payload == nil || len(payload.PlayersGUID) == 0 {
		return
	}

	if err := s.eventsProducer.LfgProposalAccepted(payload); err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", payload.RealmID).
			Uint32("proposalID", payload.ProposalID).
			Int("players", len(payload.PlayersGUID)).
			Msg("failed to publish LFG proposal accepted event")
	}
}

func (s *lfgService) registerAcceptedLfgGroup(ctx context.Context, payload *events.MatchmakingEventLfgProposalAcceptedPayload) error {
	if s.groupRegistrar == nil || payload == nil || len(payload.Members) == 0 {
		return nil
	}

	members := make([]*pbGroup.AcceptedLfgGroupMember, 0, len(payload.Members))
	for _, member := range payload.Members {
		members = append(members, &pbGroup.AcceptedLfgGroupMember{
			RealmID:            member.RealmID,
			PlayerGUID:         uint64(member.PlayerGUID),
			SelectedRoles:      uint32(member.SelectedRoles),
			AssignedRole:       uint32(member.AssignedRole),
			QueueLeaderRealmID: member.QueueLeaderRealmID,
			QueueLeaderGUID:    uint64(member.QueueLeaderGUID),
		})
	}

	res, err := s.groupRegistrar.RegisterAcceptedLfgGroup(ctx, &pbGroup.RegisterAcceptedLfgGroupRequest{
		RealmID:       payload.RealmID,
		ProposalID:    payload.ProposalID,
		DungeonEntry:  payload.DungeonEntry,
		LeaderRealmID: payload.LeaderRealmID,
		LeaderGUID:    uint64(payload.LeaderGUID),
		CrossRealm:    payload.CrossRealm,
		Members:       members,
	})
	if err != nil {
		log.Error().
			Err(err).
			Uint32("realmID", payload.RealmID).
			Uint32("leaderRealmID", payload.LeaderRealmID).
			Uint32("proposalID", payload.ProposalID).
			Int("members", len(payload.Members)).
			Msg("failed to register accepted LFG group")
		return err
	}
	if res == nil {
		return fmt.Errorf("accepted LFG group registration returned nil response")
	}
	if res.GetGroupID() == 0 {
		return fmt.Errorf("accepted LFG group registration returned empty group id")
	}
	payload.GroupID = res.GetGroupID()
	return nil
}

func lfgProposalAcceptedEventPayload(proposal *lfgProposal) *events.MatchmakingEventLfgProposalAcceptedPayload {
	if proposal == nil {
		return nil
	}

	payload := &events.MatchmakingEventLfgProposalAcceptedPayload{
		RealmID:             proposal.realmID,
		LeaderRealmID:       proposal.leaderRealmID,
		BattlegroupID:       proposal.battlegroupID,
		CrossRealm:          proposal.crossRealm,
		ProposalID:          proposal.id,
		DungeonEntry:        proposal.dungeonID,
		LeaderGUID:          guid.LowType(proposal.leader.PlayerGUID),
		LeaderWorldserverID: proposal.worldserverID,
		PlayersGUID:         make([]guid.LowType, 0, len(proposal.members)),
		Members:             make([]events.MatchmakingLfgProposalAcceptedMember, 0, len(proposal.members)),
	}
	for _, member := range proposal.members {
		playerGUID := guid.LowType(member.PlayerGUID)
		payload.PlayersGUID = append(payload.PlayersGUID, playerGUID)
		payload.Members = append(payload.Members, events.MatchmakingLfgProposalAcceptedMember{
			RealmID:            member.RealmID,
			PlayerGUID:         playerGUID,
			SelectedRoles:      member.SelectedRoles,
			AssignedRole:       member.AssignedRole,
			QueueLeaderRealmID: member.QueueLeaderRealmID,
			QueueLeaderGUID:    guid.LowType(member.QueueLeaderGUID),
			WorldserverID:      member.WorldserverID,
		})
	}
	return payload
}

func lfgStatusToEventPayload(status *LFGStatus) events.MatchmakingLfgStatusPayload {
	if status == nil {
		status = &LFGStatus{State: LFGStateNone}
	}

	queuedMembers := make([]events.MatchmakingLfgMember, 0, len(status.QueuedMembers))
	for _, member := range status.QueuedMembers {
		queuedMembers = append(queuedMembers, events.MatchmakingLfgMember{
			RealmID:            member.RealmID,
			PlayerGUID:         guid.LowType(member.PlayerGUID),
			Roles:              member.Roles,
			Leader:             member.Leader,
			QueueLeaderRealmID: member.QueueLeaderRealmID,
			QueueLeaderGUID:    guid.LowType(member.QueueLeaderGUID),
		})
	}

	proposalMembers := make([]events.MatchmakingLfgProposalMember, 0, len(status.ProposalMembers))
	for _, member := range status.ProposalMembers {
		proposalMembers = append(proposalMembers, events.MatchmakingLfgProposalMember{
			RealmID:       member.RealmID,
			PlayerGUID:    guid.LowType(member.PlayerGUID),
			SelectedRoles: member.SelectedRoles,
			AssignedRole:  member.AssignedRole,
			Answered:      member.Answered,
			Accepted:      member.Accepted,
		})
	}

	return events.MatchmakingLfgStatusPayload{
		State:                  events.MatchmakingLfgState(status.State),
		ProposalID:             status.ProposalID,
		ProposalState:          events.MatchmakingLfgProposalState(status.ProposalState),
		ProposalFailure:        events.MatchmakingLfgProposalFailure(status.ProposalFailure),
		DungeonEntry:           status.DungeonEntry,
		SelectedDungeons:       append([]uint32(nil), status.SelectedDungeons...),
		QueuedMembers:          queuedMembers,
		ProposalMembers:        proposalMembers,
		QueuedTimeMilliseconds: status.QueuedTimeMilliseconds,
		TanksNeeded:            status.TanksNeeded,
		HealersNeeded:          status.HealersNeeded,
		DamageNeeded:           status.DamageNeeded,
	}
}
