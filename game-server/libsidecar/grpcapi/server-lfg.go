package grpcapi

import (
	"context"
	"time"

	"github.com/walkline/ToCloud9/game-server/libsidecar/queue"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
)

// Proposal materialization runs after every member has already accepted the
// client proposal. Keep this as an infrastructure redirect/load budget instead
// of AzerothCore's 40s answer window.
const lfgMaterializeProposalTimeout = 90 * time.Second

func (w *WorldServerGRPCAPI) GetLfgPlayerLockInfo(ctx context.Context, request *pb.GetLfgPlayerLockInfoRequest) (*pb.GetLfgPlayerLockInfoResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		resp *LFGPlayerLockInfoResponse
		err  error
	}
	var resp respType

	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		if w.bindings.GetLFGPlayerLockInfo == nil {
			respChan <- respType{err: LFGPlayerLockInfoError(LFGPlayerLockInfoErrorCodeNoHandler)}
			close(respChan)
			return
		}
		r, err := w.bindings.GetLFGPlayerLockInfo(LFGPlayerLockInfoRequest{
			PlayerGUID:     request.PlayerGUID,
			DungeonEntries: request.DungeonEntries,
		})
		respChan <- respType{
			resp: r,
			err:  err,
		}
		close(respChan)
	}))
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case resp = <-respChan:
	}

	if resp.err != nil {
		switch LFGPlayerLockInfoErrorCodeForError(resp.err) {
		case LFGPlayerLockInfoErrorCodePlayerNotFound:
			return &pb.GetLfgPlayerLockInfoResponse{
				Api:    LibVer,
				Status: pb.GetLfgPlayerLockInfoResponse_PlayerNotFound,
			}, nil
		default:
			return &pb.GetLfgPlayerLockInfoResponse{
				Api:    LibVer,
				Status: pb.GetLfgPlayerLockInfoResponse_InternalError,
			}, nil
		}
	}
	if resp.resp == nil {
		return &pb.GetLfgPlayerLockInfoResponse{
			Api:    LibVer,
			Status: pb.GetLfgPlayerLockInfoResponse_InternalError,
		}, nil
	}

	locks := make([]*pb.LfgDungeonLock, 0, len(resp.resp.Locks))
	for _, lock := range resp.resp.Locks {
		locks = append(locks, &pb.LfgDungeonLock{
			DungeonEntry: lock.DungeonEntry,
			LockStatus:   lock.LockStatus,
		})
	}

	return &pb.GetLfgPlayerLockInfoResponse{
		Api:                 LibVer,
		Status:              pb.GetLfgPlayerLockInfoResponse_Success,
		Locks:               locks,
		JoinResult:          resp.resp.JoinResult,
		ValidDungeonEntries: resp.resp.ValidDungeonEntries,
	}, nil
}

func (w *WorldServerGRPCAPI) GetLfgPlayerInfo(ctx context.Context, request *pb.GetLfgPlayerInfoRequest) (*pb.GetLfgPlayerInfoResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		resp *LFGPlayerInfoResponse
		err  error
	}
	var resp respType
	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		if w.bindings.GetLFGPlayerInfo == nil {
			respChan <- respType{err: LFGPlayerInfoError(LFGPlayerInfoErrorCodeNoHandler)}
			close(respChan)
			return
		}
		r, err := w.bindings.GetLFGPlayerInfo(LFGPlayerInfoRequest{
			PlayerGUID: request.GetPlayerGUID(),
		})
		respChan <- respType{
			resp: r,
			err:  err,
		}
		close(respChan)
	}))

	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case resp = <-respChan:
	}

	if resp.err != nil {
		return &pb.GetLfgPlayerInfoResponse{
			Api:    LibVer,
			Status: lfgPlayerInfoStatusForError(resp.err),
		}, nil
	}
	if resp.resp == nil {
		return &pb.GetLfgPlayerInfoResponse{
			Api:    LibVer,
			Status: pb.GetLfgPlayerInfoResponse_InternalError,
		}, nil
	}

	randomDungeons := make([]*pb.LfgRandomDungeonInfo, 0, len(resp.resp.RandomDungeons))
	for _, dungeon := range resp.resp.RandomDungeons {
		rewardItems := make([]*pb.LfgRewardItem, 0, len(dungeon.RewardItems))
		for _, item := range dungeon.RewardItems {
			rewardItems = append(rewardItems, &pb.LfgRewardItem{
				ItemID:    item.ItemID,
				DisplayID: item.DisplayID,
				Count:     item.Count,
			})
		}
		randomDungeons = append(randomDungeons, &pb.LfgRandomDungeonInfo{
			DungeonEntry:   dungeon.DungeonEntry,
			Done:           dungeon.Done,
			RewardMoney:    dungeon.RewardMoney,
			RewardXP:       dungeon.RewardXP,
			RewardUnknown1: dungeon.RewardUnknown1,
			RewardUnknown2: dungeon.RewardUnknown2,
			RewardItems:    rewardItems,
		})
	}

	locks := make([]*pb.LfgDungeonLock, 0, len(resp.resp.Locks))
	for _, lock := range resp.resp.Locks {
		locks = append(locks, &pb.LfgDungeonLock{
			DungeonEntry: lock.DungeonEntry,
			LockStatus:   lock.LockStatus,
		})
	}

	return &pb.GetLfgPlayerInfoResponse{
		Api:            LibVer,
		Status:         pb.GetLfgPlayerInfoResponse_Success,
		RandomDungeons: randomDungeons,
		Locks:          locks,
	}, nil
}

