package service

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"unsafe"

	"github.com/o0olele/detour-go/detour"
)

const (
	mmapMagic   = 0x4d4d4150 // 'MMAP'
	mmapVersion = 19

	// mmapTileHeaderSize is the binary size of the MmapTileHeader struct (C++ layout).
	// uint32 mmapMagic + uint32 dtVersion + uint32 mmapVersion + uint32 size + int8 usesLiquids + 3 padding + 36 bytes recastConfig = 56
	mmapTileHeaderSize = 56
)

// MMapManager manages lazy loading of navigation meshes and their tiles.
// It is safe for concurrent use.
type MMapManager struct {
	mmapsDir string

	mu       sync.RWMutex
	meshes   map[uint32]*mapData // mapID -> mapData
}

type mapData struct {
	mu          sync.Mutex
	navMesh     *detour.DtNavMesh
	loadedTiles map[uint32]bool // packed(x,y) -> loaded
}

func packTileID(x, y int32) uint32 {
	return uint32(x)<<16 | uint32(y)&0xFFFF
}

// NewMMapManager creates a new MMapManager that reads files from mmapsDir.
func NewMMapManager(mmapsDir string) *MMapManager {
	return &MMapManager{
		mmapsDir: mmapsDir,
		meshes:   make(map[uint32]*mapData),
	}
}

// GetNavMesh returns the nav mesh for the given map, loading it lazily if needed.
func (m *MMapManager) GetNavMesh(mapID uint32) (*detour.DtNavMesh, error) {
	md, err := m.getOrLoadMapData(mapID)
	if err != nil {
		return nil, err
	}
	return md.navMesh, nil
}

// EnsureTileLoaded ensures the tile at (x, y) for the given map is loaded.
func (m *MMapManager) EnsureTileLoaded(mapID uint32, x, y int32) error {
	md, err := m.getOrLoadMapData(mapID)
	if err != nil {
		return err
	}
	return m.loadTile(md, mapID, x, y)
}

// CreateQuery creates a new DtNavMeshQuery for the given map.
// Each query instance is NOT thread-safe and should be used from a single goroutine.
func (m *MMapManager) CreateQuery(mapID uint32) (*detour.DtNavMeshQuery, error) {
	md, err := m.getOrLoadMapData(mapID)
	if err != nil {
		return nil, err
	}

	query := detour.DtAllocNavMeshQuery()
	if status := query.Init(md.navMesh, 1024); detour.DtStatusFailed(status) {
		return nil, fmt.Errorf("failed to init NavMeshQuery for map %d, status: %d", mapID, status)
	}
	return query, nil
}

func (m *MMapManager) getOrLoadMapData(mapID uint32) (*mapData, error) {
	m.mu.RLock()
	md, ok := m.meshes[mapID]
	m.mu.RUnlock()
	if ok {
		return md, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock.
	md, ok = m.meshes[mapID]
	if ok {
		return md, nil
	}

	navMesh, err := m.loadNavMesh(mapID)
	if err != nil {
		return nil, err
	}

	md = &mapData{
		navMesh:     navMesh,
		loadedTiles: make(map[uint32]bool),
	}
	m.meshes[mapID] = md
	return md, nil
}

func (m *MMapManager) loadNavMesh(mapID uint32) (*detour.DtNavMesh, error) {
	fileName := fmt.Sprintf("%s/%03d.mmap", m.mmapsDir, mapID)
	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("could not open mmap file '%s': %w", fileName, err)
	}
	defer file.Close()

	// DtNavMeshParams binary layout: orig[3]float32, tileWidth float32, tileHeight float32, maxTiles uint32, maxPolys uint32 = 28 bytes
	var params detour.DtNavMeshParams
	if err := binary.Read(file, binary.LittleEndian, &params.Orig); err != nil {
		return nil, fmt.Errorf("could not read params from '%s': %w", fileName, err)
	}
	if err := binary.Read(file, binary.LittleEndian, &params.TileWidth); err != nil {
		return nil, fmt.Errorf("could not read params from '%s': %w", fileName, err)
	}
	if err := binary.Read(file, binary.LittleEndian, &params.TileHeight); err != nil {
		return nil, fmt.Errorf("could not read params from '%s': %w", fileName, err)
	}
	if err := binary.Read(file, binary.LittleEndian, &params.MaxTiles); err != nil {
		return nil, fmt.Errorf("could not read params from '%s': %w", fileName, err)
	}
	if err := binary.Read(file, binary.LittleEndian, &params.MaxPolys); err != nil {
		return nil, fmt.Errorf("could not read params from '%s': %w", fileName, err)
	}

	navMesh := detour.DtAllocNavMesh()
	if status := navMesh.Init(&params); detour.DtStatusFailed(status) {
		return nil, fmt.Errorf("failed to initialize dtNavMesh for map %03d, status: %d", mapID, status)
	}

	return navMesh, nil
}

func (m *MMapManager) loadTile(md *mapData, mapID uint32, x, y int32) error {
	packed := packTileID(x, y)

	md.mu.Lock()
	defer md.mu.Unlock()

	if md.loadedTiles[packed] {
		return nil
	}

	fileName := fmt.Sprintf("%s/%03d%02d%02d.mmtile", m.mmapsDir, mapID, x, y)
	file, err := os.Open(fileName)
	if err != nil {
		// Tile file doesn't exist; this is not necessarily an error.
		return nil
	}
	defer file.Close()

	// Read the MmapTileHeader.
	headerBuf := make([]byte, mmapTileHeaderSize)
	if _, err := io.ReadFull(file, headerBuf); err != nil {
		return fmt.Errorf("could not read header from '%s': %w", fileName, err)
	}

	magic := binary.LittleEndian.Uint32(headerBuf[0:4])
	if magic != mmapMagic {
		return fmt.Errorf("bad magic in '%s': 0x%08x", fileName, magic)
	}

	version := binary.LittleEndian.Uint32(headerBuf[8:12])
	if version != mmapVersion {
		return fmt.Errorf("version mismatch in '%s': got %d, expected %d", fileName, version, mmapVersion)
	}

	dataSize := binary.LittleEndian.Uint32(headerBuf[12:16])

	data := make([]byte, dataSize)
	if _, err := io.ReadFull(file, data); err != nil {
		return fmt.Errorf("could not read tile data from '%s': %w", fileName, err)
	}

	// Verify dtNavMesh header magic.
	dtMeshHeaderSize := int(unsafe.Sizeof(detour.DtMeshHeader{}))
	if int(dataSize) < dtMeshHeaderSize {
		return fmt.Errorf("tile data too small in '%s'", fileName)
	}

	var tileRef detour.DtTileRef
	if status := md.navMesh.AddTile(data, int(dataSize), detour.DT_TILE_FREE_DATA, 0, &tileRef); detour.DtStatusFailed(status) {
		return fmt.Errorf("could not add tile %03d[%02d,%02d] to navmesh, status: %d", mapID, x, y, status)
	}

	md.loadedTiles[packed] = true
	return nil
}