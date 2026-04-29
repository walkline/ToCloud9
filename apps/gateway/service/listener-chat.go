package service

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/shared/events"
)

type chatNatsListener struct {
	nc          *nats.Conn
	subs        []*nats.Subscription
	lbID        string
	broadcaster eBroadcaster.Broadcaster
}

func NewChatNatsListener(nc *nats.Conn, lbID string, broadcaster eBroadcaster.Broadcaster) Listener {
	return &chatNatsListener{
		nc:          nc,
		lbID:        lbID,
		broadcaster: broadcaster,
	}
}

func (c *chatNatsListener) Listen() error {
	// Subscribe to whisper messages
	sb, err := c.nc.Subscribe(events.ChatEventIncomingWhisper.SubjectName(c.lbID), func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read incoming whisper event")
			return
		}

		chatMsg := events.ChatEventIncomingWhisperPayload{}
		err = json.Unmarshal(p.Payload, &chatMsg)
		if err != nil {
			log.Error().Err(err).Msg("can't read incoming whisper payload")
			return
		}

		c.broadcaster.NewIncomingWhisperEvent(&eBroadcaster.IncomingWhisperPayload{
			SenderGUID:   chatMsg.SenderGUID,
			SenderName:   chatMsg.SenderName,
			SenderRace:   chatMsg.SenderRace,
			ReceiverGUID: chatMsg.ReceiverGUID,
			ReceiverName: chatMsg.ReceiverName,
			Language:     chatMsg.Language,
			Msg:          chatMsg.Msg,
		})
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sb)

	// Subscribe to channel messages (use "ALL" for broadcast events)
	subject := events.ChatEventChannelMessage.SubjectName("ALL")
	sbChannelMsg, err := c.nc.Subscribe(subject, func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read channel message event")
			return
		}

		channelMsg := events.ChatEventChannelMessagePayload{}
		err = json.Unmarshal(p.Payload, &channelMsg)
		if err != nil {
			log.Error().Err(err).Msg("can't read channel message payload")
			return
		}

		c.broadcaster.NewChannelMessageEvent(&eBroadcaster.ChannelMessagePayload{
			RealmID:     channelMsg.RealmID,
			ChannelName: channelMsg.ChannelName,
			ChannelID:   channelMsg.ChannelID,
			SenderGUID:  channelMsg.SenderGUID,
			SenderName:  channelMsg.SenderName,
			Language:    channelMsg.Language,
			Message:     channelMsg.Message,
		})
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sbChannelMsg)

	// Subscribe to channel join events (use "ALL" for broadcast events)
	subject = events.ChatEventChannelJoined.SubjectName("ALL")
	sbChannelJoin, err := c.nc.Subscribe(subject, func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read channel joined event")
			return
		}

		channelJoin := events.ChatEventChannelJoinedPayload{}
		err = json.Unmarshal(p.Payload, &channelJoin)
		if err != nil {
			log.Error().Err(err).Msg("can't read channel joined payload")
			return
		}

		c.broadcaster.NewChannelJoinedEvent(&eBroadcaster.ChannelJoinedPayload{
			RealmID:     channelJoin.RealmID,
			ChannelName: channelJoin.ChannelName,
			ChannelID:   channelJoin.ChannelID,
			PlayerGUID:  channelJoin.PlayerGUID,
			PlayerName:  channelJoin.PlayerName,
			PlayerFlags: channelJoin.PlayerFlags,
		})
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sbChannelJoin)

	// Subscribe to channel leave events (use "ALL" for broadcast events)
	subject = events.ChatEventChannelLeft.SubjectName("ALL")
	sbChannelLeft, err := c.nc.Subscribe(subject, func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read channel left event")
			return
		}

		channelLeft := events.ChatEventChannelLeftPayload{}
		err = json.Unmarshal(p.Payload, &channelLeft)
		if err != nil {
			log.Error().Err(err).Msg("can't read channel left payload")
			return
		}

		c.broadcaster.NewChannelLeftEvent(&eBroadcaster.ChannelLeftPayload{
			RealmID:     channelLeft.RealmID,
			ChannelName: channelLeft.ChannelName,
			ChannelID:   channelLeft.ChannelID,
			PlayerGUID:  channelLeft.PlayerGUID,
			PlayerName:  channelLeft.PlayerName,
		})
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sbChannelLeft)

	// Subscribe to channel notification events (use "ALL" for broadcast events)
	subject = events.ChatEventChannelNotification.SubjectName("ALL")
	sbChannelNotif, err := c.nc.Subscribe(subject, func(msg *nats.Msg) {
		p := events.EventToReadGenericPayload{}
		err := json.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read channel notification event")
			return
		}

		channelNotif := events.ChatEventChannelNotificationPayload{}
		err = json.Unmarshal(p.Payload, &channelNotif)
		if err != nil {
			log.Error().Err(err).Msg("can't read channel notification payload")
			return
		}

		c.broadcaster.NewChannelNotificationEvent(&eBroadcaster.ChannelNotificationPayload{
			RealmID:       channelNotif.RealmID,
			ChannelName:   channelNotif.ChannelName,
			ChannelID:     channelNotif.ChannelID,
			NotifyType:    channelNotif.NotifyType,
			TargetGUID:    channelNotif.TargetGUID,
			TargetName:    channelNotif.TargetName,
			SecondGUID:    channelNotif.SecondGUID,
			OldFlags:      channelNotif.OldFlags,
			NewFlags:      channelNotif.NewFlags,
			ExtraData:     channelNotif.ExtraData,
			AffectsPlayer: channelNotif.AffectsPlayer,
		})
	})
	if err != nil {
		return err
	}
	c.subs = append(c.subs, sbChannelNotif)

	return nil
}

func (c *chatNatsListener) Stop() error {
	return c.unsubscribe()
}

func (c *chatNatsListener) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
