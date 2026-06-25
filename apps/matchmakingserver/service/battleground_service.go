package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	matchmaking "github.com/walkline/ToCloud9/apps/matchmakingserver"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/battleground"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/repo"
	pbServRegistry "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/gameserver/conn"
	wowarena "github.com/walkline/ToCloud9/shared/wow/arena"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

var (
	ErrAlreadyInQueue        = errors.New("already in queue")
	ErrInvalidArenaType      = errors.New("invalid arena type")
	ErrRatedArenaUnavailable = errors.New("rated arena team data unavailable")
	ErrNotFound              = errors.New("not found")
)

type BracketID uint8

type BattlegroundCreator interface {
	// CreateBattleground creates a battleground on game server side and starts it.
	CreateBattleground(
		ctx context.Context,
		template repo.BattlegroundTemplate,
		queueType battleground.QueueTypeID,
		bracketID BracketID,
		realmID, battlegroupID uint32,
		allianceGroups, hordeGroups []QueuedGroup,
	) error
}

//go:generate mockery --name=BattleGroundService
type BattleGroundService interface {
	BattlegroundCreator

	TemplateForQueueTypeID(ctx context.Context, id battleground.QueueTypeID) repo.BattlegroundTemplate

	BattlegroundsThatNeedPlayers(
		ctx context.Context,
		battlegroundTypeID battleground.QueueTypeID,
		bracketID uint8,
		realmID, battlegroupID uint32,
	) ([]battleground.Battleground, error)

	AddGroupToQueue(
		ctx context.Context,
		realmID uint32,
		leaderGUID uint64,
		partyMembers []uint64,
		typeID battleground.QueueTypeID,
		leaderLvl uint8,
		teamID battleground.PVPTeam,
		arenaType uint8,
		isRated bool,
	) error

	InviteGroups(ctx context.Context, groups []QueuedGroup, bg *battleground.Battleground, team battleground.PVPTeam) error

	GetQueueOrBattlegroundLinkForPlayer(k QueuesByRealmAndPlayerKey) []QueueOrBattlegroundLink

	GetBattlegroundByBattlegroundKey(ctx context.Context, instanceID uint32, realmKey repo.RealmWithBattlegroupKey) (*battleground.Battleground, error)

	PlayerLeftBattleground(ctx context.Context, playerGUID uint64, realmID, instanceID uint32, isCrossrealm bool) error

	PlayerJoinedBattleground(ctx context.Context, playerGUID uint64, realmID, instanceID uint32, isCrossrealm bool) error

	BattlegroundStatusChanged(ctx context.Context, status battleground.Status, realmID, instanceID uint32, isCrossrealm bool) error

	FinishRatedArenaMatch(ctx context.Context, request RatedArenaMatchResultRequest) (*RatedArenaMatchResult, error)

	RemovePlayerFromQueue(ctx context.Context, playerGUID uint64, realmID uint32, typeID battleground.QueueTypeID) error

	PlayerBecomeOffline(ctx context.Context, playerGUID uint64, realmID uint32) error

	ProcessExpiredBattlegroundInvites(ctx context.Context)

	ServersRegistryGSRemovedConsumer
}

type QueueByRealmOrBattlegroupKey struct {
	BattlegroupID uint32
	RealmID       uint32
}

type QueuesByRealmAndPlayerKey struct {
	guid.PlayerUnwrapped
}

type BattlegroundKey struct {
	RealmID       uint32
	InstanceID    uint32
	BattlegroupID uint32
}

type QueueOrBattlegroundLink struct {
	Queue           PVPQueue
	BattlegroundKey *BattlegroundKey
}

type battleGroundService struct {
	bgTemplates                     [battleground.QueueTypeIDEnd]*repo.BattlegroundTemplate
	queues                          map[QueueByRealmOrBattlegroupKey]map[battleground.QueueTypeID]map[BracketID]PVPQueue
	playersQueueOrBattleground      map[QueuesByRealmAndPlayerKey][]QueueOrBattlegroundLink
	playersQueueOrBattlegroundMutex sync.RWMutex
	arenaPreviousOpponents          map[uint32]uint32
	arenaPreviousOpponentsMutex     sync.RWMutex

	battleGroupsRepo       repo.BattleGroupsRepository
	arenaTeamRepo          repo.ArenaTeamRepository
	battlegroundsRepo      repo.BattlegroundRepo
	crossRealmNodesTracker *CrossRealmNodesTracker
	eventsProducer         events.MatchmakingServiceProducer
	serversRegistryClient  pbServRegistry.ServersRegistryServiceClient
	gameserverGRPCConnMgr  conn.GameServerGRPCConnMgr
	arenaStartMMR          uint32
	arenaRatingConfig      ArenaRatingConfig
}

type ArenaRatingConfig struct {
	WinModifier1       float64
	WinModifier2       float64
	LoseModifier       float64
	MatchmakerModifier float64
	MaxAllowedMMRDrop  uint32
}

func (s *battleGroundService) BattlegroundsThatNeedPlayers(ctx context.Context, battlegroundTypeID battleground.QueueTypeID, bracketID uint8, battleGroupID, realmID uint32) ([]battleground.Battleground, error) {
	bgs, err := s.battlegroundsRepo.GetActiveBattlegrounds(ctx, battlegroundTypeID, bracketID, repo.RealmWithBattlegroupKey{RealmID: realmID, BattlegroupID: battleGroupID})
	if err != nil {
		return nil, err
	}

	res := make([]battleground.Battleground, 0, len(bgs)/2)
	for _, bg := range bgs {
		if bg.FreeSlotsForTeam(battleground.TeamAlliance) > 0 {
			res = append(res, bg)
			continue
		}

		if bg.FreeSlotsForTeam(battleground.TeamHorde) > 0 {
			res = append(res, bg)
		}
	}

	return res, nil
}

