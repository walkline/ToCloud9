package events

import (
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

type GWCharacterLoggedInHandler interface {
	// HandleCharacterLoggedIn handles player logged in events.
	HandleCharacterLoggedIn(payload GWEventCharacterLoggedInPayload) error
}

type GWCharacterLoggedOutHandler interface {
	// HandleCharacterLoggedOut handles player logged out events.
	HandleCharacterLoggedOut(payload GWEventCharacterLoggedOutPayload) error
}

type GWCharactersUpdatesHandler interface {
	// HandleCharactersUpdates handles pack of characters updates events.
	HandleCharactersUpdates(payload GWEventCharactersUpdatesPayload) error
}

// GatewayConsumer listens to gateway events and handles events if there are handlers.
type GatewayConsumer interface {
	// Listen is non-blocking operation that listens to the gateway events.
	Listen() error

	// Stop stops listening to events.
	Stop() error
}

// NewGatewayConsumer creates new GatewayConsumer with given options.
// Need to provide at least one handler as option, otherwise it will not listen at all.
func NewGatewayConsumer(nc *nats.Conn, options ...GatewayConsumerOption) GatewayConsumer {
	params := &gatewayConsumerParams{}
	for _, opt := range options {
		opt.apply(params)
	}
	return &gatewayConsumerImpl{
		nc:                  nc,
		loggedInHandler:     params.loggedInHandler,
		loggedOutHandler:    params.loggedOutHandler,
		charsUpdatesHandler: params.charsUpdatesHandler,
	}
}

// GatewayConsumerOption option to initialize gateway consumer.
type GatewayConsumerOption interface {
	apply(*gatewayConsumerParams)
}

// WithGWConsumerLoggedInHandler creates gateway consumer option with logged in handler.
// If not specified, listener will ignore these kind of events.
func WithGWConsumerLoggedInHandler(h GWCharacterLoggedInHandler) GatewayConsumerOption {
	return newFuncGatewayConsumerOption(func(params *gatewayConsumerParams) {
		params.loggedInHandler = h
	})
}

// WithGWConsumerLoggedOutHandler creates gateway consumer option with logged out handler.
// If not specified, listener will ignore this kind of events.
func WithGWConsumerLoggedOutHandler(h GWCharacterLoggedOutHandler) GatewayConsumerOption {
	return newFuncGatewayConsumerOption(func(params *gatewayConsumerParams) {
		params.loggedOutHandler = h
	})
}

// WithGWConsumerCharsUpdatesHandler creates gateway consumer option with characters updates handler.
// If not specified, listener will ignore this kind of events.
func WithGWConsumerCharsUpdatesHandler(h GWCharactersUpdatesHandler) GatewayConsumerOption {
	return newFuncGatewayConsumerOption(func(params *gatewayConsumerParams) {
		params.charsUpdatesHandler = h
	})
}

// funcGatewayConsumerOption wraps a function that modifies funcGatewayConsumerOption into an
// implementation of the GatewayConsumerOption interface.
type funcGatewayConsumerOption struct {
	f func(*gatewayConsumerParams)
}

func (f *funcGatewayConsumerOption) apply(do *gatewayConsumerParams) {
	f.f(do)
}

func newFuncGatewayConsumerOption(f func(*gatewayConsumerParams)) *funcGatewayConsumerOption {
	return &funcGatewayConsumerOption{
		f: f,
	}
}

// gatewayConsumerParams list of all possible parameters of GatewayConsumer.
type gatewayConsumerParams struct {
	loggedInHandler     GWCharacterLoggedInHandler
	loggedOutHandler    GWCharacterLoggedOutHandler
	charsUpdatesHandler GWCharactersUpdatesHandler
}

// gatewayConsumerImpl implementation of GatewayConsumer.
type gatewayConsumerImpl struct {
	nc *nats.Conn
	// subs is all subscriptions list.
	subs []*nats.Subscription

	loggedInHandler     GWCharacterLoggedInHandler
	loggedOutHandler    GWCharacterLoggedOutHandler
	charsUpdatesHandler GWCharactersUpdatesHandler
}

// Listen is non-blocking operation that listens to gateway events.
func (c *gatewayConsumerImpl) Listen() error {
	if c.loggedInHandler != nil {
		sub, err := c.nc.Subscribe(GWEventCharacterLoggedIn.SubjectName(), func(msg *nats.Msg) {
			loggedInP := GWEventCharacterLoggedInPayload{}
			_, err := Unmarshal(msg.Data, &loggedInP)
			if err != nil {
				log.Error().Err(err).Msg("can't read GWEventCharacterLoggedIn (payload part) event")
				return
			}

			err = c.loggedInHandler.HandleCharacterLoggedIn(loggedInP)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GWEventCharacterLoggedIn event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.loggedOutHandler != nil {
		sub, err := c.nc.Subscribe(GWEventCharacterLoggedOut.SubjectName(), func(msg *nats.Msg) {
			loggedOutP := GWEventCharacterLoggedOutPayload{}
			_, err := Unmarshal(msg.Data, &loggedOutP)
			if err != nil {
				log.Error().Err(err).Msg("can't read GWEventCharacterLoggedOut (payload part) event")
				return
			}

			err = c.loggedOutHandler.HandleCharacterLoggedOut(loggedOutP)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GWEventCharacterLoggedOut event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.charsUpdatesHandler != nil {
		sub, err := c.nc.Subscribe(GWEventCharactersUpdates.SubjectName(), func(msg *nats.Msg) {
			charsUpdtsP := GWEventCharactersUpdatesPayload{}
			_, err := Unmarshal(msg.Data, &charsUpdtsP)
			if err != nil {
				log.Error().Err(err).Msg("can't read GWEventCharactersUpdates (payload part) event")
				return
			}

			err = c.charsUpdatesHandler.HandleCharactersUpdates(charsUpdtsP)
			if err != nil {
				log.Error().Err(err).Msg("can't handle GWEventCharactersUpdates event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	return nil
}

// Stop stops listening to events.
func (c *gatewayConsumerImpl) Stop() error {
	return c.unsubscribe()
}

func (c *gatewayConsumerImpl) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
