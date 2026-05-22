package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/walkline/ToCloud9/apps/groupserver/repo"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow/guid"
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

	// groupMemberHomeRealmCache stores the realm that owns the group row for a
	// member identity. The first index is the member's own realm, not the group
	// realm.
	groupMemberHomeRealmCache [MaxRealmID]map[uint64]uint32
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
	if err = validateCacheRealmID(realmID); err != nil {
		return nil, err
	}

	g.cacheLock.RLock()
	if g.groupsCache[realmID] != nil {
		r = cloneGroup(g.groupsCache[realmID][groupID])
	}
	g.cacheLock.RUnlock()
	return
}

func (g *groupsCacheInMem) GroupIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint, error) {
	memberRealmID, memberGUID, err := cacheMemberKey(realmID, player)
	if err != nil {
		return 0, err
	}

	g.cacheLock.RLock()
	defer g.cacheLock.RUnlock()

	if g.groupMembersCache[memberRealmID] == nil {
		return 0, nil
	}
	member := g.groupMembersCache[memberRealmID][memberGUID]
	if member == nil {
		return 0, nil
	}

	return member.GroupID, nil
}

func (g *groupsCacheInMem) GroupRealmIDByPlayer(ctx context.Context, realmID uint32, player uint64) (uint32, uint, error) {
	memberRealmID, memberGUID, err := cacheMemberKey(realmID, player)
	if err != nil {
		return 0, 0, err
	}

	g.cacheLock.RLock()
	defer g.cacheLock.RUnlock()

	member := (*repo.GroupMember)(nil)
	if g.groupMembersCache[memberRealmID] != nil {
		member = g.groupMembersCache[memberRealmID][memberGUID]
	}
	if member == nil {
		return 0, 0, nil
	}

	groupRealmID := realmID
	if g.groupMemberHomeRealmCache[memberRealmID] != nil {
		if cachedRealmID, ok := g.groupMemberHomeRealmCache[memberRealmID][memberGUID]; ok {
			groupRealmID = cachedRealmID
		}
	}

	return groupRealmID, member.GroupID, nil
}

func (g *groupsCacheInMem) Create(ctx context.Context, realmID uint32, group *repo.Group) error {
	if err := validateCacheRealmID(realmID); err != nil {
		return err
	}

	err := g.r.Create(ctx, realmID, group)
	if err != nil {
		return err
	}

	g.cacheLock.Lock()
	g.ensureRealmCacheLocked(realmID)
	cachedGroup := cloneGroup(group)
	normalizeGroupForRealm(cachedGroup, realmID)
	g.groupsCache[realmID][cachedGroup.ID] = cachedGroup
	g.rebuildGroupMembersCacheLocked(realmID, cachedGroup)
	g.cacheLock.Unlock()

	return nil
}

func (g *groupsCacheInMem) Delete(ctx context.Context, realmID uint32, groupID uint) error {
	if err := validateCacheRealmID(realmID); err != nil {
		return err
	}

	if err := g.r.Delete(ctx, realmID, groupID); err != nil {
		return err
	}

	g.cacheLock.Lock()
	defer g.cacheLock.Unlock()

	if g.groupsCache[realmID] == nil {
		return nil
	}
	group := g.groupsCache[realmID][groupID]
	if group == nil {
		return nil
	}

	g.ensureRealmCacheLocked(realmID)

	g.clearGroupMembersCacheLocked(realmID, group)

	delete(g.groupsCache[realmID], groupID)

	return nil
}

func (g *groupsCacheInMem) Update(ctx context.Context, realmID uint32, group *repo.Group) error {
	if err := validateCacheRealmID(realmID); err != nil {
		return err
	}

	if err := g.r.Update(ctx, realmID, group); err != nil {
		return err
	}

	g.cacheLock.Lock()
	g.ensureRealmCacheLocked(realmID)
	cachedGroup := cloneGroup(group)
	normalizeGroupForRealm(cachedGroup, realmID)
	g.groupsCache[realmID][cachedGroup.ID] = cachedGroup
	g.rebuildGroupMembersCacheLocked(realmID, cachedGroup)
	g.cacheLock.Unlock()

	return nil
}

func (g *groupsCacheInMem) RegisterMaterializedLfgGroup(_ context.Context, realmID uint32, group *repo.Group) error {
	if err := validateCacheRealmID(realmID); err != nil {
		return err
	}
	if group == nil {
		return nil
	}

	g.cacheLock.Lock()
	defer g.cacheLock.Unlock()

	g.ensureRealmCacheLocked(realmID)
	cachedGroup := cloneGroup(group)
	normalizeGroupForRealm(cachedGroup, realmID)
	if existing := g.groupsCache[realmID][cachedGroup.ID]; existing != nil {
		g.clearGroupMembersCacheLocked(realmID, existing)
	}
	g.groupsCache[realmID][cachedGroup.ID] = cachedGroup
	g.rebuildGroupMembersCacheLocked(realmID, cachedGroup)

	return nil
}

