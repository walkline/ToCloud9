package service

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/shared/events"
)

type CharactersUpdatesBarrier struct {
	logger *zerolog.Logger

	producer events.GatewayProducer
	updsChan chan events.CharacterUpdate

	barrierOpenTime time.Duration
}

func NewCharactersUpdatesBarrier(logger *zerolog.Logger, producer events.GatewayProducer, barrierOpenTime time.Duration) *CharactersUpdatesBarrier {
	return &CharactersUpdatesBarrier{
		logger:          logger,
		producer:        producer,
		updsChan:        make(chan events.CharacterUpdate, 1000),
		barrierOpenTime: barrierOpenTime,
	}
}

func (b *CharactersUpdatesBarrier) UpdateLevel(charGUID uint64, lvl uint8) {
	b.updsChan <- events.CharacterUpdate{ID: charGUID, Lvl: &lvl}
}

func (b *CharactersUpdatesBarrier) UpdateZone(charGUID uint64, area, zone uint32) {
	b.updsChan <- events.CharacterUpdate{ID: charGUID, Area: &area, Zone: &zone}
}

func (b *CharactersUpdatesBarrier) UpdateMap(charGUID uint64, mapID uint32) {
	b.updsChan <- events.CharacterUpdate{ID: charGUID, Map: &mapID}
}

func (b *CharactersUpdatesBarrier) UpdateHealth(charGUID uint64, curHP uint32) {
	b.updsChan <- events.CharacterUpdate{ID: charGUID, CurHP: &curHP}
}

func (b *CharactersUpdatesBarrier) UpdateMaxHealth(charGUID uint64, maxHP uint32) {
	b.updsChan <- events.CharacterUpdate{ID: charGUID, MaxHP: &maxHP}
}

func (b *CharactersUpdatesBarrier) UpdatePowerType(charGUID uint64, powerType uint8) {
	b.updsChan <- events.CharacterUpdate{ID: charGUID, PowerType: &powerType}
}

func (b *CharactersUpdatesBarrier) UpdatePower(charGUID uint64, curPower uint32) {
	b.updsChan <- events.CharacterUpdate{ID: charGUID, CurPower: &curPower}
}

func (b *CharactersUpdatesBarrier) UpdateMaxPower(charGUID uint64, maxPower uint32) {
	b.updsChan <- events.CharacterUpdate{ID: charGUID, MaxPower: &maxPower}
}

func (b *CharactersUpdatesBarrier) Run(ctx context.Context) {
	t := time.NewTicker(b.barrierOpenTime)
	buffer := map[uint64]*events.CharacterUpdate{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := b.send(buffer); err != nil {
				b.logger.Error().Err(err).Msg("can't send updates")
				// don't clear buffer, lets try later
				continue
			}
			buffer = map[uint64]*events.CharacterUpdate{}
		case u := <-b.updsChan:
			if oldUpd := buffer[u.ID]; oldUpd != nil {
				u = mergeCharUpdates(*oldUpd, u)
			}
			buffer[u.ID] = &u
		}
	}
}

func (b *CharactersUpdatesBarrier) send(upds map[uint64]*events.CharacterUpdate) error {
	bufferSize := 1000
	if len(upds) < bufferSize {
		bufferSize = len(upds)
	}

	buffer := make([]*events.CharacterUpdate, 0, bufferSize)
	i := 0
	for _, v := range upds {
		buffer = append(buffer, v)
		if i > bufferSize {
			err := b.producer.CharactersUpdates(&events.GWEventCharactersUpdatesPayload{
				Updates: buffer,
			})
			if err != nil {
				return err
			}

			buffer = make([]*events.CharacterUpdate, 0, bufferSize)
			i = 0
			continue
		}
		i++
	}

	if len(buffer) > 0 {
		err := b.producer.CharactersUpdates(&events.GWEventCharactersUpdatesPayload{
			Updates: buffer,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func mergeCharUpdates(oldCharUpd, newCharUpd events.CharacterUpdate) events.CharacterUpdate {
	if newCharUpd.Lvl != nil {
		oldCharUpd.Lvl = newCharUpd.Lvl
	}

	if newCharUpd.Map != nil {
		oldCharUpd.Map = newCharUpd.Map
	}

	if newCharUpd.Area != nil {
		oldCharUpd.Area = newCharUpd.Area
	}

	if newCharUpd.Zone != nil {
		oldCharUpd.Zone = newCharUpd.Zone
	}

	if newCharUpd.CurHP != nil {
		oldCharUpd.CurHP = newCharUpd.CurHP
	}

	if newCharUpd.MaxHP != nil {
		oldCharUpd.MaxHP = newCharUpd.MaxHP
	}

	if newCharUpd.PowerType != nil {
		oldCharUpd.PowerType = newCharUpd.PowerType
	}

	if newCharUpd.CurPower != nil {
		oldCharUpd.CurPower = newCharUpd.CurPower
	}

	if newCharUpd.MaxPower != nil {
		oldCharUpd.MaxPower = newCharUpd.MaxPower
	}

	return oldCharUpd
}
