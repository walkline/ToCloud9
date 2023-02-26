package consumer

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/shared/events"
)

type Consumer interface {
	Start() error
	Stop() error
}

type natsConsumer struct {
	nc *nats.Conn
	// subs is all subscriptions list.
	subs []*nats.Subscription

	handlersFabric GuildHandlersFabric

	queue HandlersQueue
}

func NewNatsEventsConsumer(nc *nats.Conn, handlersFabric GuildHandlersFabric, queue HandlersQueue) Consumer {
	return &natsConsumer{
		nc:             nc,
		handlersFabric: handlersFabric,
		queue:          queue,
	}
}

func (c *natsConsumer) Start() error {
	sub, err := c.nc.Subscribe(events.GuildEventMemberAdded.SubjectName(), func(msg *nats.Msg) {
		p := events.GuildEventMemberAddedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GuildEventMemberAdded (payload part) event")
			return
		}
		fmt.Println("events.GuildEventMemberAdded")

		handler := c.handlersFabric.GuildMemberAddedHandler(p.GuildID, p.MemberGUID)
		c.queue.Push(handler)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GuildEventMemberKicked.SubjectName(), func(msg *nats.Msg) {
		p := events.GuildEventMemberKickedPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GuildEventMemberKicked (payload part) event")
			return
		}

		handler := c.handlersFabric.GuildMemberRemovedHandler(p.GuildID, p.MemberGUID)
		c.queue.Push(handler)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	sub, err = c.nc.Subscribe(events.GuildEventMemberLeft.SubjectName(), func(msg *nats.Msg) {
		p := events.GuildEventMemberLeftPayload{}
		_, err := events.Unmarshal(msg.Data, &p)
		if err != nil {
			log.Error().Err(err).Msg("can't read GuildEventMemberLeft (payload part) event")
			return
		}

		handler := c.handlersFabric.GuildMemberLeftHandler(p.GuildID, p.MemberGUID)
		c.queue.Push(handler)
	})
	if err != nil {
		return err
	}

	c.subs = append(c.subs, sub)

	return nil
}

func (c *natsConsumer) Stop() error {
	return c.unsubscribe()
}

func (c *natsConsumer) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
