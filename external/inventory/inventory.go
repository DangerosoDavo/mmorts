package inventory

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Option configures inventory construction.
type Option func(*Inventory)

// WithRegistry attaches an item registry used to resolve metadata during
// stack operations and serialization.
func WithRegistry(reg *Registry) Option {
	return func(inv *Inventory) {
		inv.registry = reg
	}
}

// NewVolume creates a volume-constrained inventory instance for an owner.
func NewVolume(id string, owner OwnerID, capacity int, opts ...Option) *Inventory {
	inv := &Inventory{
		ID:             id,
		Owner:          owner,
		Mode:           ModeVolume,
		VolumeCapacity: capacity,
		Stacks:         make([]Stack, 0),
	}
	applyOptions(inv, opts...)
	return inv
}

// NewGrid creates a grid-constrained inventory instance for an owner.
func NewGrid(id string, owner OwnerID, width, height int, opts ...Option) *Inventory {
	inv := &Inventory{
		ID:        id,
		Owner:     owner,
		Mode:      ModeGrid,
		GridWidth: width, GridHeight: height,
		Stacks:    make([]Stack, 0),
		occupancy: make(map[Point]string),
	}
	applyOptions(inv, opts...)
	return inv
}

// NewHybrid creates an inventory that enforces both volume and grid constraints.
func NewHybrid(id string, owner OwnerID, capacity, width, height int, opts ...Option) *Inventory {
	inv := &Inventory{
		ID:             id,
		Owner:          owner,
		Mode:           ModeBoth,
		VolumeCapacity: capacity,
		GridWidth:      width,
		GridHeight:     height,
		Stacks:         make([]Stack, 0),
		occupancy:      make(map[Point]string),
	}
	applyOptions(inv, opts...)
	return inv
}

func applyOptions(inv *Inventory, opts ...Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(inv)
		}
	}
}

// Serialize encodes the inventory to JSON using a compact representation.
func (inv *Inventory) Serialize() ([]byte, error) {
	type stackSnapshot struct {
		Item     ItemID  `json:"item"`
		Owner    OwnerID `json:"owner,omitempty"`
		Qty      int     `json:"qty"`
		StackMax int     `json:"stackMax,omitempty"`
		Shape    *Shape  `json:"shape,omitempty"`
		Position *Point  `json:"position,omitempty"`
	}
	type snapshot struct {
		ID             string          `json:"id"`
		Owner          OwnerID         `json:"owner,omitempty"`
		Mode           Mode            `json:"mode"`
		VolumeCapacity int             `json:"volumeCapacity,omitempty"`
		VolumeUsed     int             `json:"volumeUsed,omitempty"`
		GridWidth      int             `json:"gridWidth,omitempty"`
		GridHeight     int             `json:"gridHeight,omitempty"`
		Stacks         []stackSnapshot `json:"stacks"`
	}
	ss := snapshot{
		ID:             inv.ID,
		Owner:          inv.Owner,
		Mode:           inv.Mode,
		VolumeCapacity: inv.VolumeCapacity,
		VolumeUsed:     inv.VolumeUsed,
		GridWidth:      inv.GridWidth,
		GridHeight:     inv.GridHeight,
		Stacks:         make([]stackSnapshot, 0, len(inv.Stacks)),
	}
	for _, st := range inv.Stacks {
		ss.Stacks = append(ss.Stacks, stackSnapshot{
			Item:     st.Item,
			Owner:    st.Owner,
			Qty:      st.Qty,
			StackMax: st.StackMax,
			Shape:    st.Shape,
			Position: st.Position,
		})
	}
	return json.Marshal(ss)
}

