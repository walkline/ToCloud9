package service

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
)

// VMAP support for accurate height queries matching AzerothCore.
// We implement enough to support GetHeight (downward ray for ground Z).
// Uses BIH trees and model instances from .vmtree / .vmtile + .vmo models.

// Constants from AC
const (
	VMAP_MAGIC     = "VMAP_3.0"
	INVALID_HEIGHT = -100000.0
	VMAP_mid       = 32.0 * 533.3333 // for internal <-> game conversion used inside vmaps
)

// Vector3 simple
type Vector3 struct {
	X, Y, Z float32
}

func (v Vector3) Sub(o Vector3) Vector3 {
	return Vector3{v.X - o.X, v.Y - o.Y, v.Z - o.Z}
}

func (v Vector3) Add(o Vector3) Vector3 {
	return Vector3{v.X + o.X, v.Y + o.Y, v.Z + o.Z}
}

func (v Vector3) Mul(s float32) Vector3 {
	return Vector3{v.X * s, v.Y * s, v.Z * s}
}

func (v Vector3) Dot(o Vector3) float32 {
	return v.X*o.X + v.Y*o.Y + v.Z*o.Z
}

func (v Vector3) Cross(o Vector3) Vector3 {
	return Vector3{
		v.Y*o.Z - v.Z*o.Y,
		v.Z*o.X - v.X*o.Z,
		v.X*o.Y - v.Y*o.X,
	}
}

// Ray
type Ray struct {
	Origin, Direction Vector3
}

func NewRay(origin, dir Vector3) Ray {
	return Ray{Origin: origin, Direction: dir}
}

// AABox
type AABox struct {
	Low, High Vector3
}

func (b AABox) IntersectRay(ray Ray, maxDist *float32) bool {
	// slab method simplified for our use
	tmin := float32(0)
	tmax := *maxDist
	for i := 0; i < 3; i++ {
		var t1, t2 float32
		switch i {
		case 0:
			t1 = (b.Low.X - ray.Origin.X) / ray.Direction.X
			t2 = (b.High.X - ray.Origin.X) / ray.Direction.X
		case 1:
			t1 = (b.Low.Y - ray.Origin.Y) / ray.Direction.Y
			t2 = (b.High.Y - ray.Origin.Y) / ray.Direction.Y
		case 2:
			t1 = (b.Low.Z - ray.Origin.Z) / ray.Direction.Z
			t2 = (b.High.Z - ray.Origin.Z) / ray.Direction.Z
		}
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
		}
		if t2 < tmax {
			tmax = t2
		}
		if tmin > tmax {
			return false
		}
	}
	*maxDist = tmax
	return true
}

// BIH node encoding (from AC)
const (
	BIH_LEAF_MASK = 3 << 30
	BIH_BVH2_MASK = 1 << 29
)

// BIH simple port for ray intersect (for height we use downward)
type BIH struct {
	bounds AABox
	tree   []uint32
	objs   []uint32
}

func (b *BIH) readFromFile(f *os.File) error {
	return b.readFrom(f)
}

func (b *BIH) readFrom(r io.Reader) error {
	var lo, hi [3]float32
	if err := binary.Read(r, binary.LittleEndian, &lo); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &hi); err != nil {
		return err
	}
	b.bounds = AABox{Low: Vector3{lo[0], lo[1], lo[2]}, High: Vector3{hi[0], hi[1], hi[2]} }

	var treeSize uint32
	if err := binary.Read(r, binary.LittleEndian, &treeSize); err != nil {
		return err
	}
	b.tree = make([]uint32, treeSize)
	if err := binary.Read(r, binary.LittleEndian, &b.tree); err != nil {
		return err
	}
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return err
	}
	b.objs = make([]uint32, count)
	if err := binary.Read(r, binary.LittleEndian, &b.objs); err != nil {
		return err
	}
	return nil
}