func NewBattleGroundService(
	templatesRepo repo.BattlegroundTemplesRepo,
	arenaTeamRepo repo.ArenaTeamRepository,
	battleGroupsRepo repo.BattleGroupsRepository,
	battlegroundsRepo repo.BattlegroundRepo,
	crossRealmNodesTracker *CrossRealmNodesTracker,
	eventsProducer events.MatchmakingServiceProducer,
	serversRegistryClient pbServRegistry.ServersRegistryServiceClient,
	gameserverGRPCConnMgr conn.GameServerGRPCConnMgr,
	arenaStartMMR uint32,
	arenaRatingConfig ArenaRatingConfig,
	realmIDs []uint32,
) (BattleGroundService, error) {
	templates, err := templatesRepo.GetAll(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cannot get battleground templates: %w", err)
	}

	service := battleGroundService{
		playersQueueOrBattleground: map[QueuesByRealmAndPlayerKey][]QueueOrBattlegroundLink{},
		arenaPreviousOpponents:     map[uint32]uint32{},
		battleGroupsRepo:           battleGroupsRepo,
		arenaTeamRepo:              arenaTeamRepo,
		battlegroundsRepo:          battlegroundsRepo,
		crossRealmNodesTracker:     crossRealmNodesTracker,
		eventsProducer:             eventsProducer,
		serversRegistryClient:      serversRegistryClient,
		gameserverGRPCConnMgr:      gameserverGRPCConnMgr,
		arenaStartMMR:              arenaStartMMR,
		arenaRatingConfig:          arenaRatingConfig,
	}

	for _, template := range templates {
		service.bgTemplates[template.TypeID] = &template
	}

	battlegroups, err := battleGroupsRepo.AllBattleGroupsIDs(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cannot get all battlegroups: %w", err)
	}
	if service.arenaStartMMR == 0 {
		service.arenaStartMMR = 1500
	}
	if service.arenaRatingConfig.WinModifier1 == 0 {
		service.arenaRatingConfig.WinModifier1 = 48
	}
	if service.arenaRatingConfig.WinModifier2 == 0 {
		service.arenaRatingConfig.WinModifier2 = 24
	}
	if service.arenaRatingConfig.LoseModifier == 0 {
		service.arenaRatingConfig.LoseModifier = 24
	}
	if service.arenaRatingConfig.MatchmakerModifier == 0 {
		service.arenaRatingConfig.MatchmakerModifier = 24
	}
	if service.arenaRatingConfig.MaxAllowedMMRDrop == 0 {
		service.arenaRatingConfig.MaxAllowedMMRDrop = 500
	}

	service.queues = generateQueuesForAllBattlegroundTypes(&service, realmIDs, battlegroups)
	crossRealmNodesTracker.SetObserver(&service)

	if crossRealmNodesTracker.IsCrossRealmNodeAvailable() {
		log.Info().Msg("Crossrealm enabled")
	} else {
		log.Info().Msg("CrossRealm disabled")
	}

	return &service, nil
}

func (s *battleGroundService) TemplateForQueueTypeID(ctx context.Context, id battleground.QueueTypeID) repo.BattlegroundTemplate {
	fallbackBG := s.bgTemplates[battleground.QueueTypeIDWarsongGulch]
	if id != battleground.QueueTypeIDRandomBattleground {
		if s.bgTemplates[id] != nil {
			return *s.bgTemplates[id]
		}
		return *fallbackBG
	}

	res := selectRandomTemplate([]repo.BattlegroundTemplate{
		*s.bgTemplates[battleground.QueueTypeIDAlteracValley],
		*s.bgTemplates[battleground.QueueTypeIDArathiBasin],
		*s.bgTemplates[battleground.QueueTypeIDEyeOfTheStorm],
		*s.bgTemplates[battleground.QueueTypeIDWarsongGulch],
		*s.bgTemplates[battleground.QueueTypeIDIsleOfConquest],
		*s.bgTemplates[battleground.QueueTypeIDStrandOfTheAncients],
	})
	if res == nil {
		res = fallbackBG
	}
	return *res
}

func selectRandomTemplate(templates []repo.BattlegroundTemplate) *repo.BattlegroundTemplate {
	totalWeight := 0
	for _, t := range templates {
		if t.RandomBattlegroundWeight > 0 {
			totalWeight += t.RandomBattlegroundWeight
		}
	}

	if totalWeight == 0 {
		return nil
	}

	// Generate a random number in the range [1, totalWeight]
	rand.Seed(time.Now().UnixNano())
	randomWeight := rand.Intn(totalWeight) + 1

	currentWeight := 0
	for _, t := range templates {
		currentWeight += t.RandomBattlegroundWeight
		if randomWeight <= currentWeight {
			return &t
		}
	}

	// Fallback (should not occur if the weights are correct)
	return nil
}