func lfgPlayerInfoStatusForError(err error) pb.GetLfgPlayerInfoResponse_Status {
	switch LFGPlayerInfoErrorCodeForError(err) {
	case LFGPlayerInfoErrorCodeNoHandler:
		return pb.GetLfgPlayerInfoResponse_NoHandler
	case LFGPlayerInfoErrorCodePlayerNotFound:
		return pb.GetLfgPlayerInfoResponse_PlayerNotFound
	default:
		return pb.GetLfgPlayerInfoResponse_InternalError
	}
}

func (w *WorldServerGRPCAPI) GetLfgDungeonInfo(ctx context.Context, request *pb.GetLfgDungeonInfoRequest) (*pb.GetLfgDungeonInfoResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	type respType struct {
		resp *LFGDungeonInfoResponse
		err  error
	}
	respChan := make(chan respType, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		if w.bindings.GetLFGDungeonInfo == nil {
			respChan <- respType{err: LFGDungeonInfoError(LFGDungeonInfoErrorCodeNoHandler)}
			close(respChan)
			return
		}
		r, err := w.bindings.GetLFGDungeonInfo(LFGDungeonInfoRequest{
			DungeonEntry: request.GetDungeonEntry(),
		})
		respChan <- respType{
			resp: r,
			err:  err,
		}
		close(respChan)
	}))

	var resp respType
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case resp = <-respChan:
	}

	if resp.err != nil {
		return &pb.GetLfgDungeonInfoResponse{
			Api:    LibVer,
			Status: lfgDungeonInfoStatusForError(resp.err),
		}, nil
	}
	if resp.resp == nil {
		return &pb.GetLfgDungeonInfoResponse{
			Api:    LibVer,
			Status: pb.GetLfgDungeonInfoResponse_InternalError,
		}, nil
	}

	return &pb.GetLfgDungeonInfoResponse{
		Api:          LibVer,
		Status:       pb.GetLfgDungeonInfoResponse_Success,
		DungeonEntry: resp.resp.DungeonEntry,
		DungeonID:    resp.resp.DungeonID,
		MapID:        resp.resp.MapID,
		TypeID:       resp.resp.TypeID,
		Difficulty:   resp.resp.Difficulty,
	}, nil
}

func lfgDungeonInfoStatusForError(err error) pb.GetLfgDungeonInfoResponse_Status {
	switch LFGDungeonInfoErrorCodeForError(err) {
	case LFGDungeonInfoErrorCodeNoHandler:
		return pb.GetLfgDungeonInfoResponse_NoHandler
	case LFGDungeonInfoErrorCodeDungeonNotFound:
		return pb.GetLfgDungeonInfoResponse_DungeonNotFound
	default:
		return pb.GetLfgDungeonInfoResponse_InternalError
	}
}

func (w *WorldServerGRPCAPI) TeleportLfgPlayer(ctx context.Context, request *pb.TeleportLfgPlayerRequest) (*pb.TeleportLfgPlayerResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	respChan := make(chan error, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		if w.bindings.TeleportLFGPlayer == nil {
			respChan <- LFGTeleportPlayerError(LFGTeleportPlayerErrorCodeNoHandler)
			close(respChan)
			return
		}
		respChan <- w.bindings.TeleportLFGPlayer(LFGTeleportPlayerRequest{
			PlayerGUID:   request.GetPlayerGUID(),
			Out:          request.GetOut(),
			DungeonEntry: request.GetDungeonEntry(),
		})
		close(respChan)
	}))

	var err error
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case err = <-respChan:
	}

	return &pb.TeleportLfgPlayerResponse{
		Api:    LibVer,
		Status: teleportLfgPlayerStatusForError(err),
	}, nil
}