// intersectRay for height: we want the smallest positive t for hit in negative Z dir.
func (b *BIH) intersectRay(ray Ray, maxDist *float32, stopAtFirst bool) bool {
	// Simplified stack based traversal ported from AC BIH
	// For downward rays we can optimize but keep general for correctness.
	invDir := Vector3{1 / ray.Direction.X, 1 / ray.Direction.Y, 1 / ray.Direction.Z}
	org := ray.Origin

	intervalMin := float32(0)
	intervalMax := *maxDist

	for i := 0; i < 3; i++ {
		var t1, t2 float32
		switch i {
		case 0:
			t1 = (b.bounds.Low.X - org.X) * invDir.X
			t2 = (b.bounds.High.X - org.X) * invDir.X
		case 1:
			t1 = (b.bounds.Low.Y - org.Y) * invDir.Y
			t2 = (b.bounds.High.Y - org.Y) * invDir.Y
		case 2:
			t1 = (b.bounds.Low.Z - org.Z) * invDir.Z
			t2 = (b.bounds.High.Z - org.Z) * invDir.Z
		}
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > intervalMin {
			intervalMin = t1
		}
		if t2 < intervalMax {
			intervalMax = t2
		}
		if intervalMax <= 0 || intervalMin >= *maxDist {
			return false
		}
	}
	if intervalMin > intervalMax {
		return false
	}
	intervalMin = max32(intervalMin, 0)
	intervalMax = min32(intervalMax, *maxDist)

	// offsets for signs
	offsetFront := [3]uint32{}
	offsetBack := [3]uint32{}
	for i := 0; i < 3; i++ {
		dirI := ray.Direction.X
		if i == 1 {
			dirI = ray.Direction.Y
		} else if i == 2 {
			dirI = ray.Direction.Z
		}
		bits := floatToRawIntBits(dirI) >> 31
		offsetFront[i] = bits
		offsetBack[i] = bits ^ 1
		offsetFront[i]++
		offsetBack[i]++
	}

	type stackNode struct {
		node int
		tmin float32
		tmax float32
	}
	stack := make([]stackNode, 0, 64)
	stack = append(stack, stackNode{0, intervalMin, intervalMax})

	hit := false
	for len(stack) > 0 {
		sn := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := sn.node
		tmin := sn.tmin
		tmax := sn.tmax

		for {
			if node >= len(b.tree) {
				break
			}
			tn := b.tree[node]
			axis := (tn & (3 << 30)) >> 30
			bvh2 := (tn & (1 << 29)) != 0
			offset := int(tn & ^(uint32(7) << 29))

			if !bvh2 {
				if axis < 3 {
					// interior
					tf := intBitsToFloat(b.tree[node+int(offsetFront[axis])]) - getCoord(org, axis)
					tf *= getInv(invDir, axis)
					tb := intBitsToFloat(b.tree[node+int(offsetBack[axis])]) - getCoord(org, axis)
					tb *= getInv(invDir, axis)

					if tf < tmin && tb > tmax {
						break
					}
					back := offset + int(offsetBack[axis])*3
					// push back if needed
					if tb >= tmin && tb <= tmax {
						stack = append(stack, stackNode{back, tmin, min32(tmax, tb)})
					}
					if tf <= tmax && tf >= tmin {
						node = offset + int(offsetFront[axis])*3
						tmax = min32(tmax, tf)
						continue
					}
					break
				} else {
					// leaf
					primStart := offset
					primCount := int(tn & 0x1FFFFFFF) // approx, actual encoding
					// For our simplified, we will handle in caller via objects
					// To make height work, we fall to objects for now.
					// Full impl would intersect primitives here.
					for j := 0; j < primCount && primStart+j < len(b.objs); j++ {
						// caller will use objects[b.objs[primStart+j]]
						_ = b.objs[primStart+j]
					}
					break
				}
			} else {
				// BVH2 node simplified handling
				break
			}
		}
	}
	return hit
}

// Helpers for bits
func floatToRawIntBits(f float32) uint32 {
	bits := math.Float32bits(f)
	return bits
}

