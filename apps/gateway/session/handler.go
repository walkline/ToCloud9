package session

import (
	"context"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

// OpcodeBlacklist contains opcodes that should be dropped (not forwarded to client)
// Used to block worldserver friend system packets since we handle friends at gateway level
// Also blocks channel packets since we handle channels at gateway level in distributed fashion
var OpcodeBlacklist = map[packet.Opcode]bool{
	packet.SMsgFriendStatus:         true,
	packet.SMsgContactList:          true,
	packet.TC9SMsgWorldSessionReady: true,
}

var HandleMap = map[packet.Opcode]HandlersQueue{
	packet.CMsgCharCreate:               NewHandler("CMsgCharCreate", (*GameSession).CreateCharacter),
	packet.CMsgPlayerLogin:              NewHandler("CMsgPlayerLogin", (*GameSession).Login),
	packet.CMsgCharDelete:               NewHandler("CMsgCharDelete", (*GameSession).DeleteCharacter),
	packet.CMsgCharEnum:                 NewHandler("CMsgCharEnum", (*GameSession).CharactersList),
	packet.CMsgRealmSplit:               NewHandler("CMsgRealmSplit", (*GameSession).RealmSplit),
	packet.CMsgReadyForAccountDataTimes: NewHandler("CMsgReadyForAccountDataTimes", (*GameSession).ReadyForAccountDataTimes),
	packet.CMsgRequestAccountData:       NewHandler("CMsgRequestAccountData", (*GameSession).RequestAccountData),
	packet.CMsgUpdateAccountData:        NewHandler("CMsgUpdateAccountData", (*GameSession).UpdateAccountData),
	packet.CMsgMessageChat:              NewHandler("CMsgMessageChat", (*GameSession).HandleChatMessage),
	packet.CMsgNameQuery:                NewHandler("CMsgNameQuery", (*GameSession).HandleNameQuery),
	packet.CMsgGuildQuery:               NewHandler("CMsgGuildQuery", (*GameSession).HandleGuildQuery),
	packet.CMsgWho:                      NewHandler("CMsgWho", (*GameSession).HandleWho),

	// Friends
	packet.CMsgContactList:     NewHandler("CMsgContactList", (*GameSession).HandleContactList),
	packet.CMsgAddFriend:       NewHandler("CMsgAddFriend", (*GameSession).HandleAddFriend),
	packet.CMsgDelFriend:       NewHandler("CMsgDelFriend", (*GameSession).HandleDelFriend),
	packet.CMsgSetContactNotes: NewHandler("CMsgSetContactNotes", (*GameSession).HandleSetContactNotes),
	packet.CMsgAddIgnore:       NewHandler("CMsgAddIgnore", (*GameSession).HandleAddIgnore),
	packet.CMsgDelIgnore:       NewHandler("CMsgDelIgnore", (*GameSession).HandleDelIgnore),

	// Channels
	packet.CMsgJoinChannel:          NewHandler("CMsgJoinChannel", (*GameSession).HandleJoinChannel),
	packet.CMsgLeaveChannel:         NewHandler("CMsgLeaveChannel", (*GameSession).HandleLeaveChannel),
	packet.CMsgChannelList:          NewHandler("CMsgChannelList", (*GameSession).HandleChannelList),
	packet.CMsgChannelDisplayList:   NewHandler("CMsgChannelDisplayList", (*GameSession).HandleChannelList),
	packet.SMsgChannelNotify:        NewHandler("SMsgChannelNotify", (*GameSession).InterceptWorldserverChannelNotify),
	packet.CMsgChannelPassword:      NewHandler("CMsgChannelPassword", (*GameSession).HandleChannelPassword),
	packet.CMsgChannelSetOwner:      NewHandler("CMsgChannelSetOwner", (*GameSession).HandleChannelSetOwner),
	packet.CMsgChannelModerator:     NewHandler("CMsgChannelModerator", (*GameSession).HandleChannelSetModerator),
	packet.CMsgChannelUnModerator:   NewHandler("CMsgChannelUnModerator", (*GameSession).HandleChannelUnsetModerator),
	packet.CMsgChannelMute:          NewHandler("CMsgChannelMute", (*GameSession).HandleChannelMute),
	packet.CMsgChannelUnmute:        NewHandler("CMsgChannelUnmute", (*GameSession).HandleChannelUnmute),
	packet.CMsgChannelInvite:        NewHandler("CMsgChannelInvite", (*GameSession).HandleChannelInvite),
	packet.CMsgChannelKick:          NewHandler("CMsgChannelKick", (*GameSession).HandleChannelKick),
	packet.CMsgChannelBan:           NewHandler("CMsgChannelBan", (*GameSession).HandleChannelBan),
	packet.CMsgChannelUnban:         NewHandler("CMsgChannelUnban", (*GameSession).HandleChannelUnban),
	packet.CMsgChannelAnnouncements: NewHandler("CMsgChannelAnnouncements", (*GameSession).HandleChannelAnnouncements),
	packet.CMsgChannelModerate:      NewHandler("CMsgChannelModerate", (*GameSession).HandleChannelModerate),

	packet.CMsgGuildInvite:             NewHandler("CMsgGuildInvite", (*GameSession).HandleGuildInvite),
	packet.CMsgGuildInviteAccept:       NewHandler("CMsgGuildInviteAccept", (*GameSession).HandleGuildInviteAccept),
	packet.CMsgGuildRoster:             NewHandler("CMsgGuildRoster", (*GameSession).HandleGuildRoster),
	packet.CMsgGuildLeave:              NewHandler("CMsgGuildLeave", (*GameSession).HandleGuildLeave),
	packet.CMsgGuildRemove:             NewHandler("CMsgGuildRemove", (*GameSession).HandleGuildKick),
	packet.CMsgGuildMOTD:               NewHandler("CMsgGuildSMTD", (*GameSession).HandleGuildSetMessageOfTheDay),
	packet.CMsgGuildSetPublicNote:      NewHandler("CMsgGuildSetPublicNote", (*GameSession).HandleGuildSetPublicNote),
	packet.CMsgGuildSetOfficerNote:     NewHandler("CMsgGuildSetOfficerNote", (*GameSession).HandleGuildSetOfficerNote),
	packet.CMsgGuildInfoText:           NewHandler("CMsgGuildInfoText", (*GameSession).HandleGuildSetInfoText),
	packet.CMsgGuildRank:               NewHandler("CMsgGuildRank", (*GameSession).HandleGuildRankUpdate),
	packet.CMsgGuildAddRank:            NewHandler("CMsgGuildAddRank", (*GameSession).HandleGuildRankAdd),
	packet.CMsgGuildDelRank:            NewHandler("CMsgGuildDelRank", (*GameSession).HandleGuildRankDelete),
	packet.CMsgGuildPromote:            NewHandler("CMsgGuildPromote", (*GameSession).HandleGuildPromote),
	packet.CMsgGuildDemote:             NewHandler("CMsgGuildDemote", (*GameSession).HandleGuildDemote),
	packet.CMsgOfferPetition:           NewHandler("CMsgOfferPetition", (*GameSession).HandleOfferGuildPetition),
	packet.CMsgPetitionSign:            NewHandler("CMsgPetitionSign", (*GameSession).HandleSignGuildPetition),
	packet.CMsgPetitionShowSignatures:  NewHandler("CMsgPetitionShowSignatures", (*GameSession).HandleGuildPetitionShowSignatures),
	packet.CMsgPetitionQuery:           NewHandler("CMsgPetitionQuery", (*GameSession).HandleGuildPetitionQuery),
	packet.CMsgTurnInPetition:          NewHandler("CMsgTurnInPetition", (*GameSession).HandleArenaTeamPetitionTurnIn),
	packet.SMsgGuildCommandResult:      NewHandler("SMsgGuildCommandResult", (*GameSession).InterceptGuildCommandResult),
	packet.SMsgTurnInPetitionResults:   NewHandler("SMsgTurnInPetitionResults", (*GameSession).InterceptTurnInPetitionResults),
	packet.MsgGuildPermissions:         NewHandler("MsgGuildPermissions", (*GameSession).HandleGuildPermissions),
	packet.MsgGuildBankMoneyWithdrawn:  NewHandler("MsgGuildBankMoneyWithdrawn", (*GameSession).HandleGuildBankMoneyWithdrawn),
	packet.CMsgGuildBankerActivate:     NewHandler("CMsgGuildBankerActivate", (*GameSession).HandleGuildBankerActivate),
	packet.CMsgGuildBankQueryTab:       NewHandler("CMsgGuildBankQueryTab", (*GameSession).HandleGuildBankQueryTab),
	packet.CMsgGuildBankSwapItems:      NewHandler("CMsgGuildBankSwapItems", (*GameSession).HandleGuildBankSwapItems),
	packet.CMsgGuildBankBuyTab:         NewHandler("CMsgGuildBankBuyTab", (*GameSession).HandleGuildBankBuyTab),
	packet.CMsgGuildBankUpdateTab:      NewHandler("CMsgGuildBankUpdateTab", (*GameSession).HandleGuildBankUpdateTab),
	packet.CMsgGuildBankDepositMoney:   NewHandler("CMsgGuildBankDepositMoney", (*GameSession).HandleGuildBankDepositMoney),
	packet.CMsgGuildBankWithdrawMoney:  NewHandler("CMsgGuildBankWithdrawMoney", (*GameSession).HandleGuildBankWithdrawMoney),
	packet.MsgGuildBankLogQuery:        NewHandler("MsgGuildBankLogQuery", (*GameSession).HandleGuildBankLogQuery),
	packet.MsgQueryGuildBankText:       NewHandler("MsgQueryGuildBankText", (*GameSession).HandleQueryGuildBankText),
	packet.CMsgSetGuildBankText:        NewHandler("CMsgSetGuildBankText", (*GameSession).HandleSetGuildBankText),
	packet.CMsgSendMail:                NewHandler("CMsgSendMail", (*GameSession).HandleSendMail),
	packet.CMsgGetMailList:             NewHandler("CMsgGetMailList", (*GameSession).HandleGetMailList),
	packet.CMsgMailMarkAsRead:          NewHandler("CMsgMailMarkAsRead", (*GameSession).HandleMailMarksAsRead),
	packet.CMsgMailTakeMoney:           NewHandler("CMsgMailTakeMoney", (*GameSession).HandleMailTakeMoney),
	packet.CMsgMailTakeItem:            NewHandler("CMsgMailTakeItem", (*GameSession).HandleMailTakeItem),
	packet.CMsgMailDelete:              NewHandler("CMsgMailDelete", (*GameSession).HandleDeleteMail),
	packet.MsgQueryNextMailTime:        NewHandler("MsgQueryNextMailTime", (*GameSession).HandleQueryNextMailTime),
	packet.SMsgInitWorldStates:         NewHandler("SMsgInitWorldStates", (*GameSession).InterceptInitWorldStates),
	packet.SMsgLevelUpInfo:             NewHandler("SMsgLevelUpInfo", (*GameSession).InterceptLevelUpInfo),
	packet.CMsgPing:                    NewHandler("CMsgPing", (*GameSession).HandlePing),
	packet.SMsgPong:                    NewHandler("SMsgPong", (*GameSession).InterceptPong),
	packet.SMsgNewWorld:                NewHandler("SMsgNewWorld", (*GameSession).InterceptNewWorld),
	packet.MsgMoveWorldPortAck:         NewHandler("MsgMoveWorldPortAck", (*GameSession).InterceptMoveWorldPortAck),
	packet.CMsgSetActiveMover:          NewHandler("CMsgSetActiveMover", (*GameSession).HandlePlayerWorldActivePacket),
	packet.CMsgWorldStateUiTimerUpdate: NewHandler("CMsgWorldStateUiTimerUpdate", (*GameSession).HandlePlayerWorldActivePacket),
	packet.SMsgLogoutComplete:          NewHandler("SMsgLogoutComplete", (*GameSession).InterceptLogoutComplete),
	packet.SMsgTransferPending:         NewHandler("SMsgTransferPending", (*GameSession).InterceptTransferPending),
	packet.SMsgUpdateObject:            NewHandler("SMsgUpdateObject", (*GameSession).InterceptObjectUpdate),
	packet.SMsgCompressedUpdateObject:  NewHandler("SMsgCompressedUpdateObject", (*GameSession).InterceptObjectUpdate),
	packet.SMsgGroupDestroyed:          NewHandler("SMsgGroupDestroyed", (*GameSession).InterceptGroupPresentationPacket),
	packet.SMsgGroupList:               NewHandler("SMsgGroupList", (*GameSession).InterceptGroupPresentationPacket),
	packet.SMsgPartyMemberStats:        NewHandler("SMsgPartyMemberStats", (*GameSession).InterceptPartyMemberStats),
	packet.SMsgPartyMemberStatsFull:    NewHandler("SMsgPartyMemberStatsFull", (*GameSession).InterceptPartyMemberStats),
	packet.SMsgHealthUpdate:            NewHandler("SMsgHealthUpdate", (*GameSession).InterceptDirectPlayerStateUpdate),
	packet.SMsgPowerUpdate:             NewHandler("SMsgPowerUpdate", (*GameSession).InterceptDirectPlayerStateUpdate),
	packet.SMsgAuraUpdateAll:           NewHandler("SMsgAuraUpdateAll", (*GameSession).InterceptAuraUpdate),
	packet.SMsgAuraUpdate:              NewHandler("SMsgAuraUpdate", (*GameSession).InterceptAuraUpdate),
	packet.SMsgMOTD:                    NewHandler("SMsgMOTD", (*GameSession).InterceptMessageOfTheDay),
	packet.SMsgAccountDataTimes:        NewHandler("SMsgAccountDataTimes", (*GameSession).InterceptAccountDataTimes),

	packet.TC9SMsgReadyForRedirect: NewHandler("TC9SMsgReadyForRedirect", (*GameSession).HandleReadyForRedirectRequest),

	packet.SMsgNameQueryResponse: NewHandler("SMsgNameQueryResponse", (*GameSession).InterceptSMsgNameQueryResponse),

	// Groups
	packet.CMsgGroupInvite:            NewHandler("CMsgGroupInvite", (*GameSession).HandleGroupInvite),
	packet.CMsgGroupAccept:            NewHandler("CMsgGroupAccept", (*GameSession).HandleGroupInviteAccept),
	packet.CMsgGroupDecline:           NewHandler("CMsgGroupDecline", (*GameSession).HandleGroupInviteDeclined),
	packet.CMsgGroupUnInvite:          NewHandler("CMsgGroupUnInvite", (*GameSession).HandleGroupUninvite),
	packet.CMsgGroupUnInviteGuid:      NewHandler("CMsgGroupUnInviteGuid", (*GameSession).HandleGroupUninviteGUID),
	packet.CMsgGroupDisband:           NewHandler("CMsgGroupDisband", (*GameSession).HandleGroupLeave),
	packet.CMsgGroupRaidConvert:       NewHandler("CMsgGroupRaidConvert", (*GameSession).HandleGroupConvertToRaid),
	packet.CMsgGroupSetLeader:         NewHandler("CMsgGroupSetLeader", (*GameSession).HandleGroupSetLeader),
	packet.MsgRaidTargetUpdate:        NewHandler("MsgRaidTargetUpdate", (*GameSession).HandleSetGroupTargetIcon),
	packet.CMsgLootMethod:             NewHandler("CMsgLootMethod", (*GameSession).HandleSetLootMethod),
	packet.MsgSetDungeonDifficulty:    NewHandler("MsgSetDungeonDifficulty", (*GameSession).HandleSetDungeonDifficulty),
	packet.MsgSetRaidDifficulty:       NewHandler("MsgSetRaidDifficulty", (*GameSession).HandleSetRaidDifficulty),
	packet.MsgRaidReadyCheck:          NewHandler("MsgRaidReadyCheck", (*GameSession).HandleRaidReadyCheck),
	packet.MsgRaidReadyCheckConfirm:   NewHandler("MsgRaidReadyCheckConfirm", (*GameSession).HandleRaidReadyCheckConfirm),
	packet.MsgRaidReadyCheckFinished:  NewHandler("MsgRaidReadyCheckFinished", (*GameSession).HandleRaidReadyCheckFinished),
	packet.CMsgGroupChangeSubGroup:    NewHandler("CMsgGroupChangeSubGroup", (*GameSession).HandleGroupChangeSubGroup),
	packet.CMsgGroupSwapSubGroup:      NewHandler("CMsgGroupSwapSubGroup", (*GameSession).HandleGroupSwapSubGroup),
	packet.CMsgGroupAssistantLeader:   NewHandler("CMsgGroupAssistantLeader", (*GameSession).HandleGroupAssistantLeader),
	packet.MsgPartyAssignment:         NewHandler("MsgPartyAssignment", (*GameSession).HandlePartyAssignment),
	packet.CMsgResetInstances:         NewHandler("CMsgResetInstances", (*GameSession).HandleResetInstances),
	packet.CMsgSetSavedInstanceExtend: NewHandler("CMsgSetSavedInstanceExtend", (*GameSession).HandleSetSavedInstanceExtend),

	// Auction House
	packet.MsgAuctionHello:            NewHandler("MsgAuctionHello", (*GameSession).HandleAuctionHello),
	packet.CMsgAuctionSellItem:        NewHandler("CMsgAuctionSellItem", (*GameSession).HandleAuctionSellItem),
	packet.CMsgAuctionPlaceBid:        NewHandler("CMsgAuctionPlaceBid", (*GameSession).HandleAuctionPlaceBid),
	packet.CMsgAuctionRemoveItem:      NewHandler("CMsgAuctionRemoveItem", (*GameSession).HandleAuctionRemoveItem),
	packet.CMsgAuctionListItems:       NewHandler("CMsgAuctionListItems", (*GameSession).HandleAuctionListItems),
	packet.CMsgAuctionListOwnerItems:  NewHandler("CMsgAuctionListOwnerItems", (*GameSession).HandleAuctionListOwnerItems),
	packet.CMsgAuctionListBidderItems: NewHandler("CMsgAuctionListBidderItems", (*GameSession).HandleAuctionListBidderItems),
	packet.CMsgAuctionListPendingSales: NewHandler("CMsgAuctionListPendingSales", (*GameSession).HandleAuctionListPendingSales),

	// Battlegrounds
	packet.CMsgBattleMasterJoin:      NewHandler("CMsgBattleMasterJoin", (*GameSession).HandleEnqueueToBattleground),
	packet.CMsgBattlemasterJoinArena: NewHandler("CMsgBattlemasterJoinArena", (*GameSession).HandleEnqueueToArena),
	packet.CMsgBattlefieldPort:       NewHandler("CMsgBattlefieldPort", (*GameSession).HandleBattlegroundPort),

	// Arena teams
	packet.CMsgArenaTeamQuery:   NewHandler("CMsgArenaTeamQuery", (*GameSession).HandleArenaTeamQuery),
	packet.CMsgArenaTeamRoster:  NewHandler("CMsgArenaTeamRoster", (*GameSession).HandleArenaTeamRoster),
	packet.CMsgArenaTeamInvite:  NewHandler("CMsgArenaTeamInvite", (*GameSession).HandleArenaTeamInvite),
	packet.CMsgArenaTeamAccept:  NewHandler("CMsgArenaTeamAccept", (*GameSession).HandleArenaTeamAccept),
	packet.CMsgArenaTeamDecline: NewHandler("CMsgArenaTeamDecline", (*GameSession).HandleArenaTeamDecline),
	packet.CMsgArenaTeamLeave:   NewHandler("CMsgArenaTeamLeave", (*GameSession).HandleArenaTeamLeave),
	packet.CMsgArenaTeamRemove:  NewHandler("CMsgArenaTeamRemove", (*GameSession).HandleArenaTeamRemove),
	packet.CMsgArenaTeamDisband: NewHandler("CMsgArenaTeamDisband", (*GameSession).HandleArenaTeamDisband),
	packet.CMsgArenaTeamLeader:  NewHandler("CMsgArenaTeamLeader", (*GameSession).HandleArenaTeamLeader),

	// LFG
	packet.CMsgLFGJoin:                  NewHandler("CMsgLFGJoin", (*GameSession).HandleLfgJoin),
	packet.CMsgLFGLeave:                 NewHandler("CMsgLFGLeave", (*GameSession).HandleLfgLeave),
	packet.CMsgLFGSetRoles:              NewHandler("CMsgLFGSetRoles", (*GameSession).HandleLfgSetRoles),
	packet.CMsgLFGProposalResult:        NewHandler("CMsgLFGProposalResult", (*GameSession).HandleLfgProposalResult),
	packet.CMsgLFGGetStatus:             NewHandler("CMsgLFGGetStatus", (*GameSession).HandleLfgGetStatus),
	packet.CMsgLfdPlayerLockInfoRequest: NewHandler("CMsgLfdPlayerLockInfoRequest", (*GameSession).HandleLfgPlayerLockInfoRequest),
	packet.CMsgLFGTeleport:              NewHandler("CMsgLFGTeleport", (*GameSession).HandleLfgTeleport),
	packet.SMsgLFGTeleportDenied:        NewHandler("SMsgLFGTeleportDenied", (*GameSession).InterceptLfgTeleportDenied),
	packet.SMsgLFGUpdateParty:           NewHandler("SMsgLFGUpdateParty", (*GameSession).InterceptLfgUpdateParty),
	packet.CMsgLFGSetBootVote:           NewHandler("CMsgLFGSetBootVote", (*GameSession).HandleLfgSetBootVote),
	packet.CMsgLfdPartyLockInfoRequest:  NewHandler("CMsgLfdPartyLockInfoRequest", (*GameSession).HandleLfgPartyLockInfoRequest),

	// Movements
	packet.MsgMoveStop:      NewHandler("MsgMoveStop", (*GameSession).HandleMovement),
	packet.MsgMoveHeartbeat: NewHandler("MsgMoveHeartbeat", (*GameSession).HandleMovement),
}

type Handler func(*GameSession, context.Context, *packet.Packet) error

func NewHandler(name string, handlers ...Handler) HandlersQueue {
	return HandlersQueue{
		name:  name,
		queue: handlers,
	}
}

type HandlersQueue struct {
	name  string
	queue []Handler
}

func (q *HandlersQueue) Handle(ctx context.Context, session *GameSession, p *packet.Packet) error {
	var err error
	for i := range q.queue {
		err = q.queue[i](session, ctx, p)
		if err != nil {
			return err
		}
	}
	return nil
}

func ForwardPacketToRandomGameServer(waitOpcodeToClose packet.Opcode) Handler {
	return func(s *GameSession, ctx context.Context, p *packet.Packet) error {
		serverResult, err := s.serversRegistryClient.RandomGameServerForRealm(ctx, &pbServ.RandomGameServerForRealmRequest{
			Api:     root.SupportedServerRegistryVer,
			RealmID: root.RealmID,
		})
		if err != nil {
			return err
		}

		if serverResult.GameServer == nil {
			return fmt.Errorf("no available game servers to handle 0x%X packet", uint16(p.Opcode))
		}

		socket, err := WorldSocketCreator(s.logger, serverResult.GameServer.Address)
		if err != nil {
			return fmt.Errorf("can't connect to the world server, err: %w", err)
		}

		go socket.ListenAndProcess(s.ctx)
		newCtx, cancel := context.WithTimeout(s.ctx, time.Minute)
		waitDone := make(chan struct{})
		go func() {
			defer cancel()
			defer func() { waitDone <- struct{}{} }()

			for {
				select {
				case p, open := <-socket.ReadChannel():
					if !open {
						return
					}
					s.gameSocket.WriteChannel() <- p
					if p.Opcode == waitOpcodeToClose {
						socket.Close()
						return
					}

				case <-newCtx.Done():
					if s.worldSocket != nil {
						s.worldSocket.Close()
					}
					return
				}
			}
		}()

		socket.SendPacket(s.authPacket)

		// we need to give some time to add session on the world side
		time.Sleep(time.Millisecond * 300)

		socket.SendPacket(p)

		if waitOpcodeToClose != 0 {
			<-waitDone
		} else {
			socket.Close()
		}
		return nil
	}
}
