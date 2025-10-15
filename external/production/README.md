# Production System

A high-performance, scalable production/crafting system for Go game engines. Designed as a **tool library** that integrates with your architecture (ECS, chunk-based worlds, etc.) rather than dictating structure.

## Features

✅ **Composable Tools** - Building blocks you orchestrate, not a framework
✅ **ECS-Friendly** - Managers embed naturally in components
✅ **Extreme Scalability** - Independent managers scale linearly (1M+ jobs)
✅ **Immediate Consumption** - Resources atomically consumed on job start (no exploits)
✅ **Efficiency Modifiers** - Flexible system for buffs/upgrades/skills
✅ **Repeating Jobs** - Jobs that automatically restart until resources run out
✅ **Thread-Safe** - Concurrent manager updates supported
✅ **Zero Dependencies** - Only depends on the inventory package

## Quick Start

```go
import "github.com/gravitas-015/production"

// 1. Create recipe registry
registry := production.NewRecipeRegistry()
registry.Register(&production.Recipe{
    ID:   "iron_sword",
    Name: "Iron Sword",
    Inputs: []production.ItemRequirement{
        {Item: "iron_ingot", Quantity: 3, Consume: true},
        {Item: "hammer", Quantity: 1, Consume: false}, // Tool - not consumed
    },
    Outputs: []production.ItemYield{
        {Item: "iron_sword", Quantity: 1, Probability: 1.0},
    },
    Duration: 5 * time.Second,
})

// 2. Setup inventory provider
invProvider := production.NewSimpleInventoryProvider()
inv := inventory.NewVolume("player_inv", "player1", 1000)
// ... add items to inventory ...
invProvider.AddInventory(inv)

// 3. Create event bus (optional)
eventBus := production.NewSimpleEventBus()
eventBus.Subscribe("player1", func(event production.Event) {
    fmt.Printf("Event: %s for job %s\n", event.Type, event.Job.ID)
})

// 4. Create production manager
mgr := production.NewManager(
    "forge",            // Manager ID
    registry,           // Recipe registry
    invProvider,        // Inventory access
    eventBus,           // Event bus
    nil,                // Modifier sources (optional)
)

// 5. Start production
jobID, err := mgr.StartProduction("iron_sword", "player1", "player_inv")
if err != nil {
    log.Fatal(err) // Insufficient resources, recipe not found, etc.
}

// 6. Update in your game loop
ticker := time.NewTicker(16 * time.Millisecond) // 60 FPS
for t := range ticker.C {
    mgr.Update(t)
}
```

## Core Concepts

### Immediate Resource Consumption