func (s *battleGroundService) AddGroupToQueue(
	ctx context.Context,
	realmID uint32,
	leaderGUID uint64,
	partyMembers []uint64,
	typeID battleground.QueueTypeID,
	leaderLvl uint8,
	teamID battleground.PVPTeam,
	arenaType uint8,
	isRated bool,
) error {
	var arenaQueueData *repo.ArenaTeamQueueData
	if typeID == battleground.QueueTypeIDAllArenas {
		if arenaType != 2 && arenaType != 3 && arenaType != 5 {
			return ErrInvalidArenaType
		}
		if isRated {
			if s.arenaTeamRepo == nil {
				return ErrRatedArenaUnavailable
			}

			playerGUIDs := make([]uint64, 0, len(partyMembers)+1)
			playerGUIDs = append(playerGUIDs, uint64(guid.LowType(leaderGUID)))
			for _, memberGUID := range partyMembers {
				playerGUIDs = append(playerGUIDs, uint64(guid.LowType(memberGUID)))
			}

			var err error
			arenaQueueData, err = s.arenaTeamRepo.QueueDataForRatedArena(ctx, realmID, uint64(guid.LowType(leaderGUID)), playerGUIDs, arenaType, s.arenaStartMMR)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrRatedArenaUnavailable, err)
			}
		}
		teamID = battleground.TeamAny
	} else {
		arenaType = 0
		isRated = false
	}

	leaderUnwrappedGUID := guid.PlayerUnwrapped{
		RealmID: uint16(realmID),
		LowGUID: guid.LowType(leaderGUID),
	}

	// Probably player can be in several queues in the same time.
	if len(s.GetQueueOrBattlegroundLinkForPlayer(QueuesByRealmAndPlayerKey{leaderUnwrappedGUID})) > 0 {
		return ErrAlreadyInQueue
	}

	partyMembersGUIDs := make([]guid.PlayerUnwrapped, len(partyMembers))
	for i, playerGUID := range partyMembers {
		if len(s.GetQueueOrBattlegroundLinkForPlayer(QueuesByRealmAndPlayerKey{
			guid.PlayerUnwrapped{
				RealmID: uint16(realmID),
				LowGUID: guid.LowType(playerGUID),
			},
		})) > 0 {
			return ErrAlreadyInQueue
		}
		partyMembersGUIDs[i] = guid.PlayerUnwrapped{
			RealmID: uint16(realmID),
			LowGUID: guid.LowType(playerGUID),
		}
	}

	battleGroupID, err := s.battleGroupsRepo.BattleGroupIDByRealmID(ctx, realmID)
	if err != nil {
		return fmt.Errorf("cannot get BattleGroupID from repository: %w", err)
	}

	queueKey := QueueByRealmOrBattlegroupKey{}

	// Disable cross-realm functionality if there are no cross-realm nodes available.
	if battleGroupID != 0 && s.crossRealmNodesTracker.IsCrossRealmNodeAvailable() {
		queueKey.BattlegroupID = battleGroupID
	} else {
		queueKey.RealmID = realmID
	}

	if _, exists := s.queues[queueKey]; !exists {
		return fmt.Errorf("unknown queue for key %v", queueKey)
	}

	if _, exists := s.queues[queueKey][typeID]; !exists {
		return fmt.Errorf("unknown queue for type %v", typeID)
	}

	bracketID := BracketID(battleground.BracketIDByLevel(leaderLvl))
	if _, exists := s.queues[queueKey][typeID][bracketID]; !exists {
		return fmt.Errorf("unknown queue for bracket id %v", bracketID)
	}

	queue := s.queues[queueKey][typeID][bracketID]
	group := &QueuedGroup{
		LeaderGUID: guid.PlayerUnwrapped{
			RealmID: uint16(realmID),
			LowGUID: guid.LowType(leaderGUID),
		},
		Members:      partyMembersGUIDs,
		RealmID:      realmID,
		TeamID:       teamID,
		ArenaType:    arenaType,
		IsRated:      isRated,
		EnqueuedTime: time.Now(),
	}
	if arenaQueueData != nil {
		group.ArenaTeamID = arenaQueueData.ArenaTeamID
		group.ArenaTeamRating = arenaQueueData.TeamRating
		group.ArenaMatchmakerRating = arenaQueueData.MatchmakerRating
		group.ArenaPreviousOpponentsTeamID = s.previousRatedArenaOpponentID(realmID, arenaQueueData.ArenaTeamID)
	}

	slots := s.addQueueForGroupMembers(queue, group)

	err = queue.AddQueuedGroup(group)
	if err != nil {
		s.removeQueueForGroupMembers(queue, group)
		return fmt.Errorf("cannot add queue for bracket id %v and type id: %v: %w", bracketID, typeID, err)
	}

	if typeID == battleground.QueueTypeIDAllArenas {
		log.Debug().
			Uint32("realmID", realmID).
			Uint64("leaderGUID", uint64(guid.LowType(leaderGUID))).
			Interface("partyMembers", partyMembersGUIDs).
			Uint8("arenaType", arenaType).
			Bool("isRated", isRated).
			Uint32("arenaTeamID", group.ArenaTeamID).
			Uint32("arenaTeamRating", group.ArenaTeamRating).
			Uint32("arenaMatchmakerRating", group.ArenaMatchmakerRating).
			Interface("queueKey", queueKey).
			Uint8("bracketID", uint8(bracketID)).
			Interface("queueSlots", slots).
			Msg("TC9 arena group queued")
	}

	minLvl, maxLvl := battleground.LevelsDiapasonForBracket(uint8(bracketID))

	playersLowGuids := make([]guid.LowType, len(partyMembers)+1)
	for i, playerGUID := range partyMembers {
		playersLowGuids[i] = guid.LowType(playerGUID)
	}
	playersLowGuids[len(partyMembers)] = guid.LowType(leaderGUID)

	slotsWithLowGUIDs := make(map[guid.LowType]uint8)
	for k, v := range slots {
		slotsWithLowGUIDs[k.LowGUID] = v
	}

	err = s.eventsProducer.JoinedQueue(&events.MatchmakingEventPlayersQueuedPayload{
		RealmID:                        realmID,
		PlayersGUID:                    playersLowGuids,
		QueueSlotByPlayer:              slotsWithLowGUIDs,
		ArenaType:                      arenaType,
		IsRated:                        isRated,
		PVPQueueMinLVL:                 minLvl,
		PVPQueueMaxLVL:                 maxLvl,
		TypeID:                         uint8(typeID),
		AverageWaitingTimeMilliseconds: 0,
	})

	if err != nil {
		return fmt.Errorf("can't produce matchmaking event: %w", err)
	}

	return nil
}

// OnNoCrossRealmNodesAvailable is called when there were no cross-realm nodes available but now some nodes are available.
// This function should move the queued groups from the realm queue to the battle group queue.
func (s *battleGroundService) OnNoCrossRealmNodesAvailable() {
	realms, err := s.battleGroupsRepo.AllRealmsInBattleGroups(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("cannot get all realms")
		return
	}

	for _, realm := range realms {
		battlegroupID, err := s.battleGroupsRepo.BattleGroupIDByRealmID(context.Background(), realm)
		if err != nil {
			log.Error().Err(err).Msg("cannot get BattleGroupIDByRealmID")
			continue
		}

		battlegroupQueue := s.queues[QueueByRealmOrBattlegroupKey{
			BattlegroupID: battlegroupID,
			RealmID:       0,
		}]

		for queueID, bracketsMap := range s.queues[QueueByRealmOrBattlegroupKey{
			BattlegroupID: 0,
			RealmID:       realm,
		}] {
			for bracket, queue := range bracketsMap {
				groups := queue.GetAllQueuedGroups()
				if len(groups) == 0 {
					continue
				}

				queue.RemoveAllQueuedGroups()

				for _, group := range groups {
					s.removeQueueForGroupMembers(queue, &group)

					newQueue := battlegroupQueue[queueID][bracket]
					err = newQueue.AddQueuedGroup(&group)
					if err != nil {
						log.Error().Err(err).Msg("cannot add group to queue OnNoCrossRealmNodesAvailable")
					}

					s.addQueueForGroupMembers(newQueue, &group)
				}
			}
		}
	}

	log.Info().Msg("Crossrealm enabled")
}