// Deserialize replaces the inventory with data from JSON. A registry must be
// attached beforehand (or supplied via WithRegistry) if volume constraints are
// enforced and stacks omit volume overrides.
// SerializeForStorage encodes the inventory using compact numeric RegistryIDs
// instead of string ItemIDs. Requires a registry to resolve ItemID -> RegistryID.
// This format is optimized for database storage efficiency.
func (inv *Inventory) SerializeForStorage() ([]byte, error) {
	if inv.registry == nil {
		return nil, errors.New("registry required for storage serialization")
	}
	
	ss := StorageSnapshot{
		ID:             inv.ID,
		Owner:          inv.Owner,
		Mode:           inv.Mode,
		VolumeCapacity: inv.VolumeCapacity,
		VolumeUsed:     inv.VolumeUsed,
		GridWidth:      inv.GridWidth,
		GridHeight:     inv.GridHeight,
		Stacks:         make([]StorageStackSnapshot, 0, len(inv.Stacks)),
	}
	
	for _, st := range inv.Stacks {
		regID, ok := inv.registry.GetRegistryID(st.Item)
		if !ok {
			return nil, fmt.Errorf("item not found in registry: %s", st.Item)
		}
		ss.Stacks = append(ss.Stacks, StorageStackSnapshot{
			Item:     regID,
			Owner:    st.Owner,
			Qty:      st.Qty,
			StackMax: st.StackMax,
			Shape:    st.Shape,
			Position: st.Position,
		})
	}
	return json.Marshal(ss)
}

// DeserializeFromStorage replaces the inventory with data from storage-optimized
// JSON format. Requires a registry to resolve RegistryID -> ItemID.
func (inv *Inventory) DeserializeFromStorage(b []byte) error {
	if inv.registry == nil {
		return errors.New("registry required for storage deserialization")
	}
	
	var ss StorageSnapshot
	if err := json.Unmarshal(b, &ss); err != nil {
		return err
	}

	// reset inventory state
	inv.ID = ss.ID
	inv.Owner = ss.Owner
	inv.Mode = ss.Mode
	inv.VolumeCapacity = ss.VolumeCapacity
	inv.VolumeUsed = 0
	inv.GridWidth = ss.GridWidth
	inv.GridHeight = ss.GridHeight
	inv.Stacks = make([]Stack, 0, len(ss.Stacks))
	if inv.Mode == ModeGrid || inv.Mode == ModeBoth {
		inv.occupancy = make(map[Point]string)
	} else {
		inv.occupancy = nil
	}

	for _, st := range ss.Stacks {
		details, ok := inv.registry.LookupByRegistryID(st.Item)
		if !ok {
			return fmt.Errorf("registry id not found: %d", st.Item)
		}
		stack := Stack{
			Item:     details.ID,
			Owner:    st.Owner,
			Qty:      st.Qty,
			StackMax: st.StackMax,
			Shape:    st.Shape,
			Position: st.Position,
		}
		if err := inv.AddStack(stack); err != nil {
			return err
		}
	}
	return nil
}

func (inv *Inventory) Deserialize(b []byte) error {
	type stackSnapshot struct {
		Item     ItemID  `json:"item"`
		Owner    OwnerID `json:"owner,omitempty"`
		Qty      int     `json:"qty"`
		StackMax int     `json:"stackMax,omitempty"`
		Shape    *Shape  `json:"shape,omitempty"`
		Position *Point  `json:"position,omitempty"`
	}
	type snapshot struct {
		ID             string          `json:"id"`
		Owner          OwnerID         `json:"owner,omitempty"`
		Mode           Mode            `json:"mode"`
		VolumeCapacity int             `json:"volumeCapacity,omitempty"`
		GridWidth      int             `json:"gridWidth,omitempty"`
		GridHeight     int             `json:"gridHeight,omitempty"`
		Stacks         []stackSnapshot `json:"stacks"`
	}
	var ss snapshot
	if err := json.Unmarshal(b, &ss); err != nil {
		return err
	}

	// reset inventory state
	inv.ID = ss.ID
	inv.Owner = ss.Owner
	inv.Mode = ss.Mode
	inv.VolumeCapacity = ss.VolumeCapacity
	inv.VolumeUsed = 0
	inv.GridWidth = ss.GridWidth
	inv.GridHeight = ss.GridHeight
	inv.Stacks = make([]Stack, 0, len(ss.Stacks))
	if inv.Mode == ModeGrid || inv.Mode == ModeBoth {
		inv.occupancy = make(map[Point]string)
	} else {
		inv.occupancy = nil
	}

	for _, st := range ss.Stacks {
		stack := Stack{
			Item:     st.Item,
			Owner:    st.Owner,
			Qty:      st.Qty,
			StackMax: st.StackMax,
			Shape:    st.Shape,
			Position: st.Position,
		}
		if err := inv.AddStack(stack); err != nil {
			return err
		}
	}
	return nil
}

