package service

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"

	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/shared/events"
)

type petitionNatsListener struct {
	nc          *nats.Conn
	subs        []*nats.Subscription
	realmID     uint32
	broadcaster eBroadcaster.Broadcaster
}

func NewPetitionNatsListener(nc *nats.Conn, realmID uint32, broadcaster eBroadcaster.Broadcaster) Listener {
	return &petitionNatsListener{
		nc:          nc,
		realmID:     realmID,
		broadcaster: broadcaster,
	}
}

func (p *petitionNatsListener) Listen() error {
	if err := p.newSubscribe(events.PetitionEventSignResult, func() (interface{}, func()) {
		d := &events.PetitionEventSignResultPayload{}
		return d, func() {
			if d.RealmID != p.realmID {
				return
			}
			p.broadcaster.NewPetitionSignResultEvent(&eBroadcaster.PetitionSignResultPayload{
				RealmID:          d.RealmID,
				PetitionItemGUID: d.PetitionItemGUID,
				OwnerGUID:        d.OwnerGUID,
				SignerGUID:       d.SignerGUID,
				Result:           d.Result,
			})
		}
	}); err != nil {
		return err
	}

	if err := p.newSubscribe(events.PetitionEventOffered, func() (interface{}, func()) {
		d := &events.PetitionEventOfferedPayload{}
		return d, func() {
			if d.RealmID != p.realmID {
				return
			}
			p.broadcaster.NewPetitionOfferedEvent(&eBroadcaster.PetitionOfferedPayload{
				RealmID:          d.RealmID,
				PetitionItemGUID: d.PetitionItemGUID,
				OwnerGUID:        d.OwnerGUID,
				PetitionID:       d.PetitionID,
				TargetGUID:       d.TargetGUID,
				SignatoryGUIDs:   d.SignatoryGUIDs,
			})
		}
	}); err != nil {
		return err
	}

	if err := p.newSubscribe(events.PetitionEventDeclined, func() (interface{}, func()) {
		d := &events.PetitionEventDeclinedPayload{}
		return d, func() {
			if d.RealmID != p.realmID {
				return
			}
			p.broadcaster.NewPetitionDeclinedEvent(&eBroadcaster.PetitionDeclinedPayload{
				RealmID:    d.RealmID,
				OwnerGUID:  d.OwnerGUID,
				SignerGUID: d.SignerGUID,
			})
		}
	}); err != nil {
		return err
	}

	return nil
}

func (p *petitionNatsListener) Stop() error {
	for _, sub := range p.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}
	return nil
}

func (p *petitionNatsListener) newSubscribe(event events.PetitionServiceEvent, payloadAndHandler func() (interface{}, func())) error {
	sb, err := p.nc.Subscribe(event.SubjectName(), func(msg *nats.Msg) {
		envelope := events.EventToReadGenericPayload{}
		if err := json.Unmarshal(msg.Data, &envelope); err != nil {
			log.Error().Err(err).Msgf("can't read %v event", event)
			return
		}

		payload, handler := payloadAndHandler()
		if err := json.Unmarshal(envelope.Payload, payload); err != nil {
			log.Error().Err(err).Msgf("can't read %v (payload part) event", event)
			return
		}
		handler()
	})
	if err != nil {
		return err
	}
	p.subs = append(p.subs, sb)
	return nil
}