// OnNoCrossRealmNodesUnAvailable is called when there are no cross-realm nodes available.
// This function should handle the transition of queued groups from the battle group queue back to the realm queue.
func (s *battleGroundService) OnNoCrossRealmNodesUnAvailable() {
	battlegroups, err := s.battleGroupsRepo.AllBattleGroupsIDs(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("cannot get all battlegroups")
		return
	}

	for _, battlegroup := range battlegroups {
		for queueID, bracketsMap := range s.queues[QueueByRealmOrBattlegroupKey{
			BattlegroupID: battlegroup,
			RealmID:       0,
		}] {
			for bracket, queue := range bracketsMap {
				groups := queue.GetAllQueuedGroups()
				if len(groups) == 0 {
					continue
				}

				queue.RemoveAllQueuedGroups()

				for _, group := range groups {
					realmQueues := s.queues[QueueByRealmOrBattlegroupKey{
						BattlegroupID: 0,
						RealmID:       group.RealmID,
					}]

					s.removeQueueForGroupMembers(queue, &group)

					newQueue := realmQueues[queueID][bracket]
					err = newQueue.AddQueuedGroup(&group)
					if err != nil {
						log.Error().Err(err).Msg("cannot add group to queue OnNoCrossRealmNodesUnAvailable")
					}

					s.addQueueForGroupMembers(newQueue, &group)
				}
			}
		}
	}
	log.Info().Msg("Crossrealm disabled")
}

// OnGameServerRemoved called when game server node removed because it stopped, or it's unhealthy.
// We need to remove all associated battlegrounds, otherwise we might still add players to those battlegrounds.
func (s *battleGroundService) OnGameServerRemoved(gs *events.ServerRegistryEventGSRemovedPayload) {
	bgs, err := s.battlegroundsRepo.DeleteAllWithGameServerAddress(context.Background(), gs.GameServer.Address)
	if err != nil {
		log.Error().Err(err).Str("address", gs.GameServer.Address).Msg("cannot delete bgs with game server address")
	}

	for _, bg := range bgs {
		for _, guid := range bg.ActivePlayersPerTeam[battleground.TeamHorde] {
			s.removeBattlegroundLinkForPlayer(BattlegroundKey{
				RealmID:       bg.RealmID,
				InstanceID:    bg.InstanceID,
				BattlegroupID: bg.BattleGroupID,
			}, uint64(guid.LowGUID), uint32(guid.RealmID))
		}
		for _, guid := range bg.ActivePlayersPerTeam[battleground.TeamAlliance] {
			s.removeBattlegroundLinkForPlayer(BattlegroundKey{
				RealmID:       bg.RealmID,
				InstanceID:    bg.InstanceID,
				BattlegroupID: bg.BattleGroupID,
			}, uint64(guid.LowGUID), uint32(guid.RealmID))
		}
	}
}

