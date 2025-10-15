package main

import (
	"fmt"
	"time"

	"github.com/gravitas-015/inventory"
	"github.com/gravitas-015/production"
)

// Vec2 represents a 2D position
type Vec2 struct {
	X, Y int
}

// Building represents a completed building in the world
type Building struct {
	ID       string
	Type     string
	Location Vec2
	Owner    string
}

// PendingBuilding tracks a building under construction
type PendingBuilding struct {
	Type     string
	Location Vec2
	JobID    production.JobID
}

// BuildingSystem manages building placement and construction
type BuildingSystem struct {
	productionMgr     *production.Manager
	pendingBuildings  map[production.JobID]*PendingBuilding
	completedBuildings []*Building
	nextBuildingID    int
}

// NewBuildingSystem creates a new building system
func NewBuildingSystem(productionMgr *production.Manager) *BuildingSystem {
	return &BuildingSystem{
		productionMgr:      productionMgr,
		pendingBuildings:   make(map[production.JobID]*PendingBuilding),
		completedBuildings: make([]*Building, 0),
	}
}

// StartConstruction initiates building construction
func (bs *BuildingSystem) StartConstruction(
	buildingType string,
	location Vec2,
	playerID string,
	inventoryID string,
) error {
	// Building system validates placement
	if !bs.canBuildAt(location) {
		return fmt.Errorf("cannot build at location %v", location)
	}

	// Production system handles time and resource costs
	recipeID := production.RecipeID("build_" + buildingType)
	jobID, err := bs.productionMgr.StartProduction(
		recipeID,
		inventory.OwnerID(playerID),
		inventoryID,
	)
	if err != nil {
		return fmt.Errorf("failed to start construction: %w", err)
	}

	// Track construction
	bs.pendingBuildings[jobID] = &PendingBuilding{
		Type:     buildingType,
		Location: location,
		JobID:    jobID,
	}

	fmt.Printf("Construction started: %s at %v (JobID: %s)\n", buildingType, location, jobID)
	return nil
}

// OnProductionEvent handles production completion events
func (bs *BuildingSystem) OnProductionEvent(event production.Event) {
	if event.Type != production.EventJobCompleted {
		return
	}

	pending := bs.pendingBuildings[event.Job.ID]
	if pending == nil {
		return // Not a construction job
	}

	// Construction complete - spawn the actual building!
	building := &Building{
		ID:       fmt.Sprintf("building_%d", bs.nextBuildingID),
		Type:     pending.Type,
		Location: pending.Location,
		Owner:    string(event.Job.Owner),
	}
	bs.nextBuildingID++

	bs.completedBuildings = append(bs.completedBuildings, building)
	delete(bs.pendingBuildings, event.Job.ID)

	fmt.Printf("✓ Building completed: %s at %v (ID: %s)\n", building.Type, building.Location, building.ID)
}

// canBuildAt validates if a building can be placed at location
func (bs *BuildingSystem) canBuildAt(location Vec2) bool {
	// Check if location is already occupied
	for _, building := range bs.completedBuildings {
		if building.Location == location {
			return false
		}
	}
	return true
}

// GetPendingConstructions returns all buildings under construction
func (bs *BuildingSystem) GetPendingConstructions() []*PendingBuilding {
	result := make([]*PendingBuilding, 0, len(bs.pendingBuildings))
	for _, pending := range bs.pendingBuildings {
		result = append(result, pending)
	}
	return result
}

