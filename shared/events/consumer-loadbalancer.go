package events

import (
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

type LBCharacterLoggedInHandler interface {
	// HandleCharacterLoggedIn handles player logged in events.
	HandleCharacterLoggedIn(payload LBEventCharacterLoggedInPayload) error
}

type LBCharacterLoggedOutHandler interface {
	// HandleCharacterLoggedOut handles player logged out events.
	HandleCharacterLoggedOut(payload LBEventCharacterLoggedOutPayload) error
}

type LBCharactersUpdatesHandler interface {
	// HandleCharactersUpdates handles pack of characters updates events.
	HandleCharactersUpdates(payload LBEventCharactersUpdatesPayload) error
}

// LoadBalancerConsumer listens to load balancer events and handles events if there are handlers.
type LoadBalancerConsumer interface {
	// Listen is non-blocking operation that listens to the load balancer events.
	Listen() error

	// Stop stops listening to events.
	Stop() error
}

// NewLoadBalancerConsumer creates new LoadBalancerConsumer with given options.
// Need to provide at least one handler as option, otherwise it will not listen at all.
func NewLoadBalancerConsumer(nc *nats.Conn, options ...LoadBalancerConsumerOption) LoadBalancerConsumer {
	params := &loadBalancerConsumerParams{}
	for _, opt := range options {
		opt.apply(params)
	}
	return &loadBalancerConsumerImpl{
		nc:                  nc,
		loggedInHandler:     params.loggedInHandler,
		loggedOutHandler:    params.loggedOutHandler,
		charsUpdatesHandler: params.charsUpdatesHandler,
	}
}

// LoadBalancerConsumerOption option to initialize load balancer consumer.
type LoadBalancerConsumerOption interface {
	apply(*loadBalancerConsumerParams)
}

// WithLBConsumerLoggedInHandler creates load balancer consumer option with logged in handler.
// If not specified, listener will ignore this kind of events.
func WithLBConsumerLoggedInHandler(h LBCharacterLoggedInHandler) LoadBalancerConsumerOption {
	return newFuncLoadBalancerConsumerOption(func(params *loadBalancerConsumerParams) {
		params.loggedInHandler = h
	})
}

// WithLBConsumerLoggedOutHandler creates load balancer consumer option with logged out handler.
// If not specified, listener will ignore this kind of events.
func WithLBConsumerLoggedOutHandler(h LBCharacterLoggedOutHandler) LoadBalancerConsumerOption {
	return newFuncLoadBalancerConsumerOption(func(params *loadBalancerConsumerParams) {
		params.loggedOutHandler = h
	})
}

// WithLBConsumerCharsUpdatesHandler creates load balancer consumer option with characters updates handler.
// If not specified, listener will ignore this kind of events.
func WithLBConsumerCharsUpdatesHandler(h LBCharactersUpdatesHandler) LoadBalancerConsumerOption {
	return newFuncLoadBalancerConsumerOption(func(params *loadBalancerConsumerParams) {
		params.charsUpdatesHandler = h
	})
}

// funcLoadBalancerConsumerOption wraps a function that modifies funcLoadBalancerConsumerOption into an
// implementation of the LoadBalancerConsumerOption interface.
type funcLoadBalancerConsumerOption struct {
	f func(*loadBalancerConsumerParams)
}

func (f *funcLoadBalancerConsumerOption) apply(do *loadBalancerConsumerParams) {
	f.f(do)
}

func newFuncLoadBalancerConsumerOption(f func(*loadBalancerConsumerParams)) *funcLoadBalancerConsumerOption {
	return &funcLoadBalancerConsumerOption{
		f: f,
	}
}

// loadBalancerConsumerParams list of all possible parameters of LoadBalancerConsumer.
type loadBalancerConsumerParams struct {
	loggedInHandler     LBCharacterLoggedInHandler
	loggedOutHandler    LBCharacterLoggedOutHandler
	charsUpdatesHandler LBCharactersUpdatesHandler
}

// loadBalancerConsumerImpl implementation of LoadBalancerConsumer.
type loadBalancerConsumerImpl struct {
	nc *nats.Conn
	// subs is all subscriptions list.
	subs []*nats.Subscription

	loggedInHandler     LBCharacterLoggedInHandler
	loggedOutHandler    LBCharacterLoggedOutHandler
	charsUpdatesHandler LBCharactersUpdatesHandler
}

// Listen is non-blocking operation that listens to load balancer events.
func (c *loadBalancerConsumerImpl) Listen() error {
	if c.loggedInHandler != nil {
		sub, err := c.nc.Subscribe(LBEventCharacterLoggedIn.SubjectName(), func(msg *nats.Msg) {
			loggedInP := LBEventCharacterLoggedInPayload{}
			_, err := Unmarshal(msg.Data, &loggedInP)
			if err != nil {
				log.Error().Err(err).Msg("can't read LBEventCharacterLoggedIn (payload part) event")
				return
			}

			err = c.loggedInHandler.HandleCharacterLoggedIn(loggedInP)
			if err != nil {
				log.Error().Err(err).Msg("can't handle LBEventCharacterLoggedIn event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.loggedOutHandler != nil {
		sub, err := c.nc.Subscribe(LBEventCharacterLoggedOut.SubjectName(), func(msg *nats.Msg) {
			loggedOutP := LBEventCharacterLoggedOutPayload{}
			_, err := Unmarshal(msg.Data, &loggedOutP)
			if err != nil {
				log.Error().Err(err).Msg("can't read LBEventCharacterLoggedOut (payload part) event")
				return
			}

			err = c.loggedOutHandler.HandleCharacterLoggedOut(loggedOutP)
			if err != nil {
				log.Error().Err(err).Msg("can't handle LBEventCharacterLoggedOut event")
				return
			}
		})
		if err != nil {
			return err
		}

		c.subs = append(c.subs, sub)
	}

	if c.charsUpdatesHandler != nil {
		sub, err := c.nc.Subscribe(LBEventCharactersUpdates.SubjectName(), func(msg *nats.Msg) {
			charsUpdtsP := LBEventCharactersUpdatesPayload{}
			_, err := Unmarshal(msg.Data, &charsUpdtsP)
			if err != nil {
				log.Error().Err(err).Msg("can't read LBEventCharactersUpdates (payload part) event")
				return
			}

			err = c.charsUpdatesHandler.HandleCharactersUpdates(charsUpdtsP)
			if err != nil {
				log.Error().Err(err).Msg("can't handle LBEventCharactersUpdates event")
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
func (c *loadBalancerConsumerImpl) Stop() error {
	return c.unsubscribe()
}

func (c *loadBalancerConsumerImpl) unsubscribe() error {
	for _, sub := range c.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}

	return nil
}