func (s *battleGroundService) CreateBattleground(
	ctx context.Context,
	template repo.BattlegroundTemplate,
	queueType battleground.QueueTypeID,
	bracketID BracketID,
	realmID, battlegroupID uint32,
	allianceGroups, hordeGroups []QueuedGroup,
) error {
	isCrossRealm := battlegroupID > 0
	// Just in case.
	if isCrossRealm {
		realmID = 0
	}

	gameserversResp, err := s.serversRegistryClient.AvailableGameServersForMapAndRealm(ctx, &pbServRegistry.AvailableGameServersForMapAndRealmRequest{
		Api:          matchmaking.SupportedServerRegistryVer,
		RealmID:      realmID,
		MapID:        template.MapID,
		IsCrossRealm: isCrossRealm,
	})
	if err != nil {
		return fmt.Errorf("cannot get available game servers (mapid - %d, realm - %d): %w", template.MapID, realmID, err)
	}

	if len(gameserversResp.GameServers) == 0 {
		return fmt.Errorf("no available game servers (mapid - %d, realm - %d)", template.MapID, realmID)
	}

	gameserverAddress := gameserversResp.GameServers[0].Address
	s.gameserverGRPCConnMgr.AddAddressMapping(gameserverAddress, gameserversResp.GameServers[0].GrpcAddress)

	gameServerGRPCClient, err := s.gameserverGRPCConnMgr.GRPCConnByGameServerAddress(gameserversResp.GameServers[0].Address)
	if err != nil {
		return fmt.Errorf("can't get game server grpc client, err: %w", err)
	}

	var alliancePlayers []uint64
	var hordePlayers []uint64

	for _, group := range allianceGroups {
		for _, m := range group.Members {
			alliancePlayers = append(alliancePlayers, uint64(m.LowGUID))
		}
		alliancePlayers = append(alliancePlayers, uint64(group.LeaderGUID.LowGUID))
	}

	for _, group := range hordeGroups {
		for _, m := range group.Members {
			hordePlayers = append(hordePlayers, uint64(m.LowGUID))
		}
		hordePlayers = append(hordePlayers, uint64(group.LeaderGUID.LowGUID))
	}

	minLvl, _ := battleground.LevelsDiapasonForBracket(uint8(bracketID))
	arenaType, isRated := queuePVPOptions(allianceGroups, hordeGroups)
	arenaStart := arenaStartOptionsForGroups(allianceGroups, hordeGroups, isCrossRealm)

	startBGResponse, err := gameServerGRPCClient.StartBattleground(ctx, &pb.StartBattlegroundRequest{
		Api:                           matchmaking.SupportedGameServerVer,
		BattlegroundTypeID:            pb.BattlegroundType(template.TypeID),
		ArenaType:                     uint32(arenaType),
		IsRated:                       isRated,
		MapID:                         template.MapID,
		BracketLvl:                    uint32(minLvl),
		PlayersToAddAlliance:          alliancePlayers,
		PlayersToAddHorde:             hordePlayers,
		AllianceArenaTeamID:           arenaStart.allianceTeamID,
		HordeArenaTeamID:              arenaStart.hordeTeamID,
		AllianceArenaMatchmakerRating: arenaStart.allianceMMR,
		HordeArenaMatchmakerRating:    arenaStart.hordeMMR,
	})
	if err != nil {
		return fmt.Errorf("start battleground failed: %w", err)
	}

	bg := &battleground.Battleground{
		InstanceID:         startBGResponse.InstanceID,
		MapID:              template.MapID,
		GameserverAddress:  gameserverAddress,
		BattlegroundTypeID: battleground.TypeID(template.TypeID),
		QueueTypeID:        queueType,
		BracketID:          uint8(bracketID),
		BattleGroupID:      battlegroupID,
		RealmID:            realmID,
		Status:             battleground.StatusWaitJoin,
		MinPlayersPerTeam:  template.MinPlayersPerTeam,
		MaxPlayersPerTeam:  template.MaxPlayersPerTeam,
	}

	bgHordeGroups := make([]battleground.QueuedGroup, len(hordeGroups))
	for i, group := range hordeGroups {
		bgHordeGroups[i] = battleground.QueuedGroup{
			LeaderGUID:                   group.LeaderGUID,
			Members:                      group.Members,
			SlotsPerMember:               group.SlotsPerMember,
			RealmID:                      group.RealmID,
			TeamID:                       group.TeamID,
			ArenaType:                    group.ArenaType,
			IsRated:                      group.IsRated,
			ArenaTeamID:                  group.ArenaTeamID,
			ArenaTeamRating:              group.ArenaTeamRating,
			ArenaMatchmakerRating:        group.ArenaMatchmakerRating,
			ArenaPreviousOpponentsTeamID: group.ArenaPreviousOpponentsTeamID,
			EnqueuedTime:                 group.EnqueuedTime,
		}
	}

	bgAllianceGroups := make([]battleground.QueuedGroup, len(allianceGroups))
	for i, group := range allianceGroups {
		bgAllianceGroups[i] = battleground.QueuedGroup{
			LeaderGUID:                   group.LeaderGUID,
			Members:                      group.Members,
			SlotsPerMember:               group.SlotsPerMember,
			RealmID:                      group.RealmID,
			TeamID:                       group.TeamID,
			ArenaType:                    group.ArenaType,
			IsRated:                      group.IsRated,
			ArenaTeamID:                  group.ArenaTeamID,
			ArenaTeamRating:              group.ArenaTeamRating,
			ArenaMatchmakerRating:        group.ArenaMatchmakerRating,
			ArenaPreviousOpponentsTeamID: group.ArenaPreviousOpponentsTeamID,
			EnqueuedTime:                 group.EnqueuedTime,
		}
	}

	err = bg.InviteGroups(s.eventsProducer, bgHordeGroups, battleground.TeamHorde)
	if err != nil {
		return fmt.Errorf("invite horde groups failed: %w", err)
	}

	err = bg.InviteGroups(s.eventsProducer, bgAllianceGroups, battleground.TeamAlliance)
	if err != nil {
		return fmt.Errorf("invite alliance groups failed: %w", err)
	}

	err = s.battlegroundsRepo.SaveBattleground(ctx, bg)
	if err != nil {
		return fmt.Errorf("save battleground failed: %w", err)
	}

	realmOrBGGroupQueueKey := QueueByRealmOrBattlegroupKey{
		BattlegroupID: bg.BattleGroupID,
		RealmID:       bg.RealmID,
	}
	queue := s.queues[realmOrBGGroupQueueKey][queueType][BracketID(bg.BracketID)]

	for _, group := range hordeGroups {
		s.removeQueueForGroupMembers(queue, &group)
		s.addBattlegroundForGroupMembers(bg, &group)
	}

	for _, group := range allianceGroups {
		s.removeQueueForGroupMembers(queue, &group)
		s.addBattlegroundForGroupMembers(bg, &group)
	}
	s.recordRatedArenaOpponents(allianceGroups, hordeGroups)

	log.Debug().
		Interface("RealmKey", realmOrBGGroupQueueKey).
		Uint8("QType", uint8(queueType)).
		Uint8("Bracket", bg.BracketID).
		Uint8("ArenaType", arenaType).
		Bool("IsRated", isRated).
		Uint32("InstanceID", bg.InstanceID).
		Uint32("MapID", bg.MapID).
		Uint32("BattlegroupID", bg.BattleGroupID).
		Uint32("AllianceArenaTeamID", arenaStart.allianceTeamID).
		Uint32("HordeArenaTeamID", arenaStart.hordeTeamID).
		Uint32("AllianceArenaMMR", arenaStart.allianceMMR).
		Uint32("HordeArenaMMR", arenaStart.hordeMMR).
		Interface("AlliancePlayers", alliancePlayers).
		Interface("HordePlayers", hordePlayers).
		Msg("Created New Battleground")

	return nil
}

func (s *battleGroundService) GetBattlegroundByBattlegroundKey(ctx context.Context, instanceID uint32, realmKey repo.RealmWithBattlegroupKey) (*battleground.Battleground, error) {
	return s.battlegroundsRepo.GetBattlegroundByInstanceID(ctx, instanceID, realmKey)
}

func (s *battleGroundService) PlayerLeftBattleground(ctx context.Context, playerGUID uint64, realmID, instanceID uint32, isCrossrealm bool) error {
	realmKey := repo.RealmWithBattlegroupKey{
		RealmID: realmID,
	}
	if isCrossrealm {
		realmKey.RealmID = 0
	}
	linkKey := BattlegroundKey{
		RealmID:    realmKey.RealmID,
		InstanceID: instanceID,
	}

	err := s.battlegroundsRepo.UpdateBattleground(ctx, instanceID, realmKey, func(b *battleground.Battleground) error {
		b.RemovePlayer(playerGUID, realmID)
		return nil
	})
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			s.removeBattlegroundLinkForPlayer(linkKey, playerGUID, realmID)
			return nil
		}

		return err
	}

	s.removeBattlegroundLinkForPlayer(linkKey, playerGUID, realmID)

	log.Debug().
		Uint64("playerGUID", playerGUID).
		Uint32("realmID", realmID).
		Uint32("instanceID", instanceID).
		Bool("isCrossrealm", isCrossrealm).
		Msg("TC9 PVP player left battleground")

	return nil
}

