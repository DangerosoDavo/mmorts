# Inventory Package

A flexible, constraint-based inventory system for Go applications supporting multiple capacity models and storage optimization.

## Overview

The inventory package provides a minimal, item-agnostic inventory system that tracks item identifiers, owners, quantities, and optional capacity/placement constraints. It supports volume-based, grid-based, and hybrid inventory models with efficient serialization formats optimized for both human readability and database storage.

## Features

- **Multiple Constraint Modes**: Volume, grid, hybrid, or no constraints
- **Flexible Item Shapes**: Rectangular or custom Tetris-like shapes for grid inventories
- **Registry System**: Item metadata management with numeric IDs for efficient storage
- **Dual Serialization**: Human-readable JSON and storage-optimized formats
- **Thread-Safe Operations**: Concurrent access to registries
- **Auto-Placement**: Automatic item placement in grid inventories

## Installation

```go
import "github.com/gravitas-015/inventory"
```

## Quick Start

### Recommended Pattern: Shared Application Registry

The most efficient approach is to create a single application-level registry shared across all inventories:

```go
// Application-level registry (typically a global variable or singleton)
var AppRegistry = inventory.NewRegistry(
    inventory.ItemDetails{ID: inventory.ItemID("health-potion"), VolumePerUnit: 2, Name: "Health Potion"},
    inventory.ItemDetails{ID: inventory.ItemID("sword"), VolumePerUnit: 5, Name: "Iron Sword"},
    inventory.ItemDetails{ID: inventory.ItemID("armor"), VolumePerUnit: 8, Name: "Steel Armor"},
    // ... all your game items
)

// Multiple inventories sharing the same registry
playerBackpack := inventory.NewVolume("player-backpack", inventory.OwnerID("player1"), 100, 
    inventory.WithRegistry(AppRegistry))

playerEquipment := inventory.NewGrid("player-equipment", inventory.OwnerID("player1"), 8, 6,
    inventory.WithRegistry(AppRegistry))

shipCargo := inventory.NewHybrid("ship-cargo", inventory.OwnerID("ship1"), 1000, 20, 15,
    inventory.WithRegistry(AppRegistry))
```

**Benefits of Shared Registry:**
- **Memory Efficient**: Item metadata stored once, not per inventory
- **Consistent**: All inventories use identical item definitions  
- **Database Optimized**: Consistent numeric IDs across all inventories
- **Thread-Safe**: Concurrent access across inventories is safe
- **No Conflicts**: Each inventory maintains independent state

### Basic Volume Inventory

```go
// Add items to any inventory using the shared registry
err := playerBackpack.AddStack(inventory.Stack{
    Item: inventory.ItemID("health-potion"),
    Owner: inventory.OwnerID("player1"),
    Qty: 5, // Uses 10 volume units
})
```

### Grid-Based Inventory

```go
// Create 8x6 grid inventory
inv := inventory.NewGrid("player-equipment", inventory.OwnerID("player1"), 8, 6, 
    inventory.WithRegistry(reg))

// Add a 2x3 sword
err := inv.AddStack(inventory.Stack{
    Item: inventory.ItemID("two-handed-sword"),
    Owner: inventory.OwnerID("player1"),
    Qty: 1,
    Shape: &inventory.Shape{Width: 2, Height: 3},
    Position: &inventory.Point{X: 0, Y: 0}, // Optional: auto-place if nil
})

// Add an L-shaped item
lShape := inventory.Shape{
    Cells: []inventory.Point{{0, 0}, {0, 1}, {1, 1}},
}
err = inv.AddStack(inventory.Stack{
    Item: inventory.ItemID("boomerang"),
    Owner: inventory.OwnerID("player1"),
    Qty: 1,
    Shape: &lShape, // Auto-placed
})
```

### Hybrid Inventory (Volume + Grid)

```go
// Both volume (200 units) and grid (10x8) constraints
inv := inventory.NewHybrid("ship-cargo", inventory.OwnerID("ship1"), 200, 10, 8, 
    inventory.WithRegistry(reg))
```

## Inventory Modes

| Mode | Description | Constraints |
|------|-------------|-------------|
| `ModeNone` | No constraints | Accepts all stacks |
| `ModeVolume` | Volume-based | Total volume ≤ capacity |
| `ModeGrid` | Grid-based | Placement within grid bounds |
| `ModeBoth` | Hybrid | Both volume and grid constraints |

## Item Registry

The registry provides metadata storage and numeric ID mapping for efficient database storage. **Use a single shared registry across your entire application.**

