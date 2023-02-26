package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/guidserver/repo"
)

var RequestGuidsBufferMultiplayer = 5

type GuidType uint8

const (
	GuidTypeCharacter GuidType = iota
	GuidTypeItem
	GuidTypeMax
)

type GuidService interface {
	GetGuids(ctx context.Context, realmID uint32, guidType uint8, desiredSize uint64) ([]GuidDiapason, error)
}

type guidServiceImpl struct {
	maxGuidsStorage repo.MaxGuidStorage

	requestChan chan *guidsRequest
	localCache  map[uint32][GuidTypeMax]*AvailableDiapasons
}

func NewGuidService(ctx context.Context, mysql repo.MaxGuidProvider, redisStorage repo.MaxGuidStorage, realmIDs []uint32, workersCount int) (GuidService, error) {
	service := &guidServiceImpl{
		maxGuidsStorage: redisStorage,
		localCache:      map[uint32][GuidTypeMax]*AvailableDiapasons{},
	}

	for _, realmID := range realmIDs {
		// Inits characters max guids in redis if needed.
		max, err := redisStorage.MaxGuidForCharacters(ctx, realmID)
		if err != nil {
			return nil, err
		}

		if max == 0 {
			max, err = mysql.MaxGuidForCharacters(ctx, realmID)
			if err != nil {
				return nil, err
			}

			err = redisStorage.SetMaxGuidForCharacters(ctx, realmID, max)
			if err != nil {
				return nil, err
			}
		}

		// Inits items max guids in redis if needed.
		max, err = redisStorage.MaxGuidForItems(ctx, realmID)
		if err != nil {
			return nil, err
		}

		if max == 0 {
			max, err = mysql.MaxGuidForItems(ctx, realmID)
			if err != nil {
				return nil, err
			}

			err = redisStorage.SetMaxGuidForItems(ctx, realmID, max)
			if err != nil {
				return nil, err
			}
		}

		caches := [GuidTypeMax]*AvailableDiapasons{}
		for i := range service.localCache[realmID] {
			caches[i] = &AvailableDiapasons{}
		}
		service.localCache[realmID] = caches
	}

	service.startProcessingGoroutines(ctx, workersCount)

	return service, nil
}

func (g *guidServiceImpl) GetGuids(ctx context.Context, realmID uint32, guidType uint8, desiredSize uint64) ([]GuidDiapason, error) {
	availableGuidsWithTypes, found := g.localCache[realmID]
	if !found {
		return nil, fmt.Errorf("realmID %d not found", realmID)
	}

	availableGuids := availableGuidsWithTypes[guidType]

	pctUsed := availableGuids.PctUsage()
	switch {
	case pctUsed >= 99:
		waitCh := make(chan struct{})
		g.requestChan <- &guidsRequest{
			realmID:       realmID,
			guidType:      GuidType(guidType),
			desiredAmount: desiredSize * uint64(RequestGuidsBufferMultiplayer),
			callback:      waitCh,
		}
		<-waitCh
		return g.GetGuids(ctx, realmID, guidType, desiredSize)
	case pctUsed >= 70:
		g.requestChan <- &guidsRequest{
			realmID:       realmID,
			guidType:      GuidType(guidType),
			desiredAmount: desiredSize * uint64(RequestGuidsBufferMultiplayer),
		}
	}

	diapasons := availableGuids.UseGuids(desiredSize)
	// Some goroutine already stole guids. Retry.
	if len(diapasons) == 0 {
		return g.GetGuids(ctx, realmID, guidType, desiredSize)
	}

	return diapasons, nil
}

type guidsRequest struct {
	realmID       uint32
	guidType      GuidType
	desiredAmount uint64
	callback      chan<- struct{}
}

func (r *guidsRequest) key() string {
	return fmt.Sprintf("%d:%d:%d", r.realmID, r.guidType, r.desiredAmount)
}

type guidsResponse struct {
	request guidsRequest

	newGuids GuidDiapason
}

func (g *guidServiceImpl) startProcessingGoroutines(ctx context.Context, processorsCount int) {
	requestsChan := make(chan guidsRequest, 1000)
	responseChan := make(chan guidsResponse, 1000)
	for i := 0; i < processorsCount; i++ {
		go g.requestProcessor(requestsChan, responseChan)
	}

	requestsWithRespInProcess := map[string]*guidsRequest{}
	callbacksQueue := map[string][]chan<- struct{}{}

	requestsChanMiddleware := make(chan *guidsRequest, 1000)
	go func() {
		var key string
		for {
			select {
			case r := <-requestsChanMiddleware:
				key = r.key()
				request := requestsWithRespInProcess[key]
				if request != nil {
					if r.callback != nil {
						callbacksQueue[key] = append(callbacksQueue[key], r.callback)
					}
					break
				}

				requestsWithRespInProcess[key] = r
				requestsChan <- *r

			case r := <-responseChan:
				key = r.request.key()

				g.localCache[r.request.realmID][r.request.guidType].AddDiapason(r.newGuids)

				if r.request.callback != nil {
					r.request.callback <- struct{}{}
				}

				for _, callback := range callbacksQueue[key] {
					callback <- struct{}{}
					close(callback)
				}

				delete(callbacksQueue, key)
				delete(requestsWithRespInProcess, key)
			case <-ctx.Done():
				close(requestsChan)
				close(requestsChanMiddleware)
				return
			}
		}
	}()
	g.requestChan = requestsChanMiddleware
}

func (g *guidServiceImpl) requestProcessor(requests <-chan guidsRequest, response chan<- guidsResponse) {
	for r := range requests {
		switch r.guidType {
		case GuidTypeCharacter:
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			newMax, err := g.maxGuidsStorage.IncreaseMaxGuidForCharacters(ctx, r.realmID, r.desiredAmount)
			cancel()
			if err != nil {
				log.Err(err).Msg("can't increase characters guid")
				response <- guidsResponse{
					request:  r,
					newGuids: GuidDiapason{},
				}
				continue
			}

			response <- guidsResponse{
				request: r,
				newGuids: GuidDiapason{
					Start: newMax - r.desiredAmount + 1,
					End:   newMax,
				},
			}

		case GuidTypeItem:
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			newMax, err := g.maxGuidsStorage.IncreaseMaxGuidForItems(ctx, r.realmID, r.desiredAmount)
			cancel()
			if err != nil {
				log.Err(err).Msg("can't increase items guid")
				continue
			}

			response <- guidsResponse{
				request: r,
				newGuids: GuidDiapason{
					Start: newMax - r.desiredAmount + 1,
					End:   newMax,
				},
			}
		default:
			log.Err(fmt.Errorf("unk guid type: %d", r.guidType))
		}
	}
}

func (g *guidServiceImpl) requestNewDiapason() {
	time.Sleep(time.Second * 300)
}