// Registry returns the currently attached item registry.
func (inv *Inventory) Registry() *Registry { return inv.registry }

// SetRegistry attaches or replaces the item registry.
func (inv *Inventory) SetRegistry(reg *Registry) { inv.registry = reg }

// AddStack inserts a stack into the inventory. For volume tracking, provide
// VolumePerUnit on the stack or register a volume in the attached registry.
// For grid placement, provide Shape and (optional) Position. If Position is
// nil, the inventory attempts to auto-place the item.
func (inv *Inventory) AddStack(s Stack) error {
	if s.Qty <= 0 {
		return errors.New("quantity must be positive")
	}
	if inv.Owner != "" && s.Owner != "" && inv.Owner != s.Owner {
		return fmt.Errorf("owner mismatch: inv=%s stack=%s", inv.Owner, s.Owner)
	}
	if s.ItemRef != nil && s.ItemRef.InventoryItemID() != s.Item {
		return errors.New("item reference id mismatch")
	}
	if inv.registry != nil && s.ItemRef != nil {
		if rich, ok := s.ItemRef.(RichItem); ok {
			_ = inv.registry.RegisterItem(rich)
		}
	}
	// Normalize StackMax for grid-constrained inventories
	if (inv.Mode == ModeGrid || inv.Mode == ModeBoth) && s.StackMax == 0 {
		s.StackMax = 1
	}
	if (inv.Mode == ModeGrid || inv.Mode == ModeBoth) && s.StackMax > 0 && s.Qty > s.StackMax {
		return fmt.Errorf("qty exceeds stackMax: qty=%d stackMax=%d", s.Qty, s.StackMax)
	}
	volPerUnit, err := inv.resolveVolume(&s)
	if err != nil {
		return err
	}
	s.VolumePerUnit = volPerUnit
	// Volume constraint
	if inv.Mode == ModeVolume || inv.Mode == ModeBoth {
		required := volPerUnit * s.Qty
		if inv.VolumeUsed+required > inv.VolumeCapacity {
			return fmt.Errorf("volume exceeded: used=%d req=%d cap=%d", inv.VolumeUsed, required, inv.VolumeCapacity)
		}
		inv.VolumeUsed += required
	}
	// Grid constraint
	if inv.Mode == ModeGrid || inv.Mode == ModeBoth {
		if s.Shape == nil {
			// default to 1x1 if shape missing
			s.Shape = &Shape{Width: 1, Height: 1}
		}
		// Pre-assign a key based on the final index if appended.
		// This allows occupancy bookkeeping to tag cells correctly.
		s.key = inv.stackKey(len(inv.Stacks))
		// attempt to place at provided position or auto-place
		if s.Position != nil {
			if err := inv.placeAt(&s, *s.Position); err != nil {
				return err
			}
		} else {
			p, ok := inv.findFirstFit(*s.Shape)
			if !ok {
				return errors.New("no space available for shape")
			}
			if err := inv.placeAt(&s, p); err != nil {
				return err
			}
		}
	}
	// Append and assign key
	inv.Stacks = append(inv.Stacks, s)
	return nil
}

// RemoveStack reduces quantity or removes the stack entirely by index.
// For grid-based inventories, this also frees occupied cells.
func (inv *Inventory) RemoveStack(index int, qty int) error {
	if index < 0 || index >= len(inv.Stacks) {
		return errors.New("index out of range")
	}
	if qty <= 0 {
		return errors.New("qty must be positive")
	}
	st := inv.Stacks[index]
	if qty > st.Qty {
		return errors.New("qty exceeds stack amount")
	}
	// Volume refund
	if inv.Mode == ModeVolume || inv.Mode == ModeBoth {
		inv.VolumeUsed -= st.VolumePerUnit * qty
		if inv.VolumeUsed < 0 {
			inv.VolumeUsed = 0
		}
	}
	st.Qty -= qty
	if st.Qty == 0 {
		// remove stack and free placement
		if inv.Mode == ModeGrid || inv.Mode == ModeBoth {
			inv.freePlacement(st)
		}
		inv.Stacks = append(inv.Stacks[:index], inv.Stacks[index+1:]...)
		// reassign keys for subsequent stacks to keep invariants simple
		for i := index; i < len(inv.Stacks); i++ {
			inv.Stacks[i].key = inv.stackKey(i)
		}
	} else {
		inv.Stacks[index] = st
	}
	return nil
}