func intBitsToFloat(i uint32) float32 {
	return math.Float32frombits(i)
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func getCoord(v Vector3, axis uint32) float32 {
	switch axis {
	case 0:
		return v.X
	case 1:
		return v.Y
	default:
		return v.Z
	}
}

func getInv(v Vector3, axis uint32) float32 {
	switch axis {
	case 0:
		return v.X
	case 1:
		return v.Y
	default:
		return v.Z
	}
}

// ModelSpawn
type ModelSpawn struct {
	Flags  uint32
	ADTId  uint16
	ID     uint32
	iPos   Vector3
	iRot   Vector3
	iScale float32
	iBound [2]Vector3 // low,high if MOD_HAS_BOUND
	name   string
}

func (s *ModelSpawn) readFromFile(f *os.File) error {
	return s.readFrom(f)
}

func (s *ModelSpawn) readFrom(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &s.Flags); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &s.ADTId); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &s.ID); err != nil {
		return err
	}
	var pos [3]float32
	if err := binary.Read(r, binary.LittleEndian, &pos); err != nil {
		return err
	}
	s.iPos = Vector3{pos[0], pos[1], pos[2]}
	var rot [3]float32
	if err := binary.Read(r, binary.LittleEndian, &rot); err != nil {
		return err
	}
	s.iRot = Vector3{rot[0], rot[1], rot[2]}
	if err := binary.Read(r, binary.LittleEndian, &s.iScale); err != nil {
		return err
	}
	hasBound := (s.Flags & 0x4) != 0 // MOD_HAS_BOUND
	if hasBound {
		var lo, hi [3]float32
		if err := binary.Read(r, binary.LittleEndian, &lo); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &hi); err != nil {
			return err
		}
		// vmap bounds are in internal rep too; convert + swap min/max because x' = mid - x (sign flip)
		s.iBound[0] = Vector3{VMAP_mid - hi[0], VMAP_mid - hi[1], lo[2]}
		s.iBound[1] = Vector3{VMAP_mid - lo[0], VMAP_mid - lo[1], hi[2]}
	}
	var nameLen uint32
	if err := binary.Read(r, binary.LittleEndian, &nameLen); err != nil {
		return err
	}
	nameBytes := make([]byte, nameLen)
	if _, err := io.ReadFull(r, nameBytes); err != nil {
		return err
	}
	s.name = string(nameBytes)
	return nil
}

// ModelInstance (simplified for height)
type ModelInstance struct {
	spawn ModelSpawn
	model *WorldModel // loaded on demand
	iPos  Vector3
	iInvRot [3][3]float32 // inverse rotation matrix
	iInvScale float32
}

func (mi *ModelInstance) getWorldModel() *WorldModel {
	return mi.model
}

func (mi *ModelInstance) initTransform() {
	mi.iPos = mi.spawn.iPos
	scale := mi.spawn.iScale
	if scale == 0 {
		scale = 1
	}
	mi.iInvScale = 1 / scale
	mi.iInvRot = inverseRotationMatrix(mi.spawn.iRot)
}

// inverseRotationMatrix builds the inverse of the ZYX euler rotation matrix used by AC (G3D fromEulerAnglesZYX).
// AC: fromEulerAnglesZYX( pi*rot.y/180, pi*rot.x/180, pi*rot.z/180 ).inverse()
// For orthonormal rotation, inverse == transpose.
func inverseRotationMatrix(rot Vector3) [3][3]float32 {
	// Build forward rotation R = Z(y) * Y(x) * X(z) matching G3D, then transpose for inverse.
	ry := rot.Y * (math.Pi / 180.0)
	rx := rot.X * (math.Pi / 180.0)
	rz := rot.Z * (math.Pi / 180.0)

	cz, sz := float32(math.Cos(float64(ry))), float32(math.Sin(float64(ry)))
	cy, sy := float32(math.Cos(float64(rx))), float32(math.Sin(float64(rx)))
	cx, sx := float32(math.Cos(float64(rz))), float32(math.Sin(float64(rz)))

	// kZ (arg0=ry)
	kz := [3][3]float32{
		{cz, -sz, 0},
		{sz, cz, 0},
		{0, 0, 1},
	}
	// kY (arg1=rx)
	ky := [3][3]float32{
		{cy, 0, sy},
		{0, 1, 0},
		{-sy, 0, cy},
	}
	// kX (arg2=rz)
	kx := [3][3]float32{
		{1, 0, 0},
		{0, cx, -sx},
		{0, sx, cx},
	}

	// R = kz * (ky * kx)
	tmp := mat3Mul(ky, kx)
	r := mat3Mul(kz, tmp)

	// inverse for pure rotation is transpose
	return [3][3]float32{
		{r[0][0], r[1][0], r[2][0]},
		{r[0][1], r[1][1], r[2][1]},
		{r[0][2], r[1][2], r[2][2]},
	}
}

