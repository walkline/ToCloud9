package service

import "sync"

type GuidDiapason struct {
	Start uint64
	End   uint64
}

type GuidDiapasonWithState struct {
	GuidDiapason
	CurrentGuid uint64
}

func (d *GuidDiapasonWithState) GuidsLeft() uint64 {
	if d.CurrentGuid > d.End {
		return 0
	}
	return d.End - d.CurrentGuid + 1
}

func (d *GuidDiapasonWithState) TotalCount() uint64 {
	return d.End - d.Start + 1
}

type AvailableDiapasons struct {
	Diapasons []GuidDiapasonWithState
	lock      sync.RWMutex
}

func (d *AvailableDiapasons) PctUsage() float32 {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if len(d.Diapasons) == 0 {
		return 100
	}

	totalCount, totalLeft := uint64(0), uint64(0)
	for _, diapason := range d.Diapasons {
		totalCount += diapason.TotalCount()
		totalLeft += diapason.GuidsLeft()
	}
	return 100 - float32(float64(totalLeft)/(float64(totalCount)/100))
}

func (d *AvailableDiapasons) UseGuids(guidsAmount uint64) []GuidDiapason {
	diapasonsToReturn := []GuidDiapason{}
	remainingGuidsToRequest := guidsAmount

	d.lock.Lock()
	defer d.lock.Unlock()

	defer d.CleanupEmpty()

	for i, diapason := range d.Diapasons {
		leftInDiapason := diapason.GuidsLeft()
		// Handle case when we can fulfill request.
		if remainingGuidsToRequest <= leftInDiapason {
			diapasonsToReturn = append(diapasonsToReturn, GuidDiapason{
				Start: diapason.CurrentGuid,
				End:   diapason.CurrentGuid - 1 + remainingGuidsToRequest,
			})

			d.Diapasons[i].CurrentGuid += remainingGuidsToRequest
			return diapasonsToReturn
		}

		// Handle case when we need to drain all left guids in diapason.
		remainingGuidsToRequest -= leftInDiapason
		diapasonsToReturn = append(diapasonsToReturn, GuidDiapason{
			Start: diapason.CurrentGuid,
			End:   diapason.End,
		})

		diapason.CurrentGuid = diapason.End
	}

	return diapasonsToReturn
}

func (d *AvailableDiapasons) AddDiapason(diapason GuidDiapason) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if len(d.Diapasons) > 0 {
		if d.Diapasons[len(d.Diapasons)-1].End == diapason.Start-1 {
			d.Diapasons[len(d.Diapasons)-1].End = diapason.End
			return
		}
	}

	d.Diapasons = append(d.Diapasons, GuidDiapasonWithState{GuidDiapason: diapason, CurrentGuid: diapason.Start})
}

func (d *AvailableDiapasons) CleanupEmpty() {
	for i := range d.Diapasons {
		if d.Diapasons[i].GuidsLeft() != 0 {
			d.Diapasons = d.Diapasons[i:]
			return
		}
	}
	d.Diapasons = []GuidDiapasonWithState{}
}