func (g *groupsCacheInMem) RegisterAcceptedLfgGroup(ctx context.Context, realmID uint32, group *repo.Group) error {
	if err := validateCacheRealmID(realmID); err != nil {
		return err
	}
	if group == nil {
		return nil
	}

	if registrar, ok := g.r.(acceptedLfgGroupRegistrar); ok {
		if err := registrar.RegisterAcceptedLfgGroup(ctx, realmID, group); err != nil {
			return err
		}
	} else if group.ID == 0 {
		if err := g.r.Create(ctx, realmID, group); err != nil {
			return err
		}
	} else if err := g.r.Update(ctx, realmID, group); err != nil {
		return err
	}

	g.cacheLock.Lock()
	defer g.cacheLock.Unlock()

	g.ensureRealmCacheLocked(realmID)
	cachedGroup := cloneGroup(group)
	normalizeGroupForRealm(cachedGroup, realmID)
	if existing := g.groupsCache[realmID][cachedGroup.ID]; existing != nil {
		g.clearGroupMembersCacheLocked(realmID, existing)
	}
	g.groupsCache[realmID][cachedGroup.ID] = cachedGroup
	g.rebuildGroupMembersCacheLocked(realmID, cachedGroup)

	return nil
}

func (g *groupsCacheInMem) AddMember(ctx context.Context, realmID uint32, groupMember *repo.GroupMember) error {
	if err := validateCacheRealmID(realmID); err != nil {
		return err
	}

	g.cacheLock.RLock()
	group := g.groupsCache[realmID][groupMember.GroupID]
	g.cacheLock.RUnlock()
	if group == nil {
		return fmt.Errorf("group %d not found in realm %d cache", groupMember.GroupID, realmID)
	}

	if err := g.r.AddMember(ctx, realmID, groupMember); err != nil {
		return err
	}

	g.cacheLock.Lock()
	g.ensureRealmCacheLocked(realmID)
	group = g.groupsCache[realmID][groupMember.GroupID]
	if group == nil {
		g.cacheLock.Unlock()
		return fmt.Errorf("group %d not found in realm %d cache", groupMember.GroupID, realmID)
	}
	normalizeGroupMemberForRealm(groupMember, realmID)
	group.Members = append(group.Members, *groupMember)
	g.rebuildGroupMembersCacheLocked(realmID, group)
	g.cacheLock.Unlock()

	return nil
}

func (g *groupsCacheInMem) UpdateMember(ctx context.Context, realmID uint32, groupMember *repo.GroupMember) error {
	if err := validateCacheRealmID(realmID); err != nil {
		return err
	}

	if err := g.r.UpdateMember(ctx, realmID, groupMember); err != nil {
		return err
	}

	g.cacheLock.Lock()
	defer g.cacheLock.Unlock()

	g.ensureRealmCacheLocked(realmID)
	group := g.groupsCache[realmID][groupMember.GroupID]
	if group == nil {
		return fmt.Errorf("group %d not found in realm %d cache", groupMember.GroupID, realmID)
	}

	normalizeGroupMemberForRealm(groupMember, realmID)
	for i, member := range group.Members {
		if guid.SamePlayer(realmID, member.MemberGUID, realmID, groupMember.MemberGUID) {
			group.Members[i] = *groupMember
			g.rebuildGroupMembersCacheLocked(realmID, group)
			break
		}
	}

	return nil
}

func (g *groupsCacheInMem) RemoveMember(ctx context.Context, realmID uint32, memberGUID uint64) error {
	memberRealmID, memberLowGUID, err := cacheMemberKey(realmID, memberGUID)
	if err != nil {
		return err
	}

	groupID, err := g.GroupIDByPlayer(ctx, realmID, memberGUID)
	if err != nil {
		return err
	}

	groupRealmID := realmID
	g.cacheLock.RLock()
	if g.groupMemberHomeRealmCache[memberRealmID] != nil {
		if cachedRealmID, ok := g.groupMemberHomeRealmCache[memberRealmID][memberLowGUID]; ok {
			groupRealmID = cachedRealmID
		}
	}
	g.cacheLock.RUnlock()

	if err := g.r.RemoveMember(ctx, groupRealmID, guid.PlayerGUIDForRealm(groupRealmID, memberRealmID, memberLowGUID)); err != nil {
		return err
	}

	g.cacheLock.Lock()
	defer g.cacheLock.Unlock()

	group := (*repo.Group)(nil)
	if g.groupsCache[groupRealmID] != nil {
		group = g.groupsCache[groupRealmID][groupID]
	}
	if group == nil {
		return nil
	}

	for i, member := range group.Members {
		if guid.SamePlayer(realmID, memberGUID, groupRealmID, member.MemberGUID) {
			group.Members = append(group.Members[:i], group.Members[i+1:]...)
			g.rebuildGroupMembersCacheLocked(groupRealmID, group)
			break
		}
	}

	return nil
}

