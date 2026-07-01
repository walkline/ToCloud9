package service

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
)

const (
	mapResolution = 128 // MAP_RESOLUTION

	// Map file magic and version.
	mapMagicUint   = uint32(0x5350414d) // 'MAPS'
	mapVersion     = uint32(9)
	mapHeightMagic = uint32(0x5447484d) // 'MHGT'
	mapAreaMagic   = uint32(0x41455241) // 'AREA'
	mapLiquidMagic = uint32(0x51494c4d) // 'MLIQ'

	// Height flags.
	mapHeightNoHeight = 0x0001
	mapHeightAsInt16  = 0x0002
	mapHeightAsInt8   = 0x0004

	invalidHeight = -100000.0
)

// TerrainManager manages lazy loading of .map terrain data for height queries.
// It is safe for concurrent use.
type TerrainManager struct {
	mapsDir string

	mu    sync.RWMutex
	grids map[uint64]*gridTerrain // key: (mapID<<32) | (gx<<16|gy)  or per map then tile
}

type gridTerrain struct {
	mu sync.RWMutex

	// height data
	gridHeight float32

	// flat
	hasHeight bool

	// float data
	v9f []float32 // 129*129
	v8f []float32 // 128*128

	// uint16
	v9u16 []uint16
	v8u16 []uint16
	u16Mul float32

	// uint8
	v9u8 []uint8
	v8u8 []uint8
	u8Mul float32

	flags uint32 // height header flags
}

func packGridKey(mapID uint32, gx, gy int) uint64 {
	return (uint64(mapID) << 32) | (uint64(uint16(gx))<<16) | uint64(uint16(gy))
}

// NewTerrainManager creates a TerrainManager reading .map files from mapsDir.
func NewTerrainManager(mapsDir string) *TerrainManager {
	return &TerrainManager{
		mapsDir: mapsDir,
		grids:   make(map[uint64]*gridTerrain),
	}
}

// GetHeight returns the terrain height at (x, y) using the same interpolation as AzerothCore GridTerrainData.
// Matches AC: only returns the grid height if the search z is not below it by more than tolerance.
func (tm *TerrainManager) GetHeight(mapID uint32, x, y, z float32) (float32, bool) {
	gx, gy := gameToGridCoords(x, y)
	if gx < 0 || gy < 0 || gx >= 64 || gy >= 64 {
		return 0, false
	}

	gt, err := tm.getOrLoadGrid(mapID, gx, gy)
	if err != nil || gt == nil {
		return 0, false
	}

	h := gt.getHeight(x, y)
	if h <= invalidHeight+1 { // treat as invalid
		return 0, false
	}
	if !fuzzyGe(z, h - GROUND_HEIGHT_TOLERANCE) {
		return 0, false
	}
	return h, true
}

func (tm *TerrainManager) getOrLoadGrid(mapID uint32, gx, gy int) (*gridTerrain, error) {
	key := packGridKey(mapID, gx, gy)

	tm.mu.RLock()
	g, ok := tm.grids[key]
	tm.mu.RUnlock()
	if ok {
		return g, nil
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if g, ok = tm.grids[key]; ok {
		return g, nil
	}

	gt, err := tm.loadGrid(mapID, gx, gy)
	if err != nil {
		// Cache negative? For now, don't cache failures to allow retries if files appear.
		return nil, err
	}
	tm.grids[key] = gt
	return gt, nil
}

func (tm *TerrainManager) loadGrid(mapID uint32, gx, gy int) (*gridTerrain, error) {
	fileName := fmt.Sprintf("%s/%03d%02d%02d.map", tm.mapsDir, mapID, gx, gy)
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read file header (10 uint32)
	var header [10]uint32
	if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("read map header: %w", err)
	}
	if header[0] != mapMagicUint || header[1] != mapVersion {
		return nil, fmt.Errorf("bad map magic/version in %s", fileName)
	}

	heightOffset := header[5]
	heightSize := header[6]

	if heightOffset == 0 || heightSize == 0 {
		// no height data: flat at gridHeight? but header not read yet. Treat as no height.
		gt := &gridTerrain{hasHeight: false, gridHeight: 0}
		// We still try to read height header if offset present? per AC, if offset but NO_HEIGHT flag.
		// For simplicity, return flat 0 if no height map; callers will see invalid.
		return gt, nil
	}

	if _, err := f.Seek(int64(heightOffset), io.SeekStart); err != nil {
		return nil, err
	}

	// height header: fourcc, flags, gridHeight, gridMaxHeight
	var hhFourCC, hhFlags uint32
	var gridHeight, gridMaxHeight float32
	if err := binary.Read(f, binary.LittleEndian, &hhFourCC); err != nil {
		return nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &hhFlags); err != nil {
		return nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &gridHeight); err != nil {
		return nil, err
	}
	if err := binary.Read(f, binary.LittleEndian, &gridMaxHeight); err != nil {
		return nil, err
	}
	if hhFourCC != mapHeightMagic {
		return nil, fmt.Errorf("bad height magic in %s", fileName)
	}

	gt := &gridTerrain{
		gridHeight: gridHeight,
		flags:      hhFlags,
		hasHeight:  (hhFlags & mapHeightNoHeight) == 0,
	}

	if !gt.hasHeight {
		return gt, nil
	}

	switch {
	case (hhFlags & mapHeightAsInt16) != 0:
		// v9: 129*129 uint16, v8: 128*128 uint16
		v9 := make([]uint16, 129*129)
		v8 := make([]uint16, 128*128)
		if err := binary.Read(f, binary.LittleEndian, &v9); err != nil {
			return nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &v8); err != nil {
			return nil, err
		}
		gt.v9u16 = v9
		gt.v8u16 = v8
		if gridMaxHeight > gridHeight {
			gt.u16Mul = (gridMaxHeight - gridHeight) / 65535.0
		}
	case (hhFlags & mapHeightAsInt8) != 0:
		v9 := make([]uint8, 129*129)
		v8 := make([]uint8, 128*128)
		if err := binary.Read(f, binary.LittleEndian, &v9); err != nil {
			return nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &v8); err != nil {
			return nil, err
		}
		gt.v9u8 = v9
		gt.v8u8 = v8
		if gridMaxHeight > gridHeight {
			gt.u8Mul = (gridMaxHeight - gridHeight) / 255.0
		}
	default:
		// float
		v9 := make([]float32, 129*129)
		v8 := make([]float32, 128*128)
		if err := binary.Read(f, binary.LittleEndian, &v9); err != nil {
			return nil, err
		}
		if err := binary.Read(f, binary.LittleEndian, &v8); err != nil {
			return nil, err
		}
		gt.v9f = v9
		gt.v8f = v8
	}

	return gt, nil
}

