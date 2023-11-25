package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/groupserver"
	"github.com/walkline/ToCloud9/apps/groupserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

var (
	ErrAlreadyInGroup      = errors.New("player already in group")
	ErrNoPermissions       = errors.New("player has not enough permissions")
	ErrGroupFull           = errors.New("group is full")
	ErrGroupNotFound       = errors.New("group not found")
	ErrGroupMemberNotFound = errors.New("group member not found")
)

type MessageType uint8

const (
	MessageTypeGroup       MessageType = 0x2
	MessageTypeGroupLeader MessageType = 0x33
	MessageTypeRaid        MessageType = 0x3
	MessageTypeRaidLeader  MessageType = 0x27
)

type GroupsService interface {
	GroupByID(ctx context.Context, realmID uint32, groupID uint) (*repo.Group, error)
	GroupByMemberGUID(ctx context.Context, realmID uint32, memberGUID uint64) (*repo.Group, error)
	GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error)

	Invite(ctx context.Context, realmID uint32, inviter, invited uint64, inviterName, invitedName string) error
	Uninvite(ctx context.Context, realmID uint32, initiator, target uint64, reason string) error
	Leave(ctx context.Context, realmID uint32, player uint64) error

	ChangeLeader(ctx context.Context, realmID uint32, player, newLeader uint64) error
	ConvertToRaid(ctx context.Context, realmID uint32, player uint64) error

	AcceptInvite(ctx context.Context, realmID uint32, player uint64) error

	SendMessage(ctx context.Context, realmID uint32, senderGUID uint64, message string, lang uint32, messageType MessageType) error

	SetTargetIcon(ctx context.Context, realmID uint32, updaterGUID uint64, iconID uint8, targetGUID uint64) error
	SetLootMethod(ctx context.Context, realmID uint32, updaterGUID uint64, method uint8, lootMaster uint64, lootThreshold uint8) error

	SetDungeonDifficulty(ctx context.Context, realmID uint32, updaterGUID uint64, difficulty uint8) error
	SetRaidDifficulty(ctx context.Context, realmID uint32, updaterGUID uint64, difficulty uint8) error

	// LBCharacterLoggedInHandler updates cache with player logged in.
	events.LBCharacterLoggedInHandler
	// LBCharacterLoggedOutHandler updates cache with player logged out.
	events.LBCharacterLoggedOutHandler
}

func NewGroupsService(r repo.GroupsRepo, ep events.GroupServiceProducer) GroupsService {
	return &groupServiceImpl{
		r:  r,
		ep: ep,
	}
}

type groupServiceImpl struct {
	r  repo.GroupsRepo
	ep events.GroupServiceProducer
}

func (g groupServiceImpl) GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error) {
	return g.r.GroupIDByPlayer(ctx, realmID, player)
}

func (g groupServiceImpl) GroupByID(ctx context.Context, realmID uint32, groupID uint) (*repo.Group, error) {
	return g.r.GroupByID(ctx, realmID, groupID, true)
}

func (g groupServiceImpl) GroupByMemberGUID(ctx context.Context, realmID uint32, memberGUID uint64) (*repo.Group, error) {
	groupID, err := g.GroupIDByPlayer(ctx, realmID, memberGUID)
	if err != nil {
		return nil, err
	}

	return g.GroupByID(ctx, realmID, groupID)
}

