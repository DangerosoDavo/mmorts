package main

import (
	"fmt"
	"time"

	"github.com/gravitas-015/inventory"
	"github.com/gravitas-015/production"
)

func main() {
	fmt.Println("=== Production System Example ===\n")

	// 1. Create recipe registry
	registry := production.NewRecipeRegistry()

	// Register some recipes
	recipes := []*production.Recipe{
		{
			ID:   "iron_sword",
			Name: "Iron Sword",
			Inputs: []production.ItemRequirement{
				{Item: "iron_ingot", Quantity: 3, Consume: true},
				{Item: "hammer", Quantity: 1, Consume: false}, // Tool
			},
			Outputs: []production.ItemYield{
				{Item: "iron_sword", Quantity: 1, Probability: 1.0},
			},
			Duration: 2 * time.Second,
		},
		{
			ID:   "health_potion",
			Name: "Health Potion",
			Inputs: []production.ItemRequirement{
				{Item: "herb", Quantity: 2, Consume: true},
				{Item: "water", Quantity: 1, Consume: true},
			},
			Outputs: []production.ItemYield{
				{Item: "health_potion", Quantity: 1, Probability: 1.0},
			},
			Duration: 1 * time.Second,
		},
	}

	for _, recipe := range recipes {
		if err := registry.Register(recipe); err != nil {
			panic(err)
		}
		fmt.Printf("Registered recipe: %s\n", recipe.Name)
	}

	// 2. Setup inventory
	invProvider := production.NewSimpleInventoryProvider()
	playerInv := inventory.NewVolume("player_inv", "player1", 10000)

	// Add starting materials
	playerInv.AddStack(inventory.Stack{Item: "iron_ingot", Owner: "player1", Qty: 10})
	playerInv.AddStack(inventory.Stack{Item: "hammer", Owner: "player1", Qty: 1})
	playerInv.AddStack(inventory.Stack{Item: "herb", Owner: "player1", Qty: 5})
	playerInv.AddStack(inventory.Stack{Item: "water", Owner: "player1", Qty: 3})

	invProvider.AddInventory(playerInv)

	fmt.Println("\n=== Starting Inventory ===")
	printInventory(playerInv)

	// 3. Setup event bus
	eventBus := production.NewSimpleEventBus()
	eventBus.Subscribe("player1", func(event production.Event) {
		fmt.Printf("\n[Event] %s: Job %s\n", event.Type, event.Job.ID)
	})

	// 4. Create production manager
	mgr := production.NewManager(
		"workshop",
		registry,
		invProvider,
		eventBus,
		nil, // No modifiers for this example
	)

	// 5. Start some production jobs
	fmt.Println("\n=== Starting Production ===")

	jobID1, err := mgr.StartProduction("iron_sword", "player1", "player_inv")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Started iron sword production: %s\n", jobID1)

	jobID2, err := mgr.StartProduction("health_potion", "player1", "player_inv")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Started health potion production: %s\n", jobID2)

	fmt.Println("\n=== Inventory After Starting Jobs ===")
	printInventory(playerInv)

	// 6. Simulate game loop
	fmt.Println("\n=== Simulating Production ===")
	startTime := time.Now()

	for {
		// Update manager
		mgr.Update(time.Now())

		// Check if all jobs done
		if mgr.JobCount() == 0 {
			break
		}

		// Print progress every 500ms
		if time.Since(startTime).Milliseconds()%500 < 20 {
			jobs := mgr.GetActiveJobs("player1")
			if len(jobs) > 0 {
				fmt.Printf("\n[Progress] Active jobs: %d\n", len(jobs))
				for _, job := range jobs {
					job.Progress = job.CalculateProgress(time.Now())
					fmt.Printf("  - %s: %.1f%%\n", job.ID, job.Progress*100)
				}
			}
		}

		time.Sleep(20 * time.Millisecond)
	}

	fmt.Println("\n=== Final Inventory ===")
	printInventory(playerInv)

	fmt.Println("\n=== Summary ===")
	fmt.Println("- Iron ingots: 10 → 7 (3 consumed for sword)")
	fmt.Println("- Hammer: 1 → 1 (tool not consumed)")
	fmt.Println("- Herbs: 5 → 3 (2 consumed for potion)")
	fmt.Println("- Water: 3 → 2 (1 consumed for potion)")
	fmt.Println("+ Iron sword: 0 → 1 (crafted)")
	fmt.Println("+ Health potion: 0 → 1 (crafted)")
}

func printInventory(inv *inventory.Inventory) {
	for _, stack := range inv.Stacks {
		fmt.Printf("  %s x%d\n", stack.Item, stack.Qty)
	}
}