func mat3Mul(a, b [3][3]float32) [3][3]float32 {
	var c [3][3]float32
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			c[i][j] = a[i][0]*b[0][j] + a[i][1]*b[1][j] + a[i][2]*b[2][j]
		}
	}
	return c
}

func mat3MulVec(m [3][3]float32, v Vector3) Vector3 {
	return Vector3{
		m[0][0]*v.X + m[0][1]*v.Y + m[0][2]*v.Z,
		m[1][0]*v.X + m[1][1]*v.Y + m[1][2]*v.Z,
		m[2][0]*v.X + m[2][1]*v.Y + m[2][2]*v.Z,
	}
}

func (mi *ModelInstance) intersectHeight(pos Vector3, maxSearch float32) float32 {
	if mi.model == nil {
		return INVALID_HEIGHT
	}
	// Transform to model space exactly as AC ModelInstance::intersectRay:
	// p = iInvRot * (origin - iPos) * iInvScale
	// dir = iInvRot * direction
	// model t is in model units; world t = model t * scale
	p := Vector3{
		pos.X - mi.iPos.X,
		pos.Y - mi.iPos.Y,
		pos.Z - mi.iPos.Z,
	}
	p = mat3MulVec(mi.iInvRot, p)
	p = p.Mul(mi.iInvScale)

	// direction (0,0,-1)
	d := mat3MulVec(mi.iInvRot, Vector3{0, 0, -1})
	ray := NewRay(p, d)
	modelMax := maxSearch * mi.iInvScale
	t := mi.model.getHeight(ray, modelMax)
	if t < modelMax {
		// world space distance
		return t * mi.spawn.iScale
	}
	return INVALID_HEIGHT
}

// WorldModel simplified
type WorldModel struct {
	RootWMOID   uint32
	groupModels []GroupModel
	groupTree   BIH
}

func (w *WorldModel) readFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	br := bufio.NewReader(f)

	var chunk [8]byte
	if _, err := io.ReadFull(br, chunk[:]); err != nil {
		return err
	}
	// expect VMAP_MAGIC
	if string(chunk[:]) != VMAP_MAGIC[:8] { // rough
		// continue anyway for robustness
	}

	// WMOD
	if _, err := io.ReadFull(br, chunk[:4]); err != nil {
		return err
	}
	var chunkSize uint32
	binary.Read(br, binary.LittleEndian, &chunkSize)
	binary.Read(br, binary.LittleEndian, &w.RootWMOID)

	// GMOD
	if _, err := io.ReadFull(br, chunk[:4]); err != nil {
		return err
	}
	var count uint32
	binary.Read(br, binary.LittleEndian, &count)
	w.groupModels = make([]GroupModel, count)
	for i := uint32(0); i < count; i++ {
		if err := w.groupModels[i].readFrom(br); err != nil {
			return err
		}
	}
	// GBIH
	if _, err := io.ReadFull(br, chunk[:4]); err != nil {
		return err
	}
	if err := w.groupTree.readFrom(br); err != nil {
		return err
	}
	return nil
}

func (w *WorldModel) getHeight(ray Ray, maxDist float32) float32 {
	// return min t , caller computes height
	minT := maxDist
	for i := range w.groupModels {
		t := w.groupModels[i].getHeight(ray, maxDist)
		if t < minT {
			minT = t
		}
	}
	return minT
}

// GroupModel
type GroupModel struct {
	TriFlags   uint32
	Mesh       []MeshTriangle
	Vertices   []Vector3
	bounds     AABox
	groupTree  BIH // for triangles
}

func (g *GroupModel) readFromFile(f *os.File) error {
	return g.readFrom(f)
}

