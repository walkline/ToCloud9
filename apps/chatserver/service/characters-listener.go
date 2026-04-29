package service

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/chatserver/repo"
	"github.com/walkline/ToCloud9/apps/chatserver/sender"
	"github.com/walkline/ToCloud9/shared/events"
)

type CharactersListener struct {
	charRepo   repo.CharactersRepo
	channelMgr *ChannelManager
	nc         *nats.Conn
	producer   events.ChatServiceProducer
	subs       []*nats.Subscription
}

func NewCharactersListener(charRepo repo.CharactersRepo, channelMgr *ChannelManager, nc *nats.Conn) *CharactersListener {
	return &CharactersListener{
		charRepo:   charRepo,
		channelMgr: channelMgr,
		nc:         nc,
		producer:   events.NewChatServiceProducerNatsJSON(nc, "0.0.1", "ALL"), // Broadcast to all
	}
}

func (c *CharactersListener) Listen() error {
	sb, err := c.nc.Subscribe(events.GWEventCharacterLoggedIn.SubjectName(), func(msg *nats.Msg) {
		loggedInP := events.GWEventCharacterLoggedInPayload{}
		_, err := events.Unmarshal(msg.Data, &loggedInP)
		if err != nil {
			log.Error().Err(err).Msg("can't read GWEventCharacterLoggedIn (payload part) event")
			return
		}

		err = c.charRepo.AddCharacter(context.TODO(), &repo.Character{
			RealmID:   loggedInP.RealmID,
			GatewayID: loggedInP.GatewayID,
			GUID:      loggedInP.CharGUID,
			Name:      loggedInP.CharName,
			Race:      loggedInP.CharRace,
			MsgSender: sender.NewMsgSenderNatsJSON(c.nc, loggedInP.GatewayID),
		})

		if err != nil {
			log.Error().Err(err).Msg("can't add character in GWEventCharacterLoggedIn event")
			return
		}
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sb)

	sb, err = c.nc.Subscribe(events.GWEventCharacterLoggedOut.SubjectName(), func(msg *nats.Msg) {
		loggedOutP := events.GWEventCharacterLoggedOutPayload{}
		_, err := events.Unmarshal(msg.Data, &loggedOutP)
		if err != nil {
			log.Error().Err(err).Msg("can't read GWEventCharacterLoggedOut (payload part) event")
			return
		}

		// Transfer ownership if player was owner (but keep them as member)
		transfers := c.channelMgr.TransferOwnershipOnLogout(loggedOutP.RealmID, loggedOutP.CharGUID)

		// Broadcast ownership changes for each transfer
		for _, transfer := range transfers {
			// Send mode change notification
			payload := &events.ChatEventChannelNotificationPayload{
				RealmID:     loggedOutP.RealmID,
				ChannelName: transfer.ChannelName,
				ChannelID:   transfer.ChannelID,
				NotifyType:  0x0C, // CHAT_MODE_CHANGE_NOTICE
				TargetGUID:  transfer.NewOwnerGUID,
			}
			if err := c.producer.ChannelNotification(payload); err != nil {
				log.Error().Err(err).
					Str("channelName", transfer.ChannelName).
					Uint64("newOwner", transfer.NewOwnerGUID).
					Msg("Failed to broadcast mode change on logout ownership transfer")
			}

			// Send owner changed notification
			payload.NotifyType = 0x08 // CHAT_OWNER_CHANGED_NOTICE
			if err := c.producer.ChannelNotification(payload); err != nil {
				log.Error().Err(err).
					Str("channelName", transfer.ChannelName).
					Uint64("newOwner", transfer.NewOwnerGUID).
					Msg("Failed to broadcast owner changed on logout ownership transfer")
			}
		}

		err = c.charRepo.RemoveCharacter(context.TODO(), loggedOutP.RealmID, loggedOutP.CharGUID)
		if err != nil {
			log.Error().Err(err).Msg("can't remove character in GWEventCharacterLoggedOut event")
			return
		}
	})
	if err != nil {
		c.unsubscribe()
		return err
	}

	c.subs = append(c.subs, sb)

	return nil
}

func (c *CharactersListener) Stop() error {
	return c.unsubscribe()
}

func (c *CharactersListener) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