func (g groupServiceImpl) Invite(ctx context.Context, realmID uint32, inviter, invited uint64, inviterName, invitedName string) error {
	groupID, err := g.r.GroupIDByPlayer(ctx, realmID, invited)
	if err != nil {
		return err
	}

	if groupID != 0 {
		return ErrAlreadyInGroup
	}

	inviterGroupID, err := g.r.GroupIDByPlayer(ctx, realmID, inviter)
	if err != nil {
		return err
	}

	if inviterGroupID == 0 {
		if err = g.r.AddInvite(ctx, realmID, repo.GroupInvite{
			Inviter:     inviter,
			InviterName: inviterName,
			Invitee:     invited,
			InviteeName: invitedName,
			GroupID:     0,
		}); err != nil {
			return err
		}

		err = g.ep.InviteCreated(&events.GroupEventInviteCreatedPayload{
			ServiceID:   groupserver.ServiceID,
			RealmID:     realmID,
			GroupID:     0,
			InviterGUID: inviter,
			InviterName: inviterName,
			InviteeGUID: invited,
			InviteeName: invitedName,
		})

		if err != nil {
			log.Error().Err(err).Msg("can't create invite created event")
		}

		return nil
	}

	group, err := g.r.GroupByID(ctx, realmID, inviterGroupID, true)
	if err != nil {
		return err
	}

	member := group.MemberByGUID(inviter)
	if member == nil {
		return fmt.Errorf("can't find player %d in the guild %d", inviter, inviterGroupID)
	}

	if !(group.LeaderGUID == inviter || member.IsAssistant()) {
		return ErrNoPermissions
	}

	if group.IsFull() {
		return ErrGroupFull
	}

	if err = g.r.AddInvite(ctx, realmID, repo.GroupInvite{
		Inviter:     inviter,
		InviterName: inviterName,
		Invitee:     invited,
		InviteeName: invitedName,
		GroupID:     inviterGroupID,
	}); err != nil {
		return err
	}

	err = g.ep.InviteCreated(&events.GroupEventInviteCreatedPayload{
		ServiceID:   groupserver.ServiceID,
		RealmID:     realmID,
		GroupID:     inviterGroupID,
		InviterGUID: inviter,
		InviterName: inviterName,
		InviteeGUID: invited,
		InviteeName: invitedName,
	})

	if err != nil {
		log.Error().Err(err).Msg("can't create invite created event")
	}

	return nil
}

func (g groupServiceImpl) AcceptInvite(ctx context.Context, realmID uint32, player uint64) error {
	invite, err := g.r.GetInviteByInvitedPlayer(ctx, realmID, player)
	if err != nil {
		return err
	}

	if invite.GroupID == 0 {
		return g.createGroup(ctx, realmID, invite)
	}

	group, err := g.r.GroupByID(ctx, realmID, invite.GroupID, true)
	if err != nil {
		return err
	}

	return g.addMember(ctx, realmID, group, invite)
}

func (g groupServiceImpl) Uninvite(ctx context.Context, realmID uint32, initiator, target uint64, reason string) error {
	groupID, err := g.r.GroupIDByPlayer(ctx, realmID, initiator)
	if err != nil {
		return fmt.Errorf("can't get groupID, err: %w", err)
	}
	if groupID == 0 {
		return ErrGroupNotFound
	}

	group, err := g.r.GroupByID(ctx, realmID, groupID, true)
	if err != nil {
		return fmt.Errorf("can't get group, err: %w", err)
	}

	if group == nil {
		return ErrGroupNotFound
	}

	targetMember := group.MemberByGUID(target)
	if targetMember == nil {
		return ErrGroupNotFound
	}

	if group.LeaderGUID != initiator {
		return ErrNoPermissions
	}

	membersCount := len(group.Members)

	if membersCount <= 2 {
		if err = g.disband(ctx, realmID, group); err != nil {
			return fmt.Errorf("can't disband group, err: %w", err)
		}
	} else {
		eventToSend := events.GroupEventGroupMemberLeftPayload{
			ServiceID:     groupserver.ServiceID,
			RealmID:       realmID,
			GroupID:       groupID,
			MemberGUID:    targetMember.MemberGUID,
			MemberName:    targetMember.MemberName,
			NewLeaderID:   group.LeaderGUID,
			OnlineMembers: group.OnlineMemberGUIDs(),
		}
		if err = g.r.RemoveMember(ctx, realmID, target); err != nil {
			return fmt.Errorf("can't remove member, err: %w", err)
		}

		err = g.ep.GroupMemberLeft(&eventToSend)
		if err != nil {
			log.Error().Err(err).Msg("can't create GroupMemberLeft event")
		}
	}

	return nil
}

