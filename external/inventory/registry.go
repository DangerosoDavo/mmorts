package inventory

import (
	"errors"
	"sort"
	"sync"
)

// ItemDetails captures metadata about an item that is useful for clients but
// not required to serialize the inventory state itself.
type ItemDetails struct {
	ID            ItemID            `json:"id"`
	NumericID     RegistryID        `json:"numericId,omitempty"`
	Name          string            `json:"name,omitempty"`
	Category      string            `json:"category,omitempty"`
	Description   string            `json:"description,omitempty"`
	VolumePerUnit int               `json:"volumePerUnit,omitempty"`
	WeightPerUnit int               `json:"weightPerUnit,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}


// Registry stores item details keyed by ItemID and provides numeric handles for
// compact storage.
type Registry struct {
	mu     sync.RWMutex
	items  map[ItemID]ItemDetails
	byID   map[RegistryID]ItemID
	nextID RegistryID
}

// NewRegistry constructs an empty registry and optionally seeds it with
// initial item details.
func NewRegistry(details ...ItemDetails) *Registry {
	r := &Registry{
		items: make(map[ItemID]ItemDetails, len(details)),
		byID:  make(map[RegistryID]ItemID, len(details)),
	}
	for _, d := range details {
		_ = r.RegisterDetails(d) // ignore duplicates during seed
	}
	return r
}

// RegisterItem captures metadata from a RichItem implementation.
func (r *Registry) RegisterItem(item RichItem) error {
	if item == nil {
		return errors.New("inventory: nil item")
	}
	details := item.InventoryItemDetails()
	if details.ID == "" {
		details.ID = item.InventoryItemID()
	}
	if details.ID != item.InventoryItemID() {
		return errors.New("inventory: item details ID mismatch")
	}
	return r.RegisterDetails(details)
}

// RegisterDetails inserts or updates metadata for an item. The ID must be
// non-empty.
func (r *Registry) RegisterDetails(details ItemDetails) error {
	if details.ID == "" {
		return errors.New("inventory: item details missing id")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.items == nil {
		r.items = make(map[ItemID]ItemDetails)
	}
	if r.byID == nil {
		r.byID = make(map[RegistryID]ItemID)
	}

	existing, exists := r.items[details.ID]
	if exists {
		if details.NumericID == 0 {
			details.NumericID = existing.NumericID
		} else if existing.NumericID != 0 && existing.NumericID != details.NumericID {
			return errors.New("inventory: numeric id mismatch for existing item")
		}
	}

	if details.NumericID == 0 {
		r.nextID++
		details.NumericID = r.nextID
	} else {
		if details.NumericID <= 0 {
			return errors.New("inventory: numeric id must be positive")
		}
		if owner, collision := r.byID[details.NumericID]; collision && owner != details.ID {
			return errors.New("inventory: numeric id already assigned to another item")
		}
		if details.NumericID > r.nextID {
			r.nextID = details.NumericID
		}
	}

	r.items[details.ID] = details
	r.byID[details.NumericID] = details.ID
	return nil
}

// Lookup returns details for the provided ID, if present.
func (r *Registry) Lookup(id ItemID) (ItemDetails, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.items == nil {
		return ItemDetails{}, false
	}
	details, ok := r.items[id]
	return details, ok
}

// GetRegistryID returns the numeric registry identifier for the provided item.
func (r *Registry) GetRegistryID(id ItemID) (RegistryID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.items == nil {
		return 0, false
	}
	details, ok := r.items[id]
	if !ok || details.NumericID == 0 {
		return 0, false
	}
	return details.NumericID, true
}

// LookupByRegistryID returns item details using the numeric registry ID.
func (r *Registry) LookupByRegistryID(id RegistryID) (ItemDetails, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.byID == nil {
		return ItemDetails{}, false
	}
	key, ok := r.byID[id]
	if !ok {
		return ItemDetails{}, false
	}
	details, exists := r.items[key]
	return details, exists
}

// VolumeFor returns a volume-per-unit value either from the registry or false
// if not defined.
func (r *Registry) VolumeFor(id ItemID) (int, bool) {
	if r == nil {
		return 0, false
	}
	details, ok := r.Lookup(id)
	if !ok {
		return 0, false
	}
	return details.VolumePerUnit, details.VolumePerUnit > 0
}

// Export copies registry contents into a slice sorted by ItemID, suitable for
// sending to clients.
func (r *Registry) Export() []ItemDetails {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.items) == 0 {
		return nil
	}
	out := make([]ItemDetails, 0, len(r.items))
	for _, d := range r.items {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NumericID != 0 && out[j].NumericID != 0 {
			return out[i].NumericID < out[j].NumericID
		}
		return out[i].ID < out[j].ID
	})
	return out
}