func (g *GroupModel) readFrom(r io.Reader) error {
	// Simplified read for GroupModel (flags, bounds, mesh, verts, tree)
	var flags uint32
	binary.Read(r, binary.LittleEndian, &flags)
	g.TriFlags = flags

	// bounds low high
	var lo, hi [3]float32
	binary.Read(r, binary.LittleEndian, &lo)
	binary.Read(r, binary.LittleEndian, &hi)
	g.bounds = AABox{Low: Vector3{lo[0],lo[1],lo[2]}, High: Vector3{hi[0],hi[1],hi[2]} }

	var nVerts uint32
	binary.Read(r, binary.LittleEndian, &nVerts)
	g.Vertices = make([]Vector3, nVerts)
	for i := range g.Vertices {
		var v [3]float32
		binary.Read(r, binary.LittleEndian, &v)
		g.Vertices[i] = Vector3{v[0], v[1], v[2]}
	}

	var nTris uint32
	binary.Read(r, binary.LittleEndian, &nTris)
	g.Mesh = make([]MeshTriangle, nTris)
	for i := range g.Mesh {
		var tri [3]uint32
		binary.Read(r, binary.LittleEndian, &tri)
		g.Mesh[i] = MeshTriangle{tri[0], tri[1], tri[2]}
	}

	// group BIH
	var chunk [4]byte
	io.ReadFull(r, chunk[:])
	g.groupTree.readFrom(r)
	return nil
}

func (g *GroupModel) getHeight(ray Ray, maxDist float32) float32 {
	// Simple brute force triangle intersection for height (slow but correct for correctness; can optimize later with tree)
	bestT := maxDist
	for _, tri := range g.Mesh {
		v0 := g.Vertices[tri.idx0]
		v1 := g.Vertices[tri.idx1]
		v2 := g.Vertices[tri.idx2]

		// Möller–Trumbore intersection for ray
		edge1 := v1.Sub(v0)
		edge2 := v2.Sub(v0)
		h := ray.Direction.Cross(edge2)
		a := edge1.Dot(h)
		if math.Abs(float64(a)) < 1e-5 {
			continue // parallel
		}
		f := 1.0 / a
		s := ray.Origin.Sub(v0)
		u := f * s.Dot(h)
		if u < 0 || u > 1 {
			continue
		}
		q := s.Cross(edge1)
		v := f * ray.Direction.Dot(q)
		if v < 0 || u+v > 1 {
			continue
		}
		t := f * edge2.Dot(q)
		if t > 0.00001 && t < bestT {
			bestT = t
		}
	}
	if bestT < maxDist {
		return bestT
	}
	return math.MaxFloat32
}

type MeshTriangle struct {
	idx0, idx1, idx2 uint32
}

// VMapManager manages vmap data per map for height queries.
type VMapManager struct {
	basePath string
	mu       sync.RWMutex
	trees    map[uint32]*StaticMapTree // per mapID
}

func NewVMapManager(vmapsDir string) *VMapManager {
	return &VMapManager{
		basePath: ensureTrailingSlash(vmapsDir),
		trees:    make(map[uint32]*StaticMapTree),
	}
}

func ensureTrailingSlash(p string) string {
	if len(p) > 0 && p[len(p)-1] != '/' && p[len(p)-1] != '\\' {
		return p + "/"
	}
	return p
}

