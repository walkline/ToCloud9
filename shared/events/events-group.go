package events

import "fmt"

// GroupServiceEvent is event type that group service generates
type GroupServiceEvent int

const (
	// GroupEventInviteCreated group event when player invite created
	GroupEventInviteCreated GroupServiceEvent = iota + 1

	// GroupEventGroupCreated group event when group created
	GroupEventGroupCreated

	// GroupEventGroupMemberOnlineStatusChanged group event when group member cam online or offline
	GroupEventGroupMemberOnlineStatusChanged

	// GroupEventGroupMemberLeft group event when group member leaves
	GroupEventGroupMemberLeft

	// GroupEventGroupDisband group event when group is disbanded
	GroupEventGroupDisband

	// GroupEventGroupMemberAdded group event when a member is added to the group
	GroupEventGroupMemberAdded

	// GroupEventGroupLeaderChanged group event when the group leader is changed
	GroupEventGroupLeaderChanged

	// GroupEventGroupLootTypeChanged group event when the loot type is changed
	GroupEventGroupLootTypeChanged

	// GroupEventGroupConvertedToRaid group event when the group is converted to a raid
	GroupEventGroupConvertedToRaid

	// GroupEventNewChatMessage group event when somebody sends group or raid chat message
	GroupEventNewChatMessage

	// GroupEventNewTargetIcon group event when leader or assistant sets target icon for raid
	GroupEventNewTargetIcon

	// GroupEventGroupDifficultyChanged group event when dungeon or raid difficulty changed for the group
	GroupEventGroupDifficultyChanged

	GroupEventGroupReadyCheckStarted
	GroupEventGroupReadyCheckMemberState
	GroupEventGroupReadyCheckFinished
	GroupEventGroupMemberSubGroupChanged
	GroupEventGroupMemberFlagsChanged
	GroupEventGroupMemberStateChanged
	GroupEventGroupInstanceResetRequest
	GroupEventGroupInstanceBindExtensionRequest

	// GroupEventInviteDeclined group event when player invite declined
	GroupEventInviteDeclined

	GroupEventGroupMemberStatesChanged
)

// SubjectName is key that nats uses
func (e GroupServiceEvent) SubjectName() string {
	switch e {
	case GroupEventInviteCreated:
		return "group.invite.created"
	case GroupEventInviteDeclined:
		return "group.invite.declined"
	case GroupEventGroupCreated:
		return "group.created"
	case GroupEventGroupMemberOnlineStatusChanged:
		return "group.member.online.changed"
	case GroupEventGroupMemberLeft:
		return "group.member.left"
	case GroupEventGroupDisband:
		return "group.disband"
	case GroupEventGroupMemberAdded:
		return "group.member.added"
	case GroupEventGroupLeaderChanged:
		return "group.leader.changed"
	case GroupEventGroupLootTypeChanged:
		return "group.loot.changed"
	case GroupEventGroupConvertedToRaid:
		return "group.converted.raid"
	case GroupEventNewChatMessage:
		return "group.message.new"
	case GroupEventNewTargetIcon:
		return "group.targeticons.new"
	case GroupEventGroupDifficultyChanged:
		return "group.difficulty.changed"
	case GroupEventGroupReadyCheckStarted:
		return "group.readycheck.started"
	case GroupEventGroupReadyCheckMemberState:
		return "group.readycheck.member.state"
	case GroupEventGroupReadyCheckFinished:
		return "group.readycheck.finished"
	case GroupEventGroupMemberSubGroupChanged:
		return "group.member.subgroup.changed"
	case GroupEventGroupMemberFlagsChanged:
		return "group.member.flags.changed"
	case GroupEventGroupMemberStateChanged:
		return "group.member.state.changed"
	case GroupEventGroupMemberStatesChanged:
		return "group.member.states.changed"
	case GroupEventGroupInstanceResetRequest:
		return "group.instance.reset.request"
	case GroupEventGroupInstanceBindExtensionRequest:
		return "group.instance.bind.extension.request"
	}
	panic(fmt.Errorf("unk event %d", e))
}

type GroupEventInviteCreatedPayload struct {
	// ServiceID is identifier of guild service
	ServiceID string
	RealmID   uint32

	GroupID uint

	InviterGUID uint64
	InviterName string

	InviteeGUID uint64
	InviteeName string
}

type GroupEventInviteDeclinedPayload struct {
	ServiceID string
	RealmID   uint32

	InviterGUID uint64
	InviteeGUID uint64
	InviteeName string
}

type GroupEventGroupCreatedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID          uint
	LeaderGUID       uint64
	LootMethod       uint8
	LooterGUID       uint64
	LootThreshold    uint8
	GroupType        uint8
	Difficulty       uint8
	RaidDifficulty   uint8
	MasterLooterGuid uint64
	LfgDungeonEntry  uint32
	Members          []GroupMember
}

type GroupEventGroupMemberOnlineStatusChangedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID    uint
	MemberGUID uint64
	IsOnline   bool

	OnlineMembers []uint64
}

type GroupEventGroupMemberLeftPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID     uint
	MemberGUID  uint64
	MemberName  string
	NewLeaderID uint64 // If the leaving member was the leader, specify the new leader ID

	OnlineMembers []uint64
}

type GroupEventGroupMemberAddedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID    uint
	MemberGUID uint64
	MemberName string

	OnlineMembers []uint64
}

type GroupEventGroupLeaderChangedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID        uint
	PreviousLeader uint64
	NewLeader      uint64

	OnlineMembers []uint64
}

type GroupEventGroupLootTypeChangedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID uint

	NewLootType        uint8
	NewLooterGUID      uint64
	NewLooterThreshold uint8

	OnlineMembers []uint64
}

type GroupEventGroupConvertedToRaidPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID uint
	Leader  uint64

	OnlineMembers []uint64
}

type GroupEventGroupDisbandPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID       uint
	OnlineMembers []uint64
}

type GroupEventNewMessagePayload struct {
	ServiceID string
	RealmID   uint32

	GroupID uint

	SenderGUID    uint64
	SenderName    string
	SenderChatTag uint8

	Language uint32
	Msg      string

	MessageType uint8

	Receivers []uint64
}

type GroupEventNewTargetIconPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID uint
	Updater uint64
	Target  uint64
	IconID  uint8

	Receivers []uint64
}

type GroupEventGroupDifficultyChangedPayload struct {
	ServiceID string
	RealmID   uint32

	GroupID uint
	Updater uint64

	DungeonDifficulty *uint8 `json:"DungeonDifficulty,omitempty"`
	RaidDifficulty    *uint8 `json:"RaidDifficulty,omitempty"`

	Receivers []uint64
}

type GroupMember struct {
	MemberGUID  uint64
	MemberFlags uint8
	MemberName  string
	IsOnline    bool
	SubGroup    uint8
	Roles       uint8
}

type GroupEventReadyCheckStartedPayload struct {
	ServiceID  string
	RealmID    uint32
	GroupID    uint
	LeaderGUID uint64
	DurationMs uint32
	Receivers  []uint64
}

type GroupEventReadyCheckMemberStatePayload struct {
	ServiceID  string
	RealmID    uint32
	GroupID    uint
	MemberGUID uint64
	State      uint8 // 0 waiting, 1 ready, 2 not ready
	Receivers  []uint64
}

type GroupEventReadyCheckFinishedPayload struct {
	ServiceID string
	RealmID   uint32
	GroupID   uint
	Receivers []uint64
}

type GroupEventMemberSubGroupChangedPayload struct {
	ServiceID  string
	RealmID    uint32
	GroupID    uint
	MemberGUID uint64
	SubGroup   uint8
	Receivers  []uint64
}

type GroupEventMemberFlagsChangedPayload struct {
	ServiceID  string
	RealmID    uint32
	GroupID    uint
	MemberGUID uint64
	Flags      uint8
	Roles      uint8
	Receivers  []uint64
}

type GroupEventMemberStateChangedPayload struct {
	ServiceID           string
	RealmID             uint32
	GroupID             uint
	SourceGatewayID     string
	SourceWorldserverID string
	MemberGUID          uint64
	Online              bool
	Level               uint8
	Class               uint8
	ZoneID              uint32
	MapID               uint32
	Health              uint32
	MaxHealth           uint32
	PowerType           uint8
	Power               uint32
	MaxPower            uint32
	AurasKnown          bool
	Auras               []GroupMemberAuraState
	DeadKnown           bool
	Dead                bool
	GhostKnown          bool
	Ghost               bool
	Receivers           []uint64
}

type GroupEventMemberStatesChangedPayload struct {
	ServiceID           string
	RealmID             uint32
	GroupID             uint
	SourceGatewayID     string
	SourceWorldserverID string
	States              []GroupMemberStateUpdate
	Receivers           []uint64
}

type GroupMemberStateUpdate struct {
	MemberGUID uint64
	Online     bool
	Level      uint8
	Class      uint8
	ZoneID     uint32
	MapID      uint32
	Health     uint32
	MaxHealth  uint32
	PowerType  uint8
	Power      uint32
	MaxPower   uint32
	AurasKnown bool
	Auras      []GroupMemberAuraState
	DeadKnown  bool
	Dead       bool
	GhostKnown bool
	Ghost      bool
}

type GroupMemberAuraState struct {
	Slot    uint8
	SpellID uint32
	Flags   uint8
}

type GroupEventInstanceResetRequestPayload struct {
	ServiceID  string
	RealmID    uint32
	GroupID    uint
	PlayerGUID uint64
	MapID      uint32
	Difficulty uint8
	Receivers  []uint64
}

type GroupEventInstanceBindExtensionRequestPayload struct {
	ServiceID  string
	RealmID    uint32
	GroupID    uint
	PlayerGUID uint64
	MapID      uint32
	Difficulty uint8
	Extended   bool
	Receivers  []uint64
}
