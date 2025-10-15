package production

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gravitas-015/inventory"
)

// SimpleInventoryProvider is a basic implementation of InventoryProvider.
// It stores inventories in memory and provides atomic operations.
type SimpleInventoryProvider struct {
	mu          sync.RWMutex
	inventories map[string]*inventory.Inventory
}

// NewSimpleInventoryProvider creates a new simple inventory provider.
func NewSimpleInventoryProvider() *SimpleInventoryProvider {
	return &SimpleInventoryProvider{
		inventories: make(map[string]*inventory.Inventory),
	}
}

// AddInventory registers an inventory with the provider.
func (p *SimpleInventoryProvider) AddInventory(inv *inventory.Inventory) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inventories[inv.ID] = inv
}

// GetInventory retrieves an inventory by ID.
func (p *SimpleInventoryProvider) GetInventory(id string) (*inventory.Inventory, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	inv, exists := p.inventories[id]
	if !exists {
		return nil, fmt.Errorf("inventory not found: %s", id)
	}

	return inv, nil
}

// ConsumeItems atomically removes items from inventory.
// Only consumes items where ItemRequirement.Consume == true.
// For non-consumed items (tools), validates existence but doesn't remove.
func (p *SimpleInventoryProvider) ConsumeItems(inv *inventory.Inventory, items []ItemRequirement) error {
	if inv == nil {
		return errors.New("inventory is nil")
	}

	// First pass: validate all items exist
	for _, req := range items {
		available := p.countItem(inv, req.Item)
		if available < req.Quantity {
			return fmt.Errorf("insufficient %s: have %d, need %d", req.Item, available, req.Quantity)
		}
	}

	// Second pass: consume only items marked for consumption
	for _, req := range items {
		if req.Consume {
			if err := p.removeItem(inv, req.Item, req.Quantity); err != nil {
				// Should never happen due to validation above
				return fmt.Errorf("failed to consume %s: %w", req.Item, err)
			}
		}
	}

	return nil
}

// AddItems adds produced items to inventory.
func (p *SimpleInventoryProvider) AddItems(inv *inventory.Inventory, items []ItemYield) error {
	if inv == nil {
		return errors.New("inventory is nil")
	}

	for _, yield := range items {
		if yield.Quantity <= 0 {
			continue
		}

		stack := inventory.Stack{
			Item:  yield.Item,
			Owner: inv.Owner,
			Qty:   yield.Quantity,
		}

		if err := inv.AddStack(stack); err != nil {
			return fmt.Errorf("failed to add %s x%d: %w", yield.Item, yield.Quantity, err)
		}
	}

	return nil
}

// countItem counts the total quantity of an item in the inventory.
func (p *SimpleInventoryProvider) countItem(inv *inventory.Inventory, itemID inventory.ItemID) int {
	total := 0
	for _, stack := range inv.Stacks {
		if stack.Item == itemID {
			total += stack.Qty
		}
	}
	return total
}

// removeItem removes a quantity of an item from the inventory.
func (p *SimpleInventoryProvider) removeItem(inv *inventory.Inventory, itemID inventory.ItemID, quantity int) error {
	remaining := quantity

	for i := 0; i < len(inv.Stacks) && remaining > 0; i++ {
		stack := &inv.Stacks[i]
		if stack.Item != itemID {
			continue
		}

		if stack.Qty <= remaining {
			// Remove entire stack
			removeQty := stack.Qty
			if err := inv.RemoveStack(i, removeQty); err != nil {
				return err
			}
			remaining -= removeQty
			i-- // Adjust index since we removed a stack
		} else {
			// Partial removal
			if err := inv.RemoveStack(i, remaining); err != nil {
				return err
			}
			remaining = 0
		}
	}

	if remaining > 0 {
		return fmt.Errorf("could not remove full quantity of %s: %d remaining", itemID, remaining)
	}

	return nil
}