func (gt *gridTerrain) getHeight(x, y float32) float32 {
	if !gt.hasHeight {
		return gt.gridHeight
	}
	h := float32(0)
	switch {
	case gt.v9f != nil:
		h = gt.getHeightFromFloat(x, y)
	case gt.v9u16 != nil:
		h = gt.getHeightFromUint16(x, y)
	case gt.v9u8 != nil:
		h = gt.getHeightFromUint8(x, y)
	default:
		h = gt.gridHeight
	}
	if math.Abs(float64(x-1568)) < 2 && math.Abs(float64(y+4405.87)) < 2 {
		// best effort, import math may not be here; use a side log via fmt if needed, but to avoid, just compute
	}
	_ = h
	return h
}

func (gt *gridTerrain) getHeightFromFloat(x, y float32) float32 {
	x = float32(mapResolution) * (32 - x/sizeOfGrids)
	y = float32(mapResolution) * (32 - y/sizeOfGrids)

	xInt := int(x)
	yInt := int(y)
	x -= float32(xInt)
	y -= float32(yInt)
	xInt &= (mapResolution - 1)
	yInt &= (mapResolution - 1)

	// hole check omitted for speed (holes affect navmesh already); if needed, implement isHole.

	if x+y < 1 {
		if x > y {
			// tri 1: h1,h2,h5
			h1 := gt.v9f[xInt*129+yInt]
			h2 := gt.v9f[(xInt+1)*129+yInt]
			h5 := 2 * gt.v8f[xInt*128+yInt]
			a := h2 - h1
			b := h5 - h1 - h2
			c := h1
			return a*x + b*y + c
		}
		// tri 2: h1,h3,h5
		h1 := gt.v9f[xInt*129+yInt]
		h3 := gt.v9f[xInt*129+yInt+1]
		h5 := 2 * gt.v8f[xInt*128+yInt]
		a := h5 - h1 - h3
		b := h3 - h1
		c := h1
		return a*x + b*y + c
	}
	if x > y {
		// tri 3: h2,h4,h5
		h2 := gt.v9f[(xInt+1)*129+yInt]
		h4 := gt.v9f[(xInt+1)*129+yInt+1]
		h5 := 2 * gt.v8f[xInt*128+yInt]
		a := h2 + h4 - h5
		b := h4 - h2
		c := h5 - h4
		return a*x + b*y + c
	}
	// tri 4: h3,h4,h5
	h3 := gt.v9f[xInt*129+yInt+1]
	h4 := gt.v9f[(xInt+1)*129+yInt+1]
	h5 := 2 * gt.v8f[xInt*128+yInt]
	a := h4 - h3
	b := h3 + h4 - h5
	c := h5 - h4
	return a*x + b*y + c
}

