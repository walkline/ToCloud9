package guids

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// GuidProvider provides guids that are unique in cluster.
type GuidProvider interface {
	// Next provides cluster unique safe to use guid.
	Next() uint64
}

// threadUnsafeGuidProvider provides next guids. Thread unsafe.
type threadUnsafeGuidProvider struct {
	iterator    uint64
	iteratorMax uint64

	iteratorValueToTriggerUpdate uint64

	pctUsedToTriggerUpdate float32

	diapasonsChan     chan []diapason
	diapasonsProvider DiapasonsProvider

	nextDiapasons []diapason
}

func NewThreadUnsafeGuidProvider(ctx context.Context, diapasonsProvider DiapasonsProvider, pctUsedToTriggerUpdate float32) (GuidProvider, error) {
	diapasons, err := diapasonsProvider.NewDiapasons(ctx)
	if err != nil {
		return nil, err
	}

	if len(diapasons) == 0 {
		return nil, fmt.Errorf("diapasons provider returned empty diapason")
	}

	provider := &threadUnsafeGuidProvider{
		pctUsedToTriggerUpdate: pctUsedToTriggerUpdate,
		diapasonsChan:          make(chan []diapason),
		diapasonsProvider:      diapasonsProvider,
		nextDiapasons:          diapasons[1:],
	}

	provider.iteratorValueToTriggerUpdate = provider.calculateIteratorValueToTriggerUpdate(diapasons)
	provider.iterator = diapasons[0].start
	provider.iteratorMax = diapasons[0].end

	return provider, nil
}

// Next provides cluster unique safe to use guid.
func (p *threadUnsafeGuidProvider) Next() (guid uint64) {
	if p.iteratorMax < p.iterator {
		p.reloadIterator()
	}

	if p.iterator == p.iteratorValueToTriggerUpdate {
		p.triggerDiapasonsRequest()
	}

	guid = p.iterator

	p.iterator++
	return
}

// reloadIterator reloads p.iterator by consuming p.nextDiapasons item.
func (p *threadUnsafeGuidProvider) reloadIterator() {
	if len(p.nextDiapasons) == 0 {
		// Blocks if new diapasons didn't arrive yet.
		p.reloadNextDiapasons()
	}

	p.iterator = p.nextDiapasons[0].start
	p.iteratorMax = p.nextDiapasons[0].end

	p.nextDiapasons = p.nextDiapasons[1:]
}

// reloadNextDiapasons reloads p.nextDiapasons with the new diapasons that were retrieved in triggerDiapasonsRequest.
// Also updates p.iteratorValueToTriggerUpdate with the new value.
func (p *threadUnsafeGuidProvider) reloadNextDiapasons() {
	p.nextDiapasons = <-p.diapasonsChan
	p.iteratorValueToTriggerUpdate = p.calculateIteratorValueToTriggerUpdate(p.nextDiapasons)
}

// calculateIteratorValueToTriggerUpdate calculates the value of iterator that will trigger retrieving of the new diapasons.
func (p *threadUnsafeGuidProvider) calculateIteratorValueToTriggerUpdate(diapasons []diapason) uint64 {
	totalGuids := uint64(0)
	for _, d := range diapasons {
		totalGuids += d.end - d.start + 1
	}

	pctValueToTrigger := uint64(float64(totalGuids) / 100 * float64(p.pctUsedToTriggerUpdate))

	if pctValueToTrigger <= diapasons[0].start {
		if diapasons[0].start == diapasons[0].end {
			return diapasons[0].end
		}

		return diapasons[0].start + 1
	}

	for _, d := range diapasons {
		if d.end-d.start+1 >= pctValueToTrigger {
			return d.start + pctValueToTrigger
		}

		pctValueToTrigger -= d.end - d.start + 1
	}

	log.Fatal().Msg("can't reload next guid diapason, trigger percent is bigger than available")
	return 0
}

// triggerDiapasonsRequest triggers request to get new diapasons in the new goroutine.
func (p *threadUnsafeGuidProvider) triggerDiapasonsRequest() {
	go func() {
		const retriesCount = 5
		const progressiveThrottleDelayMultiplayer = time.Millisecond * 100

		for i := 0; i < retriesCount; i++ {
			d, err := p.diapasonsProvider.NewDiapasons(context.Background())
			// We received new diapason go out of retries cycle.
			if err == nil && len(d) > 0 {
				p.diapasonsChan <- d
				return
			}

			log.Err(err).Msg("can't get new guid diapason, retrying...")

			time.Sleep(progressiveThrottleDelayMultiplayer * time.Duration(i+1))
		}

		log.Fatal().Msg("can't get new guid diapason")
	}()
}