func (g *groupsCacheInMem) AddInvite(ctx context.Context, realmID uint32, invite repo.GroupInvite) error {
	return g.r.AddInvite(ctx, realmID, invite)
}

func (g *groupsCacheInMem) GetInviteByInvitedPlayer(ctx context.Context, realmID uint32, invitedPlayer uint64) (*repo.GroupInvite, error) {
	return g.r.GetInviteByInvitedPlayer(ctx, realmID, invitedPlayer)
}

func (g *groupsCacheInMem) RemoveInvite(ctx context.Context, realmID uint32, invitedPlayer uint64) error {
	return g.r.RemoveInvite(ctx, realmID, invitedPlayer)
}

func (g *groupsCacheInMem) HandleCharacterLoggedIn(payload events.GWEventCharacterLoggedInPayload) error {
	_, err := g.SetMemberOnlineStatus(context.Background(), payload.RealmID, payload.CharGUID, true)
	return err
}

func (g *groupsCacheInMem) HandleCharacterLoggedOut(payload events.GWEventCharacterLoggedOutPayload) error {
	_, err := g.SetMemberOnlineStatus(context.Background(), payload.RealmID, payload.CharGUID, false)
	return err
}

func (g *groupsCacheInMem) SetMemberOnlineStatus(ctx context.Context, realmID uint32, memberGUID uint64, online bool) (*repo.Group, error) {
	memberRealmID, memberLowGUID, err := cacheMemberKey(realmID, memberGUID)
	if err != nil {
		return nil, err
	}

	g.cacheLock.Lock()
	defer g.cacheLock.Unlock()

	if g.groupMembersCache[memberRealmID] == nil {
		return nil, nil
	}

	member := g.groupMembersCache[memberRealmID][memberLowGUID]
	if member == nil {
		return nil, nil
	}

	groupRealmID := realmID
	if g.groupMemberHomeRealmCache[memberRealmID] != nil {
		if cachedRealmID, ok := g.groupMemberHomeRealmCache[memberRealmID][memberLowGUID]; ok {
			groupRealmID = cachedRealmID
		}
	}

	group := g.groupsCache[groupRealmID][member.GroupID]
	if group == nil {
		return nil, nil
	}

	for i := range group.Members {
		if guid.SamePlayer(realmID, memberGUID, groupRealmID, group.Members[i].MemberGUID) {
			group.Members[i].IsOnline = online
			break
		}
	}
	g.rebuildGroupMembersCacheLocked(groupRealmID, group)

	return cloneGroup(group), nil
}

func (g *groupsCacheInMem) Warmup(ctx context.Context, realmID uint32) error {
	if err := validateCacheRealmID(realmID); err != nil {
		return err
	}
	groups, err := g.r.LoadAllForRealm(ctx, realmID)
	if err != nil {
		return err
	}
	if groups == nil {
		groups = map[uint]*repo.Group{}
	}

	cachedGroups := make(map[uint]*repo.Group, len(groups))
	for groupID, group := range groups {
		cachedGroup := cloneGroup(group)
		if cachedGroup == nil {
			continue
		}
		normalizeGroupForRealm(cachedGroup, realmID)
		// Online state is a session lease, not persistent group state. Gateway
		// login/member-state events will re-establish live members after warmup.
		for i := range cachedGroup.Members {
			cachedGroup.Members[i].IsOnline = false
		}
		cachedGroups[groupID] = cachedGroup
	}

	g.cacheLock.Lock()
	g.clearRealmOwnedMembersCacheLocked(realmID)
	g.groupsCache[realmID] = cachedGroups

	g.groupMembersCache[realmID] = map[uint64]*repo.GroupMember{}
	g.groupMemberHomeRealmCache[realmID] = map[uint64]uint32{}
	for _, group := range cachedGroups {
		g.rebuildGroupMembersCacheLocked(realmID, group)
	}
	g.cacheLock.Unlock()

	return nil
}

func validateCacheRealmID(realmID uint32) error {
	if realmID >= MaxRealmID {
		return fmt.Errorf("realmID overflow, %d >= %d", realmID, MaxRealmID)
	}

	return nil
}