func (gt *gridTerrain) getHeightFromUint16(x, y float32) float32 {
	x = float32(mapResolution) * (32 - x/sizeOfGrids)
	y = float32(mapResolution) * (32 - y/sizeOfGrids)

	xInt := int(x)
	yInt := int(y)
	x -= float32(xInt)
	y -= float32(yInt)
	xInt &= (mapResolution - 1)
	yInt &= (mapResolution - 1)

	// v9 layout matches AC indexing: x_int*129 + y_int
	v9Base := xInt*129 + yInt

	if x+y < 1 {
		if x > y {
			h1 := float32(gt.v9u16[v9Base+0])
			h2 := float32(gt.v9u16[v9Base+129])
			h5 := 2 * float32(gt.v8u16[xInt*128+yInt])
			a := h2 - h1
			b := h5 - h1 - h2
			c := h1
			res := (a*x + b*y + c) * gt.u16Mul + gt.gridHeight
			return res
		}
		h1 := float32(gt.v9u16[v9Base+0])
		h3 := float32(gt.v9u16[v9Base+1])
		h5 := 2 * float32(gt.v8u16[xInt*128+yInt])
		a := h5 - h1 - h3
		b := h3 - h1
		c := h1
		return (a*x + b*y + c) * gt.u16Mul + gt.gridHeight
	}
	if x > y {
		h2 := float32(gt.v9u16[v9Base+129])
		h4 := float32(gt.v9u16[v9Base+130])
		h5 := 2 * float32(gt.v8u16[xInt*128+yInt])
		a := h2 + h4 - h5
		b := h4 - h2
		c := h5 - h4
		return (a*x + b*y + c) * gt.u16Mul + gt.gridHeight
	}
	h3 := float32(gt.v9u16[v9Base+1])
	h4 := float32(gt.v9u16[v9Base+130])
	h5 := 2 * float32(gt.v8u16[xInt*128+yInt])
	a := h4 - h3
	b := h3 + h4 - h5
	c := h5 - h4
	return (a*x + b*y + c) * gt.u16Mul + gt.gridHeight
}

func (gt *gridTerrain) getHeightFromUint8(x, y float32) float32 {
	x = float32(mapResolution) * (32 - x/sizeOfGrids)
	y = float32(mapResolution) * (32 - y/sizeOfGrids)

	xInt := int(x)
	yInt := int(y)
	x -= float32(xInt)
	y -= float32(yInt)
	xInt &= (mapResolution - 1)
	yInt &= (mapResolution - 1)

	// AC: uint8* V9_h1_ptr = &_...v9[x_int * 128 + x_int + y_int];
	v9Base := xInt*129 + yInt

	if x+y < 1 {
		if x > y {
			h1 := float32(gt.v9u8[v9Base+0])
			h2 := float32(gt.v9u8[v9Base+129])
			h5 := 2 * float32(gt.v8u8[xInt*128+yInt])
			a := h2 - h1
			b := h5 - h1 - h2
			c := h1
			return (a*x + b*y + c) * gt.u8Mul + gt.gridHeight
		}
		h1 := float32(gt.v9u8[v9Base+0])
		h3 := float32(gt.v9u8[v9Base+1])
		h5 := 2 * float32(gt.v8u8[xInt*128+yInt])
		a := h5 - h1 - h3
		b := h3 - h1
		c := h1
		return (a*x + b*y + c) * gt.u8Mul + gt.gridHeight
	}
	if x > y {
		h2 := float32(gt.v9u8[v9Base+129])
		h4 := float32(gt.v9u8[v9Base+130])
		h5 := 2 * float32(gt.v8u8[xInt*128+yInt])
		a := h2 + h4 - h5
		b := h4 - h2
		c := h5 - h4
		return (a*x + b*y + c) * gt.u8Mul + gt.gridHeight
	}
	h3 := float32(gt.v9u8[v9Base+1])
	h4 := float32(gt.v9u8[v9Base+130])
	h5 := 2 * float32(gt.v8u8[xInt*128+yInt])
	a := h4 - h3
	b := h3 + h4 - h5
	c := h5 - h4
	return (a*x + b*y + c) * gt.u8Mul + gt.gridHeight
}

// GetLiquidStatus is a simplified stub; full liquid not required for basic Z alignment now.
func (tm *TerrainManager) GetLiquidStatus(mapID uint32, x, y, z float32) uint8 { return 0 }

// GetGridHeight returns the raw interpolated grid height (no z check).
func (tm *TerrainManager) GetGridHeight(mapID uint32, x, y float32) (float32, bool) {
	gx, gy := gameToGridCoords(x, y)
	if gx < 0 || gy < 0 || gx >= 64 || gy >= 64 {
		return 0, false
	}
	gt, err := tm.getOrLoadGrid(mapID, gx, gy)
	if err != nil || gt == nil {
		return 0, false
	}
	if !gt.hasHeight {
		return gt.gridHeight, true
	}
	h := gt.getHeight(x, y)
	if h <= invalidHeight+1 {
		return 0, false
	}
	if mapID == 1 && math.Abs(float64(x-1568)) < 5 {
		// fmt not directly, but we can ignore for now or use a side effect; skip detailed here to avoid import cycle risk in edit
	}
	return h, true
}

// EnsureGridLoaded forces load for a grid (useful for preloading).
func (tm *TerrainManager) EnsureGridLoaded(mapID uint32, gx, gy int32) error {
	_, err := tm.getOrLoadGrid(mapID, int(gx), int(gy))
	return err
}