### Application-Level Registry Setup

```go
// items/registry.go - Central item definitions
package items

import "github.com/gravitas-015/inventory"

var GlobalRegistry = inventory.NewRegistry()

func init() {
    // Load all game items at application startup
    registerWeapons()
    registerArmor()
    registerConsumables()
}

func registerWeapons() {
    GlobalRegistry.RegisterDetails(inventory.ItemDetails{
        ID: inventory.ItemID("plasma-rifle"),
        NumericID: 42, // Optional: auto-assigned if omitted
        Name: "Plasma Rifle MkIII",
        Category: "weapon",
        VolumePerUnit: 15,
        WeightPerUnit: 8,
        Attributes: map[string]string{
            "damage": "high",
            "range": "long",
        },
    })
    // ... register more weapons
}

// player/service.go - Use shared registry
func NewPlayerInventories(playerID string) (*inventory.Inventory, *inventory.Inventory) {
    owner := inventory.OwnerID(playerID)
    
    backpack := inventory.NewVolume("backpack-"+playerID, owner, 200,
        inventory.WithRegistry(items.GlobalRegistry))
        
    equipment := inventory.NewGrid("equipment-"+playerID, owner, 10, 8,
        inventory.WithRegistry(items.GlobalRegistry))
        
    return backpack, equipment
}
```

### Registry Operations

```go
// Look up items from shared registry
details, found := items.GlobalRegistry.Lookup(inventory.ItemID("plasma-rifle"))
numericID, found := items.GlobalRegistry.GetRegistryID(inventory.ItemID("plasma-rifle"))

// Export all items (useful for client item catalogs)
allItems := items.GlobalRegistry.Export()
```

## Serialization

### Human-Readable Format (APIs, Debugging)

```go
// Serialize using string ItemIDs
data, err := inv.Serialize()

// Deserialize
var restored inventory.Inventory
restored.SetRegistry(reg)
err = restored.Deserialize(data)
```

### Storage-Optimized Format (Database)

For thousands of inventory records, use numeric IDs for better performance:

```go
// Serialize using numeric RegistryIDs - requires registry
data, err := inv.SerializeForStorage()

// Deserialize from storage format
var restored inventory.Inventory
restored.SetRegistry(reg)
err = restored.DeserializeFromStorage(data)
```

**Storage Benefits:**
- Smaller payload size (int64 vs string)
- Better database indexing performance
- Reduced storage costs

## Item Implementation

Items must implement the `StoredItem` interface:

```go
type GameItem struct {
    ID string
    // ... other fields
}

func (g GameItem) InventoryItemID() inventory.ItemID {
    return inventory.ItemID(g.ID)
}

// Optional: implement RichItem for automatic registry population
func (g GameItem) InventoryItemDetails() inventory.ItemDetails {
    return inventory.ItemDetails{
        ID: inventory.ItemID(g.ID),
        Name: g.DisplayName,
        // ... other metadata
    }
}
```

## Advanced Features

### Custom Shapes

```go
// T-shaped item
tShape := inventory.Shape{
    Cells: []inventory.Point{
        {1, 0}, // Top center
        {0, 1}, {1, 1}, {2, 1}, // Bottom row
    },
}
```

### Stack Management

```go
// Remove partial quantity
err := inv.RemoveStack(stackIndex, quantity)

// Access stacks directly
for i, stack := range inv.Stacks {
    fmt.Printf("Stack %d: %s x%d\n", i, stack.Item, stack.Qty)
}
```

### Occupancy Tracking

Grid inventories automatically track cell occupancy for collision detection and provide placement validation.

## Error Handling

Common errors:
- `"quantity must be positive"` - Invalid stack quantity
- `"volume exceeded"` - Insufficient volume capacity  
- `"no space available for shape"` - Cannot fit item in grid
- `"cannot place at requested position"` - Grid collision or out of bounds
- `"registry required for storage serialization"` - Missing registry for numeric serialization

## Examples

See `sample.go` for a complete example with sci-fi themed items demonstrating both volume and grid inventories.

## Performance Considerations

- Use `SerializeForStorage()` for database persistence (numeric IDs)
- Use `Serialize()` for API responses (readable string IDs)
- Registry lookups are O(1) with read-write locks
- Grid placement uses efficient collision detection
- Auto-placement scans grid row-major for first fit

## Thread Safety

- Registry operations are thread-safe (uses sync.RWMutex)
- Inventory instances are not thread-safe - use external synchronization if needed

## License

This package is part of the gravitas-015 project.