**Critical**: When a job starts, input resources are **immediately removed** from inventory atomically. This prevents:
- Double-spending exploits (can't use same items for multiple jobs)
- Race conditions
- Confusing UI state

```go
// Before StartProduction: 10 iron ingots
jobID, err := mgr.StartProduction("iron_sword", "player1", "inv")
// After StartProduction: 7 iron ingots (3 consumed immediately)
// Job is running, will complete after duration
```

### Recipes

Recipes define transformation rules:

```go
&production.Recipe{
    ID:       "advanced_potion",
    Category: "alchemy",
    Inputs: []production.ItemRequirement{
        {Item: "herb", Quantity: 5, Consume: true},
        {Item: "water", Quantity: 1, Consume: true},
        {Item: "cauldron", Quantity: 1, Consume: false}, // Tool
    },
    Outputs: []production.ItemYield{
        {Item: "health_potion", Quantity: 3, Probability: 1.0},
        {Item: "rare_potion", Quantity: 1, Probability: 0.1}, // 10% chance
    },
    Duration: 10 * time.Second,
}
```

### Efficiency Modifiers

Modify input costs, output yields, and production time:

```go
type BuildingModifier struct {
    Level int
}

func (b *BuildingModifier) GetModifiers(owner inventory.OwnerID, recipe production.RecipeID) production.Modifiers {
    return production.Modifiers{
        InputCost:   1.0 - (float64(b.Level) * 0.05),  // 5% reduction per level
        OutputYield: 1.0 + (float64(b.Level) * 0.10),  // 10% bonus per level
        TimeSpeed:   1.0 - (float64(b.Level) * 0.10),  // 10% faster per level
        Source:      fmt.Sprintf("Building Lvl %d", b.Level),
    }
}

// Register with manager
mgr := production.NewManager(
    "forge",
    registry,
    invProvider,
    eventBus,
    []production.ModifierSource{
        &BuildingModifier{Level: 3},
        &SkillModifier{Smithing: 10},
    },
)
```

### Repeating Jobs

Jobs can automatically restart after completion until resources are exhausted:

```go
// Start a repeating job
jobID, err := mgr.StartRepeatingProduction("mine_iron", "player1", "mine_inv")
if err != nil {
    log.Fatal(err)
}

// Job will:
// 1. Consume inputs and produce outputs
// 2. Automatically attempt to restart with new cycle
// 3. Continue until resources run out or job is cancelled
// 4. Track cycles completed via job.CyclesCompleted

// Monitor cycles via events
eventBus.Subscribe("player1", func(event production.Event) {
    if event.Type == production.EventJobCompleted {
        cycles := event.Data["cyclesCompleted"].(int)
        fmt.Printf("Completed cycle %d\n", cycles)
    }
    if event.Type == production.EventJobFailed {
        if event.Data["reason"] == "failed_to_restart" {
            cycles := event.Data["cyclesCompleted"].(int)
            fmt.Printf("Mining stopped after %d cycles (resources exhausted)\n", cycles)
        }
    }
})

// Manually stop a repeating job
mgr.CancelProduction(jobID)
```

## Integration Patterns

### ECS Integration

Embed managers in components:

```go
type ProductionComponent struct {
    Manager *production.Manager
}

// System creates managers
type ChunkInitSystem struct {
    registry *production.RecipeRegistry
    // ... shared services
}

func (s *ChunkInitSystem) OnChunkCreated(entity ecs.Entity, chunk *ChunkComponent) {
    mgr := production.NewManager(
        fmt.Sprintf("chunk_%d_%d", chunk.X, chunk.Y),
        s.registry,
        s.inventories,
        s.eventBus,
        s.modifiers,
    )
    entity.Add(&ProductionComponent{Manager: mgr})
}

// System ticks production
type ProductionTickSystem struct {}

func (s *ProductionTickSystem) Update(world *ecs.World, now time.Time) {
    world.Query(func(prod *ProductionComponent) {
        if prod.Manager != nil {
            prod.Manager.Update(now)
        }
    })
}
```

### Chunk-Based World

```go
type Chunk struct {
    X, Y            int
    ProductionMgr   *production.Manager
}

func (c *Chunk) Initialize(registry *production.RecipeRegistry, ...) {
    c.ProductionMgr = production.NewManager(
        fmt.Sprintf("chunk_%d_%d", c.X, c.Y),
        registry, inventories, eventBus, modifiers,
    )
}

// Selective ticking based on player proximity
for _, chunk := range world.LoadedChunks() {
    if nearPlayer(chunk) {
        chunk.ProductionMgr.Update(time.Now())
    }
}
```

### Building-Based

```go
type Building struct {
    ID              string
    ProductionMgr   *production.Manager
}

func (b *Building) Tick(now time.Time) {
    if b.ProductionMgr != nil {
        b.ProductionMgr.Update(now)
    }
}
```

## API Reference

### Manager

```go
// Create manager
NewManager(id, registry, inventories, eventBus, modifierSources) *Manager

// Production
StartProduction(recipeID, ownerID, inventoryID) (JobID, error)
StartRepeatingProduction(recipeID, ownerID, inventoryID) (JobID, error)
CancelProduction(jobID) error
CancelProductionWithRefund(jobID) error

// Updates (call from game loop)
Update(now time.Time)

// Queries
GetJob(jobID) *Job
GetActiveJobs(ownerID) []*Job
GetAllJobs() []*Job
JobCount() int
ID() string
```

### RecipeRegistry

```go
NewRecipeRegistry() *RecipeRegistry

Register(recipe *Recipe) error
Lookup(id RecipeID) *Recipe
GetByCategory(category string) []RecipeID
GetByOutput(item inventory.ItemID) []RecipeID
GetAll() []*Recipe
Remove(id RecipeID) bool
Clear()
Count() int
```

### EventBus

```go
NewSimpleEventBus() *SimpleEventBus
NewNullEventBus() *NullEventBus // No-op for when events not needed

Subscribe(owner inventory.OwnerID, handler func(Event))
Unsubscribe(owner inventory.OwnerID)
Publish(event Event)
```

### InventoryProvider

```go
type InventoryProvider interface {
    GetInventory(id string) (*inventory.Inventory, error)
    ConsumeItems(inv *inventory.Inventory, items []ItemRequirement) error
    AddItems(inv *inventory.Inventory, items []ItemYield) error
}

// Simple implementation provided
NewSimpleInventoryProvider() *SimpleInventoryProvider
```

## Performance

- **10K+ jobs per manager**: <1ms update time
- **1M+ total jobs**: Distributed across independent managers
- **O(log n)** job completion detection via min-heap
- **Thread-safe**: Concurrent updates across managers
- **~100 bytes per job**: Memory efficient

## Thread Safety

All public methods are thread-safe. You can:
- Update multiple managers concurrently from different goroutines
- Query jobs while updates are happening
- Register recipes while managers are running

## Testing

```bash
cd production
go test -v
go test -bench=.
```

## Examples

See `examples/` directory for:
- `basic_usage.go` - Basic crafting system
- `building_construction.go` - Effect-only recipes (building construction)
- `repeating_jobs.go` - Mining operation that repeats until resources exhausted

## Architecture

For detailed architecture documentation, see [ARCHITECTURE.md](ARCHITECTURE.md).

## License

MIT