func (g groupServiceImpl) Leave(ctx context.Context, realmID uint32, player uint64) error {
	groupID, err := g.r.GroupIDByPlayer(ctx, realmID, player)
	if err != nil {
		return fmt.Errorf("can't get groupID, err: %w", err)
	}
	if groupID == 0 {
		return ErrGroupNotFound
	}

	group, err := g.r.GroupByID(ctx, realmID, groupID, true)
	if err != nil {
		return fmt.Errorf("can't get group, err: %w", err)
	}

	member := group.MemberByGUID(player)
	if member == nil {
		return ErrGroupNotFound
	}

	if len(group.Members) <= 2 {
		return g.disband(ctx, realmID, group)
	}

	if player == group.LeaderGUID {
		var newLeader uint64
		for _, groupMember := range group.Members {
			if !groupMember.IsOnline || groupMember.MemberGUID == player {
				continue
			}

			newLeader = groupMember.MemberGUID
			break
		}

		if err = g.changeLeader(ctx, realmID, group, newLeader, false); err != nil {
			return fmt.Errorf("can't change group leader, err: %w", err)
		}
	}

	eventToSend := events.GroupEventGroupMemberLeftPayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       realmID,
		GroupID:       groupID,
		MemberGUID:    member.MemberGUID,
		MemberName:    member.MemberName,
		NewLeaderID:   group.LeaderGUID,
		OnlineMembers: group.OnlineMemberGUIDs(),
	}

	if err = g.r.RemoveMember(ctx, realmID, player); err != nil {
		return fmt.Errorf("can't remove group member, err: %w", err)
	}

	err = g.ep.GroupMemberLeft(&eventToSend)
	if err != nil {
		log.Error().Err(err).Msg("can't create GroupMemberLeft event")
	}

	return nil
}

func (g groupServiceImpl) ChangeLeader(ctx context.Context, realmID uint32, player, newLeader uint64) error {
	group, err := g.getGroupWithLeader(ctx, realmID, player)
	if err != nil {
		return err
	}

	newLeaderMember := group.MemberByGUID(newLeader)
	if newLeaderMember == nil {
		return ErrGroupNotFound
	}

	return g.changeLeader(ctx, realmID, group, newLeader, true)
}

func (g groupServiceImpl) ConvertToRaid(ctx context.Context, realmID uint32, player uint64) error {
	group, err := g.getGroupWithLeader(ctx, realmID, player)
	if err != nil {
		return err
	}

	group.GroupType |= repo.GroupTypeFlagsRaid
	if err := g.r.Update(ctx, realmID, group); err != nil {
		return fmt.Errorf("can't update group win a new leader, err: %w", err)
	}
	err = g.ep.ConvertedToRaid(&events.GroupEventGroupConvertedToRaidPayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       realmID,
		GroupID:       group.ID,
		Leader:        group.LeaderGUID,
		OnlineMembers: group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't create ConvertedToRaid event")
	}

	return nil
}

