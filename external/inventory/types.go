package inventory

// Package inventory provides a minimal, item-agnostic inventory system.
// It only tracks item identifiers, owners, quantities and optional
// capacity/placement constraints (volume or grid slots).

// ItemID represents an application-defined identifier for an item.
// The inventory system does not interpret this value.
type ItemID string

// StoredItem represents a runtime object that can be tracked by the
// inventory. Implementations live in the host application; the inventory
// package only requires that the object exposes a stable ItemID.
type StoredItem interface {
	InventoryItemID() ItemID
}

// RichItem extends StoredItem with metadata used to populate registries.
// Applications can optionally implement this to allow automatic registry
// population when registering items.
type RichItem interface {
	StoredItem
	InventoryItemDetails() ItemDetails
}

// OwnerID represents an application-defined owner identifier.
// Can be user id, character id, etc.
type OwnerID string

// RegistryID is a numeric handle suitable for compact storage (e.g. databases).
// IDs start at 1 and increment as new items are registered unless explicitly
// provided via ItemDetails.NumericID.
type RegistryID int64

// Point represents a grid coordinate (x, y) with origin at top-left.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Shape describes a grid footprint as a set of cell offsets relative to an origin.
// For simple rectangular items, Width x Height may be provided and Cells can be nil.
// If Cells is non-empty, it takes precedence and allows Tetris-like shapes.
type Shape struct {
	Width  int     `json:"width,omitempty"`
	Height int     `json:"height,omitempty"`
	Cells  []Point `json:"cells,omitempty"`
}

// Stack represents an item stack tracked by the inventory.
// The inventory does not know item metadata beyond these fields.
type Stack struct {
	Item  ItemID  `json:"item"`
	Owner OwnerID `json:"owner"`
	Qty   int     `json:"qty"`
	// StackMax indicates the per-stack maximum count allowed for this item
	// when placed in a grid. If zero, defaults to 1 for grid-constrained
	// inventories. For non-grid inventories, it is ignored.
	StackMax int `json:"stackMax,omitempty"`

	// Optional constraints supplied by the caller at insertion time.
	// For volume-based inventories, set VolumePerUnit. If zero, the inventory
	// will consult its registry (if any) for volume information.
	// For grid-based inventories, set Shape and (optional) Position.
	VolumePerUnit int    `json:"-"`
	WeightPerUnit int    `json:"-"`
	Shape         *Shape `json:"shape,omitempty"`
	// For placed items, Position is the origin where the shape is anchored.
	// If nil, the inventory may auto-place where possible.
	Position *Point `json:"position,omitempty"`
	// ItemRef holds a runtime object associated with this stack. It is not
	// serialized; consumers are responsible for reattaching data after load.
	ItemRef StoredItem `json:"-"`

	// internal unique key for placement bookkeeping (not exported)
	key string
}

// Mode indicates what constraints an inventory enforces.
type Mode int

const (
	// ModeNone enforces no capacity; all stacks are accepted.
	ModeNone Mode = iota
	// ModeVolume enforces a finite volume capacity.
	ModeVolume
	// ModeGrid enforces a finite grid capacity (width x height) with occupancy.
	ModeGrid
	// ModeBoth enforces both volume and grid constraints.
	ModeBoth
)

// StorageStackSnapshot represents a stack in storage-optimized format using
// numeric RegistryID instead of string ItemID for database efficiency.
type StorageStackSnapshot struct {
	Item     RegistryID `json:"item"`
	Owner    OwnerID    `json:"owner,omitempty"`
	Qty      int        `json:"qty"`
	StackMax int        `json:"stackMax,omitempty"`
	Shape    *Shape     `json:"shape,omitempty"`
	Position *Point     `json:"position,omitempty"`
}

// StorageSnapshot represents an inventory in storage-optimized format using
// numeric RegistryIDs for database efficiency.
type StorageSnapshot struct {
	ID             string                  `json:"id"`
	Owner          OwnerID                 `json:"owner,omitempty"`
	Mode           Mode                    `json:"mode"`
	VolumeCapacity int                     `json:"volumeCapacity,omitempty"`
	VolumeUsed     int                     `json:"volumeUsed,omitempty"`
	GridWidth      int                     `json:"gridWidth,omitempty"`
	GridHeight     int                     `json:"gridHeight,omitempty"`
	Stacks         []StorageStackSnapshot  `json:"stacks"`
}

// Inventory represents a collection of stacks for a single owner under
// optional capacity constraints.
type Inventory struct {
	ID    string  `json:"id"`
	Owner OwnerID `json:"owner"`
	Mode  Mode    `json:"mode"`

	// Volume capacity (if enabled by ModeVolume/ModeBoth)
	VolumeCapacity int `json:"volumeCapacity,omitempty"`
	VolumeUsed     int `json:"volumeUsed,omitempty"`

	// Grid capacity (if enabled by ModeGrid/ModeBoth)
	GridWidth  int `json:"gridWidth,omitempty"`
	GridHeight int `json:"gridHeight,omitempty"`

	// Stacks holds all tracked stacks.
	Stacks []Stack `json:"stacks"`

	// occupancy maps cell -> stack key for grid placements
	occupancy map[Point]string

	// registry provides item metadata (volume, weight, descriptions).
	registry *Registry
}