func teleportLfgPlayerStatusForError(err error) pb.TeleportLfgPlayerResponse_Status {
	if err == nil {
		return pb.TeleportLfgPlayerResponse_Success
	}
	switch LFGTeleportPlayerErrorCodeForError(err) {
	case LFGTeleportPlayerErrorCodeNoHandler:
		return pb.TeleportLfgPlayerResponse_NoHandler
	case LFGTeleportPlayerErrorCodePlayerNotFound:
		return pb.TeleportLfgPlayerResponse_PlayerNotFound
	default:
		return pb.TeleportLfgPlayerResponse_InternalError
	}
}

func (w *WorldServerGRPCAPI) SetLfgBootVote(ctx context.Context, request *pb.SetLfgBootVoteRequest) (*pb.SetLfgBootVoteResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	respChan := make(chan error, 1)

	w.writeQueue.Push(queue.HandlerFunc(func() {
		if w.bindings.SetLFGBootVote == nil {
			respChan <- LFGSetBootVoteError(LFGSetBootVoteErrorCodeNoHandler)
			close(respChan)
			return
		}
		respChan <- w.bindings.SetLFGBootVote(LFGSetBootVoteRequest{
			PlayerGUID: request.GetPlayerGUID(),
			Agree:      request.GetAgree(),
		})
		close(respChan)
	}))

	var err error
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case err = <-respChan:
	}

	return &pb.SetLfgBootVoteResponse{
		Api:    LibVer,
		Status: setLfgBootVoteStatusForError(err),
	}, nil
}

func setLfgBootVoteStatusForError(err error) pb.SetLfgBootVoteResponse_Status {
	if err == nil {
		return pb.SetLfgBootVoteResponse_Success
	}
	switch LFGSetBootVoteErrorCodeForError(err) {
	case LFGSetBootVoteErrorCodeNoHandler:
		return pb.SetLfgBootVoteResponse_NoHandler
	case LFGSetBootVoteErrorCodePlayerNotFound:
		return pb.SetLfgBootVoteResponse_PlayerNotFound
	default:
		return pb.SetLfgBootVoteResponse_InternalError
	}
}

func (w *WorldServerGRPCAPI) MaterializeLfgProposal(ctx context.Context, request *pb.MaterializeLfgProposalRequest) (*pb.MaterializeLfgProposalResponse, error) {
	timeout := w.timeout
	if timeout < lfgMaterializeProposalTimeout {
		timeout = lfgMaterializeProposalTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	respChan := make(chan error, 1)

	members := make([]LFGMaterializeProposalMember, 0, len(request.GetMembers()))
	for _, member := range request.GetMembers() {
		members = append(members, LFGMaterializeProposalMember{
			PlayerGUID:      member.GetPlayerGUID(),
			SelectedRoles:   uint8(member.GetSelectedRoles()),
			AssignedRole:    uint8(member.GetAssignedRole()),
			QueueLeaderGUID: member.GetQueueLeaderGUID(),
		})
	}

	w.writeQueue.Push(queue.HandlerFunc(func() {
		if w.bindings.MaterializeLFGProposal == nil {
			respChan <- LFGMaterializeProposalError(LFGMaterializeProposalErrorCodeNoHandler)
			close(respChan)
			return
		}

		respChan <- w.bindings.MaterializeLFGProposal(LFGMaterializeProposalRequest{
			RealmID:      request.GetRealmID(),
			ProposalID:   request.GetProposalID(),
			DungeonEntry: request.GetDungeonEntry(),
			LeaderGUID:   request.GetLeaderGUID(),
			Members:      members,
		})
		close(respChan)
	}))

	var err error
	select {
	case <-ctx.Done():
		return nil, ErrTimeout
	case err = <-respChan:
	}

	return &pb.MaterializeLfgProposalResponse{
		Api:    LibVer,
		Status: materializeLfgProposalStatusForError(err),
	}, nil
}

func materializeLfgProposalStatusForError(err error) pb.MaterializeLfgProposalResponse_Status {
	if err == nil {
		return pb.MaterializeLfgProposalResponse_Success
	}

	switch LFGMaterializeProposalErrorCodeForError(err) {
	case LFGMaterializeProposalErrorCodeNoHandler:
		return pb.MaterializeLfgProposalResponse_NoHandler
	case LFGMaterializeProposalErrorCodeDungeonNotFound:
		return pb.MaterializeLfgProposalResponse_DungeonNotFound
	case LFGMaterializeProposalErrorCodeNoLocalPlayer:
		return pb.MaterializeLfgProposalResponse_NoLocalPlayer
	default:
		return pb.MaterializeLfgProposalResponse_InternalError
	}
}