func (g groupServiceImpl) SendMessage(ctx context.Context, realmID uint32, senderGUID uint64, message string, lang uint32, messageType MessageType) error {
	groupID, err := g.r.GroupIDByPlayer(ctx, realmID, senderGUID)
	if err != nil {
		return fmt.Errorf("can't get groupID, err: %w", err)
	}
	if groupID == 0 {
		return ErrGroupNotFound
	}

	group, err := g.r.GroupByID(ctx, realmID, groupID, true)
	if err != nil {
		return fmt.Errorf("can't get group, err: %w", err)
	}

	if group == nil {
		return ErrGroupNotFound
	}

	member := group.MemberByGUID(senderGUID)
	if member == nil {
		return ErrGroupMemberNotFound
	}

	isLeader := false
	switch messageType {
	case MessageTypeGroup, MessageTypeRaid:
		isLeader = false
	case MessageTypeGroupLeader, MessageTypeRaidLeader:
		isLeader = true
	default:
		return fmt.Errorf("message with type %d unsupported", messageType)
	}

	if isLeader && group.LeaderGUID != senderGUID {
		return ErrNoPermissions
	}

	err = g.ep.SendChatMessage(&events.GroupEventNewMessagePayload{
		ServiceID:   groupserver.ServiceID,
		RealmID:     realmID,
		GroupID:     group.ID,
		SenderGUID:  senderGUID,
		SenderName:  member.MemberName,
		Language:    lang,
		Msg:         message,
		MessageType: uint8(messageType),
		Receivers:   group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't create SendChatMessage event")
	}

	return nil
}

func (g groupServiceImpl) SetTargetIcon(ctx context.Context, realmID uint32, updaterGUID uint64, iconID uint8, targetGUID uint64) error {
	if repo.MaxTargetIcons <= iconID {
		return fmt.Errorf("iconID (%d) is invalid", iconID)
	}

	groupID, err := g.r.GroupIDByPlayer(ctx, realmID, updaterGUID)
	if err != nil {
		return fmt.Errorf("can't get groupID, err: %w", err)
	}
	if groupID == 0 {
		return ErrGroupNotFound
	}

	group, err := g.r.GroupByID(ctx, realmID, groupID, true)
	if err != nil {
		return fmt.Errorf("can't get group, err: %w", err)
	}

	if group == nil {
		return ErrGroupNotFound
	}

	groupMember := group.MemberByGUID(updaterGUID)
	if group.IsRaid() && group.LeaderGUID != updaterGUID && !groupMember.IsAssistant() {
		return ErrNoPermissions
	}

	for i, target := range group.TargetIcons {
		if target == targetGUID {
			group.TargetIcons[i] = 0
			break
		}
	}

	group.TargetIcons[iconID] = targetGUID

	if err = g.r.Update(ctx, realmID, group); err != nil {
		return fmt.Errorf("can't update icon for the group (%d), err: %w", groupID, err)
	}

	err = g.ep.TargetIconUpdated(&events.GroupEventNewTargetIconPayload{
		ServiceID: groupserver.ServiceID,
		RealmID:   realmID,
		GroupID:   group.ID,
		Updater:   updaterGUID,
		Target:    targetGUID,
		IconID:    iconID,
		Receivers: group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't create TargetIconUpdated event")
	}

	return nil
}

func (g groupServiceImpl) SetLootMethod(ctx context.Context, realmID uint32, updaterGUID uint64, method uint8, lootMaster uint64, lootThreshold uint8) error {
	group, err := g.getGroupWithLeader(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	group.LootMethod = method
	group.LootThreshold = lootThreshold
	group.LooterGUID = lootMaster

	if err = g.r.Update(ctx, realmID, group); err != nil {
		return err
	}

	err = g.ep.LootTypeChanged(&events.GroupEventGroupLootTypeChangedPayload{
		ServiceID:          groupserver.ServiceID,
		RealmID:            realmID,
		GroupID:            group.ID,
		NewLootType:        group.LootMethod,
		NewLooterGUID:      group.LooterGUID,
		NewLooterThreshold: group.LootThreshold,
		OnlineMembers:      group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't send loot changed event")
	}

	return nil
}

func (g groupServiceImpl) SetDungeonDifficulty(ctx context.Context, realmID uint32, updaterGUID uint64, difficulty uint8) error {
	group, err := g.getGroupWithLeader(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	group.Difficulty = difficulty

	if err = g.r.Update(ctx, realmID, group); err != nil {
		return err
	}

	err = g.ep.GroupDifficultyChanged(&events.GroupEventGroupDifficultyChangedPayload{
		ServiceID:         groupserver.ServiceID,
		RealmID:           realmID,
		GroupID:           group.ID,
		Updater:           updaterGUID,
		DungeonDifficulty: &difficulty,
		RaidDifficulty:    nil,
		Receivers:         group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't send difficulty changed event")
	}

	return nil
}

func (g groupServiceImpl) SetRaidDifficulty(ctx context.Context, realmID uint32, updaterGUID uint64, difficulty uint8) error {
	group, err := g.getGroupWithLeader(ctx, realmID, updaterGUID)
	if err != nil {
		return err
	}

	group.RaidDifficulty = difficulty

	if err = g.r.Update(ctx, realmID, group); err != nil {
		return err
	}

	err = g.ep.GroupDifficultyChanged(&events.GroupEventGroupDifficultyChangedPayload{
		ServiceID:         groupserver.ServiceID,
		RealmID:           realmID,
		GroupID:           group.ID,
		Updater:           updaterGUID,
		DungeonDifficulty: nil,
		RaidDifficulty:    &difficulty,
		Receivers:         group.OnlineMemberGUIDs(),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't send difficulty changed event")
	}

	return nil
}

func (g groupServiceImpl) HandleCharacterLoggedIn(payload events.LBEventCharacterLoggedInPayload) error {
	p, err := g.buildGroupMemberOnlineStatusChangedPayload(payload.RealmID, payload.CharGUID)
	if err != nil {
		return err
	}

	if p == nil {
		return nil
	}

	p.IsOnline = true
	return g.ep.GroupMemberOnlineStatusChanged(p)
}

func (g groupServiceImpl) HandleCharacterLoggedOut(payload events.LBEventCharacterLoggedOutPayload) error {
	p, err := g.buildGroupMemberOnlineStatusChangedPayload(payload.RealmID, payload.CharGUID)
	if err != nil {
		return err
	}

	if p == nil {
		return nil
	}

	p.IsOnline = false
	return g.ep.GroupMemberOnlineStatusChanged(p)
}

func (g groupServiceImpl) buildGroupMemberOnlineStatusChangedPayload(realmID uint32, player uint64) (*events.GroupEventGroupMemberOnlineStatusChangedPayload, error) {
	groupID, err := g.GroupIDByPlayer(context.Background(), realmID, player)
	if err != nil {
		return nil, err
	}

	if groupID == 0 {
		return nil, nil
	}

	group, err := g.GroupByID(context.Background(), realmID, groupID)
	if err != nil {
		return nil, err
	}

	return &events.GroupEventGroupMemberOnlineStatusChangedPayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       realmID,
		GroupID:       groupID,
		MemberGUID:    player,
		OnlineMembers: group.OnlineMemberGUIDs(),
	}, nil
}

func (g groupServiceImpl) getGroupWithLeader(ctx context.Context, realmID uint32, leaderGUID uint64) (*repo.Group, error) {
	groupID, err := g.r.GroupIDByPlayer(ctx, realmID, leaderGUID)
	if err != nil {
		return nil, fmt.Errorf("can't get groupID, err: %w", err)
	}
	if groupID == 0 {
		return nil, ErrGroupNotFound
	}

	group, err := g.r.GroupByID(ctx, realmID, groupID, true)
	if err != nil {
		return nil, fmt.Errorf("can't get group, err: %w", err)
	}

	if group == nil {
		return nil, ErrGroupNotFound
	}

	if group.LeaderGUID != leaderGUID {
		return nil, ErrNoPermissions
	}

	return group, nil
}

func (g groupServiceImpl) createGroup(ctx context.Context, realmID uint32, invite *repo.GroupInvite) error {
	group := repo.Group{
		LeaderGUID:       invite.Inviter,
		LootMethod:       uint8(repo.LootTypeFreeForAll),
		LooterGUID:       invite.Inviter,
		LootThreshold:    uint8(repo.ItemQualityUncommon),
		TargetIcons:      [8]uint64{},
		GroupType:        repo.GroupTypeFlagsNormal,
		Difficulty:       0,
		RaidDifficulty:   0,
		MasterLooterGuid: invite.Inviter,
		Members: []repo.GroupMember{
			{
				MemberGUID:  invite.Inviter,
				MemberFlags: 0,
				MemberName:  invite.InviterName,
				IsOnline:    true,
				SubGroup:    0,
				Roles:       0,
			},
			{
				MemberGUID:  invite.Invitee,
				MemberFlags: 0,
				MemberName:  invite.InviteeName,
				IsOnline:    true,
				SubGroup:    0,
				Roles:       0,
			},
		},
	}

	err := g.r.Create(ctx, realmID, &group)
	if err != nil {
		return err
	}

	members := make([]events.GroupMember, len(group.Members))
	for i, member := range group.Members {
		members[i].MemberGUID = member.MemberGUID
		members[i].MemberFlags = member.MemberFlags
		members[i].MemberName = member.MemberName
		members[i].SubGroup = member.SubGroup
		members[i].IsOnline = member.IsOnline
		members[i].Roles = uint8(member.Roles)
	}

	err = g.ep.GroupCreated(&events.GroupEventGroupCreatedPayload{
		ServiceID:        groupserver.ServiceID,
		RealmID:          realmID,
		GroupID:          group.ID,
		LeaderGUID:       group.LeaderGUID,
		LootMethod:       group.LootMethod,
		LooterGUID:       group.LooterGUID,
		LootThreshold:    group.LootThreshold,
		GroupType:        uint8(group.GroupType),
		Difficulty:       group.Difficulty,
		RaidDifficulty:   group.RaidDifficulty,
		MasterLooterGuid: group.MasterLooterGuid,
		Members:          members,
	})
	if err != nil {
		log.Error().Err(err).Msg("can't send group created event")
	}

	return nil
}

func (g groupServiceImpl) addMember(ctx context.Context, realmID uint32, group *repo.Group, invite *repo.GroupInvite) error {
	err := g.r.AddMember(ctx, realmID, &repo.GroupMember{
		GroupID:     invite.GroupID,
		MemberGUID:  invite.Invitee,
		MemberFlags: 0,
		MemberName:  invite.InviteeName,
		IsOnline:    true,
		SubGroup:    0,
		Roles:       0,
	})
	if err != nil {
		return err
	}

	err = g.ep.MemberAdded(&events.GroupEventGroupMemberAddedPayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       realmID,
		GroupID:       group.ID,
		MemberGUID:    invite.Invitee,
		MemberName:    invite.InviteeName,
		OnlineMembers: append(group.OnlineMemberGUIDs(), invite.Invitee),
	})
	if err != nil {
		log.Error().Err(err).Msg("can't send group member added event")
	}

	return nil
}

func (g groupServiceImpl) disband(ctx context.Context, realmID uint32, group *repo.Group) error {
	players := group.OnlineMemberGUIDs()
	err := g.r.Delete(ctx, realmID, group.ID)
	if err != nil {
		return fmt.Errorf("can't delete group, err: %w", err)
	}

	err = g.ep.GroupDisband(&events.GroupEventGroupDisbandPayload{
		ServiceID:     groupserver.ServiceID,
		RealmID:       realmID,
		GroupID:       group.ID,
		OnlineMembers: players,
	})
	if err != nil {
		log.Error().Err(err).Msg("can't create GroupDisband event")
	}

	return nil
}

func (g groupServiceImpl) changeLeader(ctx context.Context, realmID uint32, group *repo.Group, newLeader uint64, needsEventUpdate bool) error {
	prevLeader := group.LeaderGUID
	group.LeaderGUID = newLeader
	if err := g.r.Update(ctx, realmID, group); err != nil {
		return fmt.Errorf("can't update group win a new leader, err: %w", err)
	}
	if needsEventUpdate {
		err := g.ep.LeaderChanged(&events.GroupEventGroupLeaderChangedPayload{
			ServiceID:      groupserver.ServiceID,
			RealmID:        realmID,
			GroupID:        group.ID,
			PreviousLeader: prevLeader,
			NewLeader:      newLeader,
			OnlineMembers:  group.OnlineMemberGUIDs(),
		})
		if err != nil {
			log.Error().Err(err).Msg("can't create LeaderChanged event")
		}
	}

	return nil
}