func (s *battleGroundService) PlayerJoinedBattleground(ctx context.Context, playerGUID uint64, realmID, instanceID uint32, isCrossrealm bool) error {
	realmKey := repo.RealmWithBattlegroupKey{
		RealmID: realmID,
	}
	if isCrossrealm {
		realmKey.RealmID = 0
	}

	err := s.battlegroundsRepo.UpdateBattleground(ctx, instanceID, realmKey, func(b *battleground.Battleground) error {
		found, team := b.TeamForInvitedPlayer(playerGUID, realmID)
		if !found {
			return fmt.Errorf("player not found in invited players")
		}

		b.RemovePlayer(playerGUID, realmID)
		b.AddActivePlayer(playerGUID, realmID, team)
		return nil
	})
	if err != nil {
		return err
	}

	log.Debug().
		Uint64("playerGUID", playerGUID).
		Uint32("realmID", realmID).
		Uint32("instanceID", instanceID).
		Bool("isCrossrealm", isCrossrealm).
		Msg("TC9 PVP player joined battleground")

	return nil
}

func (s *battleGroundService) InviteGroups(ctx context.Context, groups []QueuedGroup, bg *battleground.Battleground, team battleground.PVPTeam) error {
	bgGroups := make([]battleground.QueuedGroup, len(groups))
	for i, group := range groups {
		bgGroups[i] = battleground.QueuedGroup{
			LeaderGUID:                   group.LeaderGUID,
			Members:                      group.Members,
			SlotsPerMember:               group.SlotsPerMember,
			RealmID:                      group.RealmID,
			TeamID:                       group.TeamID,
			ArenaType:                    group.ArenaType,
			IsRated:                      group.IsRated,
			ArenaTeamID:                  group.ArenaTeamID,
			ArenaTeamRating:              group.ArenaTeamRating,
			ArenaMatchmakerRating:        group.ArenaMatchmakerRating,
			ArenaPreviousOpponentsTeamID: group.ArenaPreviousOpponentsTeamID,
			EnqueuedTime:                 group.EnqueuedTime,
		}
	}

	err := bg.InviteGroups(s.eventsProducer, bgGroups, team)
	if err != nil {
		return fmt.Errorf("invite horde groups failed: %w", err)
	}

	err = s.battlegroundsRepo.SaveBattleground(ctx, bg)
	if err != nil {
		return fmt.Errorf("save battleground failed: %w", err)
	}

	queue := s.queues[QueueByRealmOrBattlegroupKey{
		BattlegroupID: bg.BattleGroupID,
		RealmID:       bg.RealmID,
	}][bg.QueueTypeID][BracketID(bg.BracketID)]

	for _, group := range groups {
		s.removeQueueForGroupMembers(queue, &group)
		s.addBattlegroundForGroupMembers(bg, &group)
	}

	return nil
}