// helper: create a deterministic per-index key
func (inv *Inventory) stackKey(i int) string {
	return fmt.Sprintf("%s#%d", inv.ID, i)
}

// placeAt attempts to place the stack at the provided origin.
func (inv *Inventory) placeAt(s *Stack, origin Point) error {
	if ok := inv.canPlaceAt(*s.Shape, origin); !ok {
		return errors.New("cannot place at requested position")
	}
	s.Position = &origin
	return inv.applyPlacement(*s, false)
}

// canPlaceAt checks grid bounds and collisions for a given shape at an origin.
func (inv *Inventory) canPlaceAt(shape Shape, origin Point) bool {
	if inv.occupancy == nil {
		inv.occupancy = make(map[Point]string)
	}
	cells := shapeCells(shape)
	for _, c := range cells {
		x := origin.X + c.X
		y := origin.Y + c.Y
		if x < 0 || y < 0 || x >= inv.GridWidth || y >= inv.GridHeight {
			return false
		}
		if _, exists := inv.occupancy[Point{X: x, Y: y}]; exists {
			return false
		}
	}
	return true
}

func (inv *Inventory) resolveVolume(s *Stack) (int, error) {
	v := s.VolumePerUnit
	if v < 0 {
		return 0, errors.New("invalid volume per unit")
	}
	if inv.Mode != ModeVolume && inv.Mode != ModeBoth {
		return v, nil
	}
	if v > 0 {
		return v, nil
	}
	if inv.registry != nil {
		if vol, ok := inv.registry.VolumeFor(s.Item); ok {
			return vol, nil
		}
	}
	// zero volume is allowed; the caller enforces capacity separately.
	return 0, nil
}

// applyPlacement reserves grid cells for a placed stack.
// If rebuild is true, uses the stack.key as-is; otherwise expects s.key to be empty (will be assigned on append).
func (inv *Inventory) applyPlacement(s Stack, rebuild bool) error {
	if inv.occupancy == nil {
		inv.occupancy = make(map[Point]string)
	}
	if s.Shape == nil || s.Position == nil {
		return errors.New("missing shape or position for placement")
	}
	cells := shapeCells(*s.Shape)
	key := s.key
	for _, c := range cells {
		p := Point{X: s.Position.X + c.X, Y: s.Position.Y + c.Y}
		inv.occupancy[p] = key
	}
	return nil
}

// freePlacement releases cells occupied by a stack.
func (inv *Inventory) freePlacement(s Stack) {
	if inv.occupancy == nil || s.Shape == nil || s.Position == nil {
		return
	}
	for _, c := range shapeCells(*s.Shape) {
		p := Point{X: s.Position.X + c.X, Y: s.Position.Y + c.Y}
		if owner, ok := inv.occupancy[p]; ok && owner == s.key {
			delete(inv.occupancy, p)
		}
	}
}

// findFirstFit scans the grid row-major and returns an origin where the shape fits.
func (inv *Inventory) findFirstFit(shape Shape) (Point, bool) {
	if inv.GridWidth <= 0 || inv.GridHeight <= 0 {
		return Point{}, false
	}
	// derive bounding box for iteration
	maxX := 0
	maxY := 0
	for _, c := range shapeCells(shape) {
		if c.X > maxX {
			maxX = c.X
		}
		if c.Y > maxY {
			maxY = c.Y
		}
	}
	for y := 0; y <= inv.GridHeight-1-maxY; y++ {
		for x := 0; x <= inv.GridWidth-1-maxX; x++ {
			p := Point{X: x, Y: y}
			if inv.canPlaceAt(shape, p) {
				return p, true
			}
		}
	}
	return Point{}, false
}

// shapeCells returns the set of relative cells for a shape.
func shapeCells(s Shape) []Point {
	if len(s.Cells) > 0 {
		return s.Cells
	}
	w := s.Width
	h := s.Height
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	out := make([]Point, 0, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out = append(out, Point{X: x, Y: y})
		}
	}
	return out
}