func (g *groupsCacheInMem) rebuildGroupMembersCacheLocked(realmID uint32, group *repo.Group) {
	g.ensureRealmCacheLocked(realmID)
	if group == nil {
		return
	}

	normalizeGroupForRealm(group, realmID)
	g.clearGroupMembersCacheLocked(realmID, group)
	for i := range group.Members {
		memberRealmID, memberLowGUID, err := cacheMemberKey(realmID, group.Members[i].MemberGUID)
		if err != nil {
			continue
		}
		g.ensureMemberRealmCacheLocked(memberRealmID)
		g.groupMembersCache[memberRealmID][memberLowGUID] = &group.Members[i]
		g.groupMemberHomeRealmCache[memberRealmID][memberLowGUID] = realmID
	}
}

func (g *groupsCacheInMem) ensureRealmCacheLocked(realmID uint32) {
	if g.groupsCache[realmID] == nil {
		g.groupsCache[realmID] = map[uint]*repo.Group{}
	}
	g.ensureMemberRealmCacheLocked(realmID)
}

func (g *groupsCacheInMem) ensureMemberRealmCacheLocked(realmID uint32) {
	if g.groupMembersCache[realmID] == nil {
		g.groupMembersCache[realmID] = map[uint64]*repo.GroupMember{}
	}
	if g.groupMemberHomeRealmCache[realmID] == nil {
		g.groupMemberHomeRealmCache[realmID] = map[uint64]uint32{}
	}
}

func (g *groupsCacheInMem) clearGroupMembersCacheLocked(groupRealmID uint32, group *repo.Group) {
	if group == nil {
		return
	}
	for memberRealmID := uint32(0); memberRealmID < MaxRealmID; memberRealmID++ {
		for memberGUID, member := range g.groupMembersCache[memberRealmID] {
			if member == nil || member.GroupID != group.ID {
				continue
			}
			if g.groupMemberHomeRealmCache[memberRealmID] != nil && g.groupMemberHomeRealmCache[memberRealmID][memberGUID] != groupRealmID {
				continue
			}
			delete(g.groupMembersCache[memberRealmID], memberGUID)
			delete(g.groupMemberHomeRealmCache[memberRealmID], memberGUID)
		}
	}
}

func (g *groupsCacheInMem) clearRealmOwnedMembersCacheLocked(groupRealmID uint32) {
	for memberRealmID := uint32(0); memberRealmID < MaxRealmID; memberRealmID++ {
		for memberGUID := range g.groupMembersCache[memberRealmID] {
			if g.groupMemberHomeRealmCache[memberRealmID] == nil || g.groupMemberHomeRealmCache[memberRealmID][memberGUID] != groupRealmID {
				continue
			}
			delete(g.groupMembersCache[memberRealmID], memberGUID)
			delete(g.groupMemberHomeRealmCache[memberRealmID], memberGUID)
		}
	}
}

func cloneGroup(group *repo.Group) *repo.Group {
	if group == nil {
		return nil
	}

	clone := *group
	if group.Members != nil {
		clone.Members = append([]repo.GroupMember(nil), group.Members...)
	}
	return &clone
}

func cacheMemberKey(defaultRealmID uint32, playerGUID uint64) (uint32, uint64, error) {
	memberRealmID := guid.PlayerRealmIDOrDefault(defaultRealmID, playerGUID)
	if err := validateCacheRealmID(memberRealmID); err != nil {
		return 0, 0, err
	}
	return memberRealmID, guid.PlayerLowGUID(playerGUID), nil
}

func normalizeGroupForRealm(group *repo.Group, realmID uint32) {
	if group == nil {
		return
	}
	group.RealmID = realmID
	group.LeaderGUID = guid.NormalizePlayerGUIDForRealm(realmID, group.LeaderGUID)
	group.LooterGUID = guid.NormalizePlayerGUIDForRealm(realmID, group.LooterGUID)
	group.MasterLooterGuid = guid.NormalizePlayerGUIDForRealm(realmID, group.MasterLooterGuid)
	for i := range group.TargetIcons {
		group.TargetIcons[i] = guid.NormalizePlayerGUIDForRealm(realmID, group.TargetIcons[i])
	}
	for i := range group.Members {
		normalizeGroupMemberForRealm(&group.Members[i], realmID)
	}
}

func normalizeGroupMemberForRealm(groupMember *repo.GroupMember, groupRealmID uint32) {
	if groupMember == nil {
		return
	}
	groupMember.RealmID = guid.PlayerRealmIDOrDefault(groupRealmID, groupMember.MemberGUID)
	groupMember.MemberGUID = guid.PlayerGUIDForRealm(groupRealmID, groupMember.RealmID, groupMember.MemberGUID)
}