func main() {
	fmt.Println("=== Building Construction Example ===\n")

	// 1. Setup recipe registry with construction recipes
	registry := production.NewRecipeRegistry()

	// Construction recipes have costs but NO item outputs
	// The building system spawns buildings on completion instead
	constructionRecipes := []*production.Recipe{
		{
			ID:   "build_house",
			Name: "House Construction",
			Inputs: []production.ItemRequirement{
				{Item: "wood", Quantity: 50, Consume: true},
				{Item: "stone", Quantity: 20, Consume: true},
			},
			Outputs:  []production.ItemYield{}, // Empty - no items produced!
			Duration: 2 * time.Second,
		},
		{
			ID:   "build_forge",
			Name: "Forge Construction",
			Inputs: []production.ItemRequirement{
				{Item: "stone", Quantity: 100, Consume: true},
				{Item: "iron", Quantity: 50, Consume: true},
			},
			Outputs:  []production.ItemYield{}, // Empty - no items produced!
			Duration: 3 * time.Second,
		},
	}

	for _, recipe := range constructionRecipes {
		if err := registry.Register(recipe); err != nil {
			panic(err)
		}
		fmt.Printf("Registered construction recipe: %s\n", recipe.Name)
	}

	// 2. Setup inventory
	invProvider := production.NewSimpleInventoryProvider()
	playerInv := inventory.NewVolume("player_inv", "player1", 10000)

	// Add building materials
	playerInv.AddStack(inventory.Stack{Item: "wood", Owner: "player1", Qty: 100})
	playerInv.AddStack(inventory.Stack{Item: "stone", Owner: "player1", Qty: 200})
	playerInv.AddStack(inventory.Stack{Item: "iron", Owner: "player1", Qty: 100})

	invProvider.AddInventory(playerInv)

	fmt.Println("\n=== Starting Inventory ===")
	printInventory(playerInv)

	// 3. Setup event bus
	eventBus := production.NewSimpleEventBus()

	// 4. Create production manager
	productionMgr := production.NewManager(
		"construction_queue",
		registry,
		invProvider,
		eventBus,
		nil,
	)

	// 5. Create building system (uses production manager)
	buildingSystem := NewBuildingSystem(productionMgr)

	// Subscribe building system to production events
	eventBus.Subscribe("player1", buildingSystem.OnProductionEvent)

	// 6. Start some constructions
	fmt.Println("\n=== Starting Construction ===")

	err := buildingSystem.StartConstruction("house", Vec2{X: 10, Y: 20}, "player1", "player_inv")
	if err != nil {
		panic(err)
	}

	err = buildingSystem.StartConstruction("forge", Vec2{X: 15, Y: 25}, "player1", "player_inv")
	if err != nil {
		panic(err)
	}

	fmt.Println("\n=== Inventory After Starting Construction ===")
	printInventory(playerInv)

	// 7. Simulate game loop
	fmt.Println("\n=== Construction Progress ===")
	startTime := time.Now()

	for {
		// Update production manager
		productionMgr.Update(time.Now())

		// Check if all constructions done
		if len(buildingSystem.GetPendingConstructions()) == 0 {
			break
		}

		// Print progress
		if time.Since(startTime).Milliseconds()%500 < 20 {
			pending := buildingSystem.GetPendingConstructions()
			if len(pending) > 0 {
				fmt.Printf("\n[Progress] Buildings under construction: %d\n", len(pending))
				for _, p := range pending {
					job := productionMgr.GetJob(p.JobID)
					if job != nil {
						fmt.Printf("  - %s at %v: %.1f%%\n", p.Type, p.Location, job.Progress*100)
					}
				}
			}
		}

		time.Sleep(20 * time.Millisecond)
	}

	// 8. Show final state
	fmt.Println("\n=== Final State ===")
	fmt.Println("\nCompleted Buildings:")
	for _, building := range buildingSystem.completedBuildings {
		fmt.Printf("  - %s (ID: %s) at %v, owned by %s\n",
			building.Type, building.ID, building.Location, building.Owner)
	}

	fmt.Println("\nFinal Inventory:")
	printInventory(playerInv)

	fmt.Println("\n=== Summary ===")
	fmt.Println("✓ Production system handled: time management, resource consumption")
	fmt.Println("✓ Building system handled: placement validation, building spawning")
	fmt.Println("✓ Separation of concerns achieved!")
}

func printInventory(inv *inventory.Inventory) {
	for _, stack := range inv.Stacks {
		fmt.Printf("  %s x%d\n", stack.Item, stack.Qty)
	}
}