func (s *battleGroundService) BattlegroundStatusChanged(ctx context.Context, status battleground.Status, realmID, instanceID uint32, isCrossrealm bool) error {
	realmKey := repo.RealmWithBattlegroupKey{
		RealmID: realmID,
	}
	if isCrossrealm {
		realmKey.RealmID = 0
	}

	err := s.battlegroundsRepo.UpdateBattleground(ctx, instanceID, realmKey, func(b *battleground.Battleground) error {
		b.Status = status
		if b.Status == battleground.StatusEnded {
			for _, guid := range b.ActivePlayersPerTeam[battleground.TeamHorde] {
				s.removeBattlegroundLinkForPlayer(BattlegroundKey{
					RealmID:       b.RealmID,
					InstanceID:    b.InstanceID,
					BattlegroupID: b.BattleGroupID,
				}, uint64(guid.LowGUID), uint32(guid.RealmID))
			}
			for _, guid := range b.ActivePlayersPerTeam[battleground.TeamAlliance] {
				s.removeBattlegroundLinkForPlayer(BattlegroundKey{
					RealmID:       b.RealmID,
					InstanceID:    b.InstanceID,
					BattlegroupID: b.BattleGroupID,
				}, uint64(guid.LowGUID), uint32(guid.RealmID))
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil
		}

		return err
	}

	log.Debug().
		Uint8("status", uint8(status)).
		Uint32("realmID", realmID).
		Uint32("instanceID", instanceID).
		Bool("isCrossrealm", isCrossrealm).
		Msg("TC9 PVP battleground status changed")

	return nil
}

func (s *battleGroundService) RemovePlayerFromQueue(ctx context.Context, playerGUID uint64, realmID uint32, typeID battleground.QueueTypeID) error {
	playerGUIDUnwrapped := guid.PlayerUnwrapped{
		RealmID: uint16(realmID),
		LowGUID: guid.LowType(playerGUID),
	}
	links := s.GetQueueOrBattlegroundLinkForPlayer(QueuesByRealmAndPlayerKey{playerGUIDUnwrapped})
	for _, link := range links {
		if link.BattlegroundKey != nil {
			bg, err := s.battlegroundsRepo.GetBattlegroundByInstanceID(ctx, link.BattlegroundKey.InstanceID, repo.RealmWithBattlegroupKey{
				RealmID:       link.BattlegroundKey.RealmID,
				BattlegroupID: link.BattlegroundKey.BattlegroupID,
			})

			if err != nil {
				return err
			}

			if bg == nil {
				continue
			}

			if bg.QueueTypeID != typeID {
				continue
			}

			err = s.PlayerLeftBattleground(ctx, playerGUID, realmID, link.BattlegroundKey.InstanceID, link.BattlegroundKey.BattlegroupID != 0)
			if err != nil {
				return err
			}
		} else if link.Queue.GetQueueTypeID() == typeID {

			if queuedGroup := link.Queue.QueuedGroupByPlayer(playerGUIDUnwrapped); queuedGroup != nil {
				s.removeQueueForGroupMembers(link.Queue, queuedGroup)
				link.Queue.RemoveQueuedGroup(playerGUIDUnwrapped)
			}
		}
	}

	return nil
}

func (s *battleGroundService) PlayerBecomeOffline(ctx context.Context, playerGUID uint64, realmID uint32) error {
	playerGUIDUnwrapped := guid.PlayerUnwrapped{
		RealmID: uint16(realmID),
		LowGUID: guid.LowType(playerGUID),
	}

	s.playersQueueOrBattlegroundMutex.Lock()

	links := s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{playerGUIDUnwrapped}]

	linksCopy := make([]QueueOrBattlegroundLink, len(links))

	for i, link := range links {
		linksCopy[i] = QueueOrBattlegroundLink{
			Queue:           link.Queue,
			BattlegroundKey: nil,
		}
		if link.BattlegroundKey != nil {
			bgKey := *link.BattlegroundKey
			linksCopy[i].BattlegroundKey = &bgKey
		}
	}

	s.playersQueueOrBattlegroundMutex.Unlock()

	for _, link := range linksCopy {
		if link.BattlegroundKey != nil {
			err := s.PlayerLeftBattleground(ctx, playerGUID, realmID, link.BattlegroundKey.InstanceID, link.BattlegroundKey.BattlegroupID != 0)
			if err != nil {
				return err
			}
			continue
		}

		if queuedGroup := link.Queue.QueuedGroupByPlayer(playerGUIDUnwrapped); queuedGroup != nil {
			s.removeQueueForGroupMembers(link.Queue, queuedGroup)
			link.Queue.RemoveQueuedGroup(playerGUIDUnwrapped)
		}
	}

	return nil
}

func (s *battleGroundService) ProcessExpiredBattlegroundInvites(ctx context.Context) {
	for {
		select {
		case <-time.After(time.Second):
			bgs, err := s.battlegroundsRepo.GetAllActiveBattlegrounds(ctx)
			if err != nil {
				log.Err(err).Msg("failed to get all active battlegrounds")
				break
			}
			for _, bg := range bgs {
				for _, invites := range bg.InvitedPlayersPerTeam {
					for _, invite := range invites {
						if time.Since(invite.InvitedTime) > time.Minute {
							err = s.battlegroundsRepo.UpdateBattleground(
								ctx,
								bg.InstanceID,
								repo.RealmWithBattlegroupKey{RealmID: bg.RealmID, BattlegroupID: bg.BattleGroupID},
								func(b *battleground.Battleground) error {
									b.RemovePlayerFromInvite(uint64(invite.GUID.LowGUID), uint32(invite.GUID.RealmID))
									return nil
								},
							)
							if err != nil {
								log.Err(err).Msg("failed to remove invite from Battleground")
								continue
							}

							err = s.RemovePlayerFromQueue(ctx, uint64(invite.GUID.LowGUID), uint32(invite.GUID.RealmID), bg.QueueTypeID)
							if err != nil {
								log.Err(err).Msg("failed to remove invited player from queue")
								continue
							}

							slot := uint8(0) // TODO: provide real slot
							err = s.eventsProducer.InviteExpired(&events.MatchmakingEventPlayersInviteExpiredPayload{
								RealmID:           uint32(invite.GUID.RealmID),
								PlayersGUID:       []guid.LowType{invite.GUID.LowGUID},
								QueueSlotByPlayer: map[guid.LowType]uint8{invite.GUID.LowGUID: slot},
							})
							if err != nil {
								log.Err(err).Msg("Failed to add invited player to queue")
							}
						}
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *battleGroundService) addQueueForGroupMembers(q PVPQueue, group *QueuedGroup) map[guid.PlayerUnwrapped]uint8 {
	return s.addQueueOrBattlegroundLinkForGroupMembers(QueueOrBattlegroundLink{Queue: q}, group)
}

func (s *battleGroundService) addBattlegroundForGroupMembers(b *battleground.Battleground, group *QueuedGroup) map[guid.PlayerUnwrapped]uint8 {
	return s.addQueueOrBattlegroundLinkForGroupMembers(QueueOrBattlegroundLink{BattlegroundKey: &BattlegroundKey{RealmID: b.RealmID, BattlegroupID: b.BattleGroupID, InstanceID: b.InstanceID}}, group)
}

func (s *battleGroundService) addQueueOrBattlegroundLinkForGroupMembers(q QueueOrBattlegroundLink, group *QueuedGroup) map[guid.PlayerUnwrapped]uint8 {
	s.playersQueueOrBattlegroundMutex.Lock()
	defer s.playersQueueOrBattlegroundMutex.Unlock()

	group.SlotsPerMember = map[guid.PlayerUnwrapped]uint8{}
	for _, playerGUID := range group.Members {
		s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{playerGUID}] = append(s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
			playerGUID,
		}], q)

		group.SlotsPerMember[playerGUID] = uint8(len(s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
			playerGUID,
		}]) - 1)
	}

	s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		group.LeaderGUID,
	}] = append(s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		group.LeaderGUID,
	}], q)

	group.SlotsPerMember[group.LeaderGUID] = uint8(len(s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		group.LeaderGUID,
	}]) - 1)

	return group.SlotsPerMember
}

func (s *battleGroundService) removeQueueForGroupMembers(q PVPQueue, group *QueuedGroup) {
	s.playersQueueOrBattlegroundMutex.Lock()
	defer s.playersQueueOrBattlegroundMutex.Unlock()

	for _, playerGUID := range group.Members {
		links := s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
			playerGUID,
		}]
		for i, link := range links {
			if link.Queue != nil && link.Queue.GetQueueTypeID() == q.GetQueueTypeID() {
				s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
					playerGUID,
				}] = append(links[:i], links[i+1:]...)
				break
			}
		}
	}

	links := s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		group.LeaderGUID,
	}]

	for i, link := range links {
		if link.Queue != nil && link.Queue.GetQueueTypeID() == q.GetQueueTypeID() {
			s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
				group.LeaderGUID,
			}] = append(links[:i], links[i+1:]...)

			break
		}
	}
}