func (vm *VMapManager) getTree(mapID uint32) (*StaticMapTree, error) {
	vm.mu.RLock()
	t, ok := vm.trees[mapID]
	vm.mu.RUnlock()
	if ok {
		return t, nil
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()
	if t, ok = vm.trees[mapID]; ok {
		return t, nil
	}

	tree := &StaticMapTree{mapID: mapID, basePath: vm.basePath}
	mapFile := fmt.Sprintf("%03d.vmtree", mapID) // e.g. 001.vmtree for map 1
	if !tree.InitMap(mapFile) {
		// no vmap for this map or error
		vm.trees[mapID] = nil
		return nil, fmt.Errorf("no vmap for map %d", mapID)
	}
	vm.trees[mapID] = tree
	return tree, nil
}

// GetHeight returns ground height by downward ray using vmap if available, else -inf.
func (vm *VMapManager) GetHeight(mapID uint32, x, y, z float32, maxSearch float32) (float32, bool) {
	tree, err := vm.getTree(mapID)
	if err != nil || tree == nil {
		return 0, false
	}
	// Load tiles around the query point using same grid coords as mmaps (AC uses gx,gy for vmap tiles too)
	// Use larger radius so large WMOs registered in neighboring tiles can be found for height.
	gx, gy := gameToGridCoords(x, y)
	for dx := -2; dx <= 2; dx++ {
		for dy := -2; dy <= 2; dy++ {
			tileX := uint32(gx + dx)
			tileY := uint32(gy + dy)
			if tileX < 0 || tileY < 0 { continue }
			_ = tree.LoadMapTile(tileX, tileY)
		}
	}

	pos := Vector3{X: x, Y: y, Z: z}
	h := tree.getHeight(pos, maxSearch)
	if h > INVALID_HEIGHT {
		return h, true
	}
	return 0, false
}

// StaticMapTree minimal port focused on height.
type StaticMapTree struct {
	mapID       uint32
	basePath    string
	isTiled     bool
	tree        BIH
	treeValues  map[uint32]*ModelInstance
	loadedTiles map[uint32]bool
	loadMu      sync.Mutex
	modelCache  map[string]*WorldModel // share by filename to avoid re-parsing same .vmo

	dataMu sync.RWMutex // protects treeValues and modelCache for concurrent access from GetHeight / LoadMapTile
}

func (s *StaticMapTree) InitMap(fname string) bool {
	full := s.basePath + fname
	f, err := os.Open(full)
	if err != nil {
		return false
	}
	defer f.Close()

	var chunk [8]byte
	if _, err := io.ReadFull(f, chunk[:]); err != nil {
		return false
	}
	var tiled byte
	if err := binary.Read(f, binary.LittleEndian, &tiled); err != nil {
		return false
	}
	s.isTiled = tiled != 0

	// NODE
	if _, err := io.ReadFull(f, chunk[:4]); err != nil {
		return false
	}
	if err := s.tree.readFromFile(f); err != nil {
		return false
	}
	s.treeValues = make(map[uint32]*ModelInstance)

	// GOBJ
	if _, err := io.ReadFull(f, chunk[:4]); err != nil {
		return false
	}

	// For non-tiled, read global spawn
	if !s.isTiled {
		var spawn ModelSpawn
		if err := spawn.readFromFile(f); err == nil {
			spawn.iPos.X = VMAP_mid - spawn.iPos.X
			spawn.iPos.Y = VMAP_mid - spawn.iPos.Y
			model := &WorldModel{}
			modelPath := s.basePath + spawn.name
			if err := model.readFile(modelPath); err == nil {
				inst := &ModelInstance{spawn: spawn, model: model}
				inst.initTransform()
				s.dataMu.Lock()
				s.treeValues[0] = inst
				s.dataMu.Unlock()
			}
		}
	}

	s.loadedTiles = make(map[uint32]bool)
	return true
}

func (s *StaticMapTree) LoadMapTile(tileX, tileY uint32) error {
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	tid := packTileID(int32(tileX), int32(tileY))
	if !s.isTiled {
		s.loadedTiles[tid] = false
		return nil
	}
	if _, loaded := s.loadedTiles[tid]; loaded {
		return nil
	}
	tileFile := s.basePath + getTileFileName(s.mapID, tileX, tileY)
	f, err := os.Open(tileFile)
	if err != nil {
		s.loadedTiles[tid] = false
		return nil // optional tile
	}
	defer f.Close()

	// bufio makes the many small reads in spawn parsing (1k+ per tile) much faster
	br := bufio.NewReader(f)

	var chunk [8]byte
	io.ReadFull(br, chunk[:]) // magic

	var numSpawns uint32
	binary.Read(br, binary.LittleEndian, &numSpawns)
	loadedRefs := 0
	for i := uint32(0); i < numSpawns; i++ {
		var spawn ModelSpawn
		if err := spawn.readFrom(br); err != nil {
			break
		}
		// vmap stores positions in internal rep (mid - game). Convert to game coords for our logic.
		spawn.iPos.X = VMAP_mid - spawn.iPos.X
		spawn.iPos.Y = VMAP_mid - spawn.iPos.Y
		var ref uint32
		binary.Read(br, binary.LittleEndian, &ref)
		// Only keep spawns that can contribute to height (have explicit bounds).
		// This keeps treeValues small, reduces snapshot allocs in getHeight, and
		// avoids tracking thousands of tiny doodads.
		if (spawn.Flags & 0x4) != 0 {
			s.dataMu.Lock()
			if _, exists := s.treeValues[ref]; !exists {
				s.treeValues[ref] = &ModelInstance{spawn: spawn}
				loadedRefs++
			}
			s.dataMu.Unlock()
		}
	}
	s.loadedTiles[tid] = true
	return nil
}

func getTileFileName(mapID, tileX, tileY uint32) string {
	// AC: map_y_x.vmtile
	return fmt.Sprintf("%03d_%02d_%02d.vmtile", mapID, tileY, tileX)
}

func (s *StaticMapTree) getHeight(pos Vector3, maxSearch float32) float32 {
	s.dataMu.RLock()
	if len(s.treeValues) == 0 {
		s.dataMu.RUnlock()
		return INVALID_HEIGHT
	}
	// Snapshot under read lock to avoid concurrent map iteration vs write
	// (LoadMapTile can add entries concurrently, and lazy model loading writes modelCache).
	snap := make([]*ModelInstance, 0, len(s.treeValues))
	for _, mi := range s.treeValues {
		snap = append(snap, mi)
	}
	s.dataMu.RUnlock()

	// Find smallest positive world-space intersection distance (closest hit when shooting down)
	minT := maxSearch
	for _, mi := range snap {
		if mi == nil || mi.spawn.name == "" {
			continue
		}
		if mi.model == nil {
			// lazy load only if roughly near query (saves huge time/memory)
			// Only load models that declare bounds (typically significant static geometry
			// that provides walkable floor). Loading every doodad/M2 explodes memory
			// (60GB+ in load tests) because we keep full vertex+tri data forever in modelCache.
			hasB := (mi.spawn.Flags & 0x4) != 0
			if !hasB {
				continue
			}
			lo := mi.spawn.iBound[0]
			hi := mi.spawn.iBound[1]
			// expand a bit for safety
			if pos.X < lo.X-1 || pos.X > hi.X+1 || pos.Y < lo.Y-1 || pos.Y > hi.Y+1 {
				continue
			}

			mf := s.basePath + mi.spawn.name
			if mf == s.basePath {
				continue
			}
			var model *WorldModel
			s.dataMu.RLock()
			if s.modelCache != nil {
				if cached, ok := s.modelCache[mf]; ok {
					model = cached
				}
			}
			s.dataMu.RUnlock()

			if model == nil {
				model = &WorldModel{}
				if err := model.readFile(mf); err != nil {
					// Mark as bad under write lock (rare)
					s.dataMu.Lock()
					mi.spawn.name = ""
					s.dataMu.Unlock()
					continue
				}
				s.dataMu.Lock()
				if s.modelCache == nil {
					s.modelCache = make(map[string]*WorldModel)
				}
				s.modelCache[mf] = model
				s.dataMu.Unlock()
			}
			mi.model = model
			mi.initTransform()
		}
		t := mi.intersectHeight(pos, maxSearch)
		if t > 0 && t < minT {
			minT = t
		}
	}
	if minT >= maxSearch {
		return INVALID_HEIGHT
	}
	// ground Z is query Z minus the distance traveled down
	return pos.Z - minT
}

func (s *StaticMapTree) getHeightACStyle(pos Vector3, maxSearch float32) float32 {
	return s.getHeight(pos, maxSearch)
}

// Integrate with TerrainManager: combined height.
func (tm *TerrainManager) getCombinedHeightWithVMap(mapID uint32, x, y, z float32, vmap *VMapManager) (float32, bool) {
	terrainH, hasT := tm.GetHeight(mapID, x, y, z)
	var vmapH float32
	hasV := false
	if vmap != nil {
		if vh, ok := vmap.GetHeight(mapID, x, y, z, 1000); ok && vh > INVALID_HEIGHT {
			vmapH = vh
			hasV = true
		}
	}

	if hasV && hasT {
		// AC style choice
		if vmapH > terrainH || math.Abs(float64(terrainH-z)) > math.Abs(float64(vmapH-z)) {
			return vmapH, true
		}
		return terrainH, true
	}
	if hasV {
		return vmapH, true
	}
	if hasT {
		return terrainH, true
	}
	return 0, false
}
