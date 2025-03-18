package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/walkline/ToCloud9/apps/groupserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

const MaxRealmID = 256

// groupsCacheInMem is in mem cached proxy of groups repo.
type groupsCacheInMem struct {
	r repo.GroupsRepo

	cacheLock sync.RWMutex

	// groupsCache in mem groups cache. Usage example: groupsCache[realmID][guildID].
	groupsCache [MaxRealmID]map[uint]*repo.Group

	// groupMembersCache in mem group members cache. Usage example: groupMembersCache[realmID][playerGUID].
	// Holds reference to the member object inside groupsCache,
	// so modifying object here modifies it in groupsCache as well.
	groupMembersCache [MaxRealmID]map[uint64]*repo.GroupMember
}

// NewInMemGroupsCache creates in memory groups cache.
func NewInMemGroupsCache(r repo.GroupsRepo) GroupsCache {
	return &groupsCacheInMem{
		r: r,
	}
}

func (g *groupsCacheInMem) LoadAllForRealm(ctx context.Context, realmID uint32) (map[uint]*repo.Group, error) {
	return g.r.LoadAllForRealm(ctx, realmID)
}

func (g *groupsCacheInMem) GroupByID(ctx context.Context, realmID uint32, groupID uint, loadMembers bool) (r *repo.Group, err error) {
	g.cacheLock.RLock()
	r = g.groupsCache[realmID][groupID]
	g.cacheLock.RUnlock()
	return
}

func (g *groupsCacheInMem) GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error) {
	member := g.groupMemberByGUID(realmID, player)
	if member == nil {
		return 0, nil
	}
	return member.GroupID, nil
}

func (g *groupsCacheInMem) Create(ctx context.Context, realmID uint32, group *repo.Group) error {
	err := g.r.Create(ctx, realmID, group)
	if err != nil {
		return err
	}

	g.cacheLock.Lock()
	g.groupsCache[realmID][group.ID] = group

	for i, member := range group.Members {
		g.groupMembersCache[realmID][member.MemberGUID] = &group.Members[i]
	}

	g.cacheLock.Unlock()

	return nil
}

func (g *groupsCacheInMem) Delete(ctx context.Context, realmID uint32, groupID uint) error {
	if err := g.r.Delete(ctx, realmID, groupID); err != nil {
		return err
	}

	group, err := g.GroupByID(ctx, realmID, groupID, true)
	if err != nil {
		return err
	}

	g.cacheLock.Lock()

	for _, member := range group.Members {
		delete(g.groupMembersCache[realmID], member.MemberGUID)
	}

	delete(g.groupsCache[realmID], groupID)

	g.cacheLock.Unlock()

	return nil
}

func (g *groupsCacheInMem) Update(ctx context.Context, realmID uint32, group *repo.Group) error {
	if err := g.r.Update(ctx, realmID, group); err != nil {
		return err
	}

	g.cacheLock.Lock()
	g.groupsCache[realmID][group.ID] = group
	g.cacheLock.Unlock()

	return nil
}

func (g *groupsCacheInMem) AddMember(ctx context.Context, realmID uint32, groupMember *repo.GroupMember) error {
	if err := g.r.AddMember(ctx, realmID, groupMember); err != nil {
		return err
	}

	g.cacheLock.Lock()
	g.groupsCache[realmID][groupMember.GroupID].Members = append(g.groupsCache[realmID][groupMember.GroupID].Members, *groupMember)
	g.groupMembersCache[realmID][groupMember.MemberGUID] = &g.groupsCache[realmID][groupMember.GroupID].Members[len(g.groupsCache[realmID][groupMember.GroupID].Members)-1]
	g.cacheLock.Unlock()

	return nil
}

func (g *groupsCacheInMem) UpdateMember(ctx context.Context, realmID uint32, groupMember *repo.GroupMember) error {
	if err := g.r.UpdateMember(ctx, realmID, groupMember); err != nil {
		return err
	}

	group, err := g.GroupByID(ctx, realmID, groupMember.GroupID, true)
	if err != nil {
		return err
	}

	g.cacheLock.Lock()
	for i, member := range group.Members {
		if member.MemberGUID == groupMember.MemberGUID {
			group.Members[i] = *groupMember
			g.groupMembersCache[realmID][groupMember.MemberGUID] = &group.Members[i]
			break
		}
	}
	g.cacheLock.Unlock()

	return nil
}

func (g *groupsCacheInMem) RemoveMember(ctx context.Context, realmID uint32, memberGUID uint64) error {
	if err := g.r.RemoveMember(ctx, realmID, memberGUID); err != nil {
		return err
	}

	groupID, err := g.GroupIDByPlayer(ctx, realmID, memberGUID)
	if err != nil {
		return err
	}

	group, err := g.GroupByID(ctx, realmID, groupID, true)
	if err != nil {
		return err
	}

	g.cacheLock.Lock()
	for i, member := range group.Members {
		if member.MemberGUID == memberGUID {
			group.Members = append(group.Members[:i], group.Members[i+1:]...)
			delete(g.groupMembersCache[realmID], member.MemberGUID)
			break
		}
	}
	g.cacheLock.Unlock()

	return nil
}

func (g *groupsCacheInMem) AddInvite(ctx context.Context, realmID uint32, invite repo.GroupInvite) error {
	return g.r.AddInvite(ctx, realmID, invite)
}

func (g *groupsCacheInMem) GetInviteByInvitedPlayer(ctx context.Context, realmID uint32, invitedPlayer uint64) (*repo.GroupInvite, error) {
	return g.r.GetInviteByInvitedPlayer(ctx, realmID, invitedPlayer)
}

func (g *groupsCacheInMem) HandleCharacterLoggedIn(payload events.GWEventCharacterLoggedInPayload) error {
	member := g.groupMemberByGUID(payload.RealmID, payload.CharGUID)
	if member == nil {
		return nil
	}

	member.IsOnline = true

	return nil
}

func (g *groupsCacheInMem) HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error {
	member := g.groupMemberByGUID(payload.RealmID, payload.CharGUID)
	if member == nil {
		return nil
	}

	member.IsOnline = false

	return nil
}

func (g *groupsCacheInMem) Warmup(ctx context.Context, realmID uint32) error {
	if realmID > MaxRealmID {
		panic(fmt.Errorf("realmID overflow, %d > %d", realmID, MaxRealmID))
	}
	groups, err := g.r.LoadAllForRealm(ctx, realmID)
	if err != nil {
		return err
	}

	g.groupsCache[realmID] = groups

	g.groupMembersCache[realmID] = map[uint64]*repo.GroupMember{}
	for _, group := range groups {
		for i := range group.Members {
			g.groupMembersCache[realmID][group.Members[i].MemberGUID] = &group.Members[i]
		}
	}

	return nil
}

func (g *groupsCacheInMem) groupMemberByGUID(realmID uint32, player uint64) (r *repo.GroupMember) {
	g.cacheLock.RLock()
	r = g.groupMembersCache[realmID][player]
	g.cacheLock.RUnlock()
	return
}