func (s *battleGroundService) removeBattlegroundLinkForPlayer(bgKey BattlegroundKey, player uint64, realmID uint32) {
	s.playersQueueOrBattlegroundMutex.Lock()
	defer s.playersQueueOrBattlegroundMutex.Unlock()

	playerGUIDUnwrapped := guid.PlayerUnwrapped{
		RealmID: uint16(realmID),
		LowGUID: guid.LowType(player),
	}

	links := s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
		playerGUIDUnwrapped,
	}]
	for i, link := range links {
		if link.BattlegroundKey != nil && link.BattlegroundKey.InstanceID == bgKey.InstanceID && link.BattlegroundKey.RealmID == bgKey.RealmID {
			s.playersQueueOrBattleground[QueuesByRealmAndPlayerKey{
				playerGUIDUnwrapped,
			}] = append(links[:i], links[i+1:]...)
			break
		}
	}
}

func (s *battleGroundService) GetQueueOrBattlegroundLinkForPlayer(k QueuesByRealmAndPlayerKey) []QueueOrBattlegroundLink {
	s.playersQueueOrBattlegroundMutex.RLock()
	defer s.playersQueueOrBattlegroundMutex.RUnlock()

	return s.playersQueueOrBattleground[k]
}

func queuePVPOptions(allianceGroups, hordeGroups []QueuedGroup) (uint8, bool) {
	for _, group := range allianceGroups {
		return group.ArenaType, group.IsRated
	}
	for _, group := range hordeGroups {
		return group.ArenaType, group.IsRated
	}
	return 0, false
}

type arenaStartOptions struct {
	allianceTeamID uint32
	hordeTeamID    uint32
	allianceMMR    uint32
	hordeMMR       uint32
}

func arenaStartOptionsForGroups(allianceGroups, hordeGroups []QueuedGroup, isCrossRealm bool) arenaStartOptions {
	var opts arenaStartOptions
	for _, group := range allianceGroups {
		if group.IsRated {
			opts.allianceTeamID = arenaTeamIDForStart(group, isCrossRealm)
			opts.allianceMMR = group.ArenaMatchmakerRating
			break
		}
	}
	for _, group := range hordeGroups {
		if group.IsRated {
			opts.hordeTeamID = arenaTeamIDForStart(group, isCrossRealm)
			opts.hordeMMR = group.ArenaMatchmakerRating
			break
		}
	}
	return opts
}

func arenaTeamIDForStart(group QueuedGroup, isCrossRealm bool) uint32 {
	if !isCrossRealm {
		return group.ArenaTeamID
	}
	return wowarena.NewCrossrealmTeamID(uint16(group.RealmID), group.ArenaTeamID)
}

func (s *battleGroundService) previousRatedArenaOpponentID(realmID, teamID uint32) uint32 {
	if teamID == 0 {
		return 0
	}
	key := wowarena.NewCrossrealmTeamID(uint16(realmID), teamID)
	s.arenaPreviousOpponentsMutex.RLock()
	defer s.arenaPreviousOpponentsMutex.RUnlock()
	return s.arenaPreviousOpponents[key]
}

func (s *battleGroundService) recordRatedArenaOpponents(allianceGroups, hordeGroups []QueuedGroup) {
	allianceTeamID := ratedArenaTeamIDForOpponentTracking(allianceGroups)
	hordeTeamID := ratedArenaTeamIDForOpponentTracking(hordeGroups)
	if allianceTeamID == 0 || hordeTeamID == 0 {
		return
	}

	s.arenaPreviousOpponentsMutex.Lock()
	defer s.arenaPreviousOpponentsMutex.Unlock()
	s.arenaPreviousOpponents[allianceTeamID] = hordeTeamID
	s.arenaPreviousOpponents[hordeTeamID] = allianceTeamID
}

func ratedArenaTeamIDForOpponentTracking(groups []QueuedGroup) uint32 {
	for _, group := range groups {
		if group.IsRated && group.ArenaTeamID != 0 {
			return wowarena.NewCrossrealmTeamID(uint16(group.RealmID), group.ArenaTeamID)
		}
	}
	return 0
}

func generateQueuesForAllBattlegroundTypes(service BattleGroundService, realmIDs []uint32, battlegroups []uint32) map[QueueByRealmOrBattlegroupKey]map[battleground.QueueTypeID]map[BracketID]PVPQueue {
	res := map[QueueByRealmOrBattlegroupKey]map[battleground.QueueTypeID]map[BracketID]PVPQueue{}
	types := []battleground.QueueTypeID{
		battleground.QueueTypeIDAlteracValley,
		battleground.QueueTypeIDWarsongGulch,
		battleground.QueueTypeIDArathiBasin,
		battleground.QueueTypeIDAllArenas,
		battleground.QueueTypeIDEyeOfTheStorm,
		battleground.QueueTypeIDIsleOfConquest,
		battleground.QueueTypeIDStrandOfTheAncients,
		battleground.QueueTypeIDRandomBattleground,
	}

	var setupForRealmOrBattlegroup = func(realmID uint32, battlegroupID uint32) {
		k := QueueByRealmOrBattlegroupKey{
			BattlegroupID: battlegroupID,
			RealmID:       realmID,
		}

		res[k] = map[battleground.QueueTypeID]map[BracketID]PVPQueue{}
		for _, typeID := range types {
			template := service.TemplateForQueueTypeID(context.Background(), typeID)
			res[k][typeID] = map[BracketID]PVPQueue{}
			for _, bracket := range template.GetAllBrackets() {
				if typeID == battleground.QueueTypeIDRandomBattleground {
					res[k][typeID][BracketID(bracket)] = NewBattlegroundRandomQueue(service, service, template, realmID, battlegroupID, bracket)
				} else {
					res[k][typeID][BracketID(bracket)] = NewGenericBattlegroundQueue(service, service, template, realmID, battlegroupID, bracket)
				}
			}
		}
	}

	for _, realmID := range realmIDs {
		setupForRealmOrBattlegroup(realmID, 0)
	}

	for _, battlegroup := range battlegroups {
		setupForRealmOrBattlegroup(0, battlegroup)
	}

	return res
}
