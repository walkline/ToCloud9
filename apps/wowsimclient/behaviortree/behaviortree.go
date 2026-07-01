// Package behaviortree provides a hierarchical behavior tree implementation
// for WoW bot AI. All node types implement the Node interface. Composite nodes
// (Selector, Sequence, Parallel) manage children; Decorator wraps a single
// child; Action and Condition are leaves. The tree is ticked once per update
// cycle by calling Tick on the root.
package behaviortree

// Status is the result returned by a node tick.
type Status int

const (
	// Success means the node completed successfully.
	Success Status = iota
	// Failure means the node failed.
	Failure
	// Running means the node is still executing and will be ticked again.
	Running
)

// Blackboard is shared state accessible to all nodes in a tree.
type Blackboard struct {
	data map[string]interface{}
}

// NewBlackboard creates a new empty blackboard.
func NewBlackboard() *Blackboard {
	return &Blackboard{data: make(map[string]interface{})}
}

// Set stores a value.
func (b *Blackboard) Set(key string, val interface{}) {
	b.data[key] = val
}

// Get retrieves a value.
func (b *Blackboard) Get(key string) (interface{}, bool) {
	v, ok := b.data[key]
	return v, ok
}

// GetString returns a string value or empty if not found.
func (b *Blackboard) GetString(key string) string {
	v, ok := b.data[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// GetFloat returns a float64 value or 0 if not found.
func (b *Blackboard) GetFloat(key string) float64 {
	v, ok := b.data[key]
	if !ok {
		return 0
	}
	f, _ := v.(float64)
	return f
}

// GetInt returns an int value or 0 if not found.
func (b *Blackboard) GetInt(key string) int {
	v, ok := b.data[key]
	if !ok {
		return 0
	}
	i, _ := v.(int)
	return i
}

// GetBool returns a bool value or false if not found.
func (b *Blackboard) GetBool(key string) bool {
	v, ok := b.data[key]
	if !ok {
		return false
	}
	bl, _ := v.(bool)
	return bl
}

// Delete removes a value.
func (b *Blackboard) Delete(key string) {
	delete(b.data, key)
}

// Node is the interface for all behavior tree nodes.
type Node interface {
	Tick(bb *Blackboard) Status
	Reset()
	Name() string
}

// ============================================================
// Composite nodes
// ============================================================

// Selector tries children left-to-right and succeeds as soon as one succeeds.
// If a child returns Running, the selector also returns Running.
type Selector struct {
	name     string
	Children []Node
	current  int
}

func NewSelector(name string, children ...Node) *Selector {
	return &Selector{name: name, Children: children}
}

func (s *Selector) Name() string { return s.name }

func (s *Selector) Tick(bb *Blackboard) Status {
	// Always re-evaluate from the start to allow higher-priority children
	// to preempt lower-priority running children.
	for i := 0; i < len(s.Children); i++ {
		st := s.Children[i].Tick(bb)
		switch st {
		case Success:
			return Success
		case Running:
			return Running
		case Failure:
			continue
		}
	}
	return Failure
}

func (s *Selector) Reset() {
	s.current = 0
	for _, c := range s.Children {
		c.Reset()
	}
}

// Sequence runs children left-to-right and fails as soon as one fails.
type Sequence struct {
	name     string
	Children []Node
	current  int
}

func NewSequence(name string, children ...Node) *Sequence {
	return &Sequence{name: name, Children: children}
}

func (s *Sequence) Name() string { return s.name }

func (s *Sequence) Tick(bb *Blackboard) Status {
	for i := s.current; i < len(s.Children); i++ {
		st := s.Children[i].Tick(bb)
		switch st {
		case Failure:
			s.current = 0
			return Failure
		case Running:
			s.current = i
			return Running
		case Success:
			continue
		}
	}
	s.current = 0
	return Success
}

func (s *Sequence) Reset() {
	s.current = 0
	for _, c := range s.Children {
		c.Reset()
	}
}

// Parallel runs all children every tick. Succeeds when successThreshold
// children have succeeded, fails when failThreshold children have failed.
type Parallel struct {
	name             string
	Children         []Node
	successThreshold int
	failThreshold    int
}

func NewParallel(name string, successThreshold, failThreshold int, children ...Node) *Parallel {
	return &Parallel{
		name:             name,
		Children:         children,
		successThreshold: successThreshold,
		failThreshold:    failThreshold,
	}
}

func (p *Parallel) Name() string { return p.name }

func (p *Parallel) Tick(bb *Blackboard) Status {
	successCount := 0
	failCount := 0
	for _, c := range p.Children {
		st := c.Tick(bb)
		switch st {
		case Success:
			successCount++
		case Failure:
			failCount++
		}
	}
	if successCount >= p.successThreshold {
		return Success
	}
	if failCount >= p.failThreshold {
		return Failure
	}
	return Running
}

func (p *Parallel) Reset() {
	for _, c := range p.Children {
		c.Reset()
	}
}

// ============================================================
// Decorator nodes
// ============================================================

// Inverter inverts the result of its child: Success->Failure, Failure->Success.
type Inverter struct {
	name  string
	Child Node
}

func NewInverter(name string, child Node) *Inverter {
	return &Inverter{name: name, Child: child}
}

func (i *Inverter) Name() string { return i.name }

func (i *Inverter) Tick(bb *Blackboard) Status {
	st := i.Child.Tick(bb)
	switch st {
	case Success:
		return Failure
	case Failure:
		return Success
	default:
		return Running
	}
}

func (i *Inverter) Reset() { i.Child.Reset() }

// Repeater ticks its child n times (0 means infinite until failure).
type Repeater struct {
	name    string
	Child   Node
	limit   int
	counter int
}

func NewRepeater(name string, limit int, child Node) *Repeater {
	return &Repeater{name: name, Child: child, limit: limit}
}

func (r *Repeater) Name() string { return r.name }

func (r *Repeater) Tick(bb *Blackboard) Status {
	st := r.Child.Tick(bb)
	if st == Failure {
		r.counter = 0
		return Failure
	}
	if st == Running {
		return Running
	}
	r.counter++
	if r.limit > 0 && r.counter >= r.limit {
		r.counter = 0
		return Success
	}
	return Running
}

func (r *Repeater) Reset() {
	r.counter = 0
	r.Child.Reset()
}

// Succeeder always returns Success regardless of child result.
type Succeeder struct {
	name  string
	Child Node
}

func NewSucceeder(name string, child Node) *Succeeder {
	return &Succeeder{name: name, Child: child}
}

func (s *Succeeder) Name() string { return s.name }

func (s *Succeeder) Tick(bb *Blackboard) Status {
	s.Child.Tick(bb)
	return Success
}

func (s *Succeeder) Reset() { s.Child.Reset() }

// ============================================================
// Leaf nodes
// ============================================================

// Action is a leaf node that executes a function.
type Action struct {
	name string
	fn   func(bb *Blackboard) Status
}

func NewAction(name string, fn func(bb *Blackboard) Status) *Action {
	return &Action{name: name, fn: fn}
}

func (a *Action) Name() string               { return a.name }
func (a *Action) Tick(bb *Blackboard) Status { return a.fn(bb) }
func (a *Action) Reset()                     {}

// Condition is a leaf node that checks a condition and returns Success or Failure.
type Condition struct {
	name string
	fn   func(bb *Blackboard) bool
}

func NewCondition(name string, fn func(bb *Blackboard) bool) *Condition {
	return &Condition{name: name, fn: fn}
}

func (c *Condition) Name() string { return c.name }

func (c *Condition) Tick(bb *Blackboard) Status {
	if c.fn(bb) {
		return Success
	}
	return Failure
}

func (c *Condition) Reset() {}

// ============================================================
// Dynamic / custom nodes
// ============================================================

// DynamicNode is a node whose behavior can be swapped at runtime.
// This is used for Lua-driven behavior replacement.
type DynamicNode struct {
	name    string
	current Node
}

func NewDynamicNode(name string, initial Node) *DynamicNode {
	return &DynamicNode{name: name, current: initial}
}

func (d *DynamicNode) Name() string { return d.name }

func (d *DynamicNode) Tick(bb *Blackboard) Status {
	if d.current == nil {
		return Failure
	}
	return d.current.Tick(bb)
}

func (d *DynamicNode) Reset() {
	if d.current != nil {
		d.current.Reset()
	}
}

// SetNode replaces the underlying node at runtime.
func (d *DynamicNode) SetNode(n Node) {
	d.current = n
}

// Tree is the root container for a behavior tree.
type Tree struct {
	Root       Node
	Blackboard *Blackboard
}

// NewTree creates a new behavior tree with the given root node.
func NewTree(root Node) *Tree {
	return &Tree{
		Root:       root,
		Blackboard: NewBlackboard(),
	}
}

// Tick runs one update cycle on the tree.
func (t *Tree) Tick() Status {
	return t.Root.Tick(t.Blackboard)
}

// Reset resets the entire tree.
func (t *Tree) Reset() {
	t.Root.Reset()
}
