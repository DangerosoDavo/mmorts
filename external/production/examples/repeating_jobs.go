package main

import (
	"fmt"
	"time"

	"github.com/gravitas-015/inventory"
	"github.com/gravitas-015/production"
)

func main() {
	fmt.Println("=== Repeating Production Jobs Example ===\n")

	// 1. Setup recipe - a simple resource generator
	registry := production.NewRecipeRegistry()

	registry.Register(&production.Recipe{
		ID:   "mine_iron",
		Name: "Mine Iron Ore",
		Inputs: []production.ItemRequirement{
			{Item: "pickaxe", Quantity: 1, Consume: false}, // Tool - not consumed
			{Item: "energy", Quantity: 10, Consume: true},  // Energy consumed each cycle
		},
		Outputs: []production.ItemYield{
			{Item: "iron_ore", Quantity: 5, Probability: 1.0},
		},
		Duration: 1 * time.Second,
	})

	// 2. Setup inventory
	invProvider := production.NewSimpleInventoryProvider()
	playerInv := inventory.NewVolume("player_inv", "player1", 10000)

	// Add starting resources
	playerInv.AddStack(inventory.Stack{Item: "pickaxe", Owner: "player1", Qty: 1})
	playerInv.AddStack(inventory.Stack{Item: "energy", Owner: "player1", Qty: 100}) // Enough for 10 cycles

	invProvider.AddInventory(playerInv)

	fmt.Println("=== Starting Inventory ===")
	printInventory(playerInv)

	// 3. Setup event bus
	eventBus := production.NewSimpleEventBus()

	var completionCount int
	eventBus.Subscribe("player1", func(event production.Event) {
		switch event.Type {
		case production.EventJobStarted:
			isRestart := false
			if event.Data != nil {
				if val, ok := event.Data["isRestart"].(bool); ok {
					isRestart = val
				}
			}
			if isRestart {
				cycles := 0
				if event.Data != nil {
					if val, ok := event.Data["cyclesCompleted"].(int); ok {
						cycles = val
					}
				}
				fmt.Printf("[Cycle %d] Job restarted\n", cycles+1)
			} else {
				fmt.Printf("[Start] Job %s started (repeating)\n", event.Job.ID)
			}
		case production.EventJobCompleted:
			completionCount++
			cycles := 0
			if event.Data != nil {
				if val, ok := event.Data["cyclesCompleted"].(int); ok {
					cycles = val
				}
			}
			fmt.Printf("[Cycle %d Complete] Produced: ", cycles)
			for i, output := range event.Job.EffectiveOutputs {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s x%d", output.Item, output.Quantity)
			}
			fmt.Println()
		case production.EventJobFailed:
			reason := "unknown"
			if event.Data != nil {
				if val, ok := event.Data["reason"].(string); ok {
					reason = val
				}
			}
			cycles := 0
			if event.Data != nil {
				if val, ok := event.Data["cyclesCompleted"].(int); ok {
					cycles = val
				}
			}
			fmt.Printf("[Stopped] Job stopped after %d cycles (reason: %s)\n", cycles, reason)
		}
	})

	// 4. Create production manager
	mgr := production.NewManager(
		"mining_site",
		registry,
		invProvider,
		eventBus,
		nil,
	)

	// 5. Start REPEATING production job
	fmt.Println("\n=== Starting Repeating Mining Job ===")
	jobID, err := mgr.StartRepeatingProduction("mine_iron", "player1", "player_inv")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Job ID: %s\n\n", jobID)

	// 6. Simulate game loop - job will run until it runs out of energy
	fmt.Println("=== Mining Progress ===")
	startTime := time.Now()

	for {
		mgr.Update(time.Now())

		// Check if job still exists
		job := mgr.GetJob(jobID)
		if job == nil {
			fmt.Println("\n[Job Complete] Mining job stopped")
			break
		}

		// Print status every second
		if time.Since(startTime).Milliseconds()%1000 < 20 {
			fmt.Printf("[Status] Cycles: %d, Progress: %.1f%%\n",
				job.CyclesCompleted,
				job.Progress*100,
			)
		}

		// Safety: stop after 15 seconds (should naturally stop around 10 cycles)
		if time.Since(startTime) > 15*time.Second {
			fmt.Println("\n[Timeout] Stopping for demo purposes")
			mgr.CancelProduction(jobID)
			break
		}

		time.Sleep(20 * time.Millisecond)
	}

	// 7. Show final state
	fmt.Println("\n=== Final Inventory ===")
	printInventory(playerInv)

	// Calculate results
	energyUsed := 100 - countItem(playerInv, "energy")
	ironOreProduced := countItem(playerInv, "iron_ore")

	fmt.Println("\n=== Summary ===")
	fmt.Printf("Total cycles completed: %d\n", completionCount)
	fmt.Printf("Energy consumed: %d (10 per cycle)\n", energyUsed)
	fmt.Printf("Iron ore produced: %d (5 per cycle)\n", ironOreProduced)
	fmt.Println("Pickaxe still in inventory: ✓ (tool not consumed)")
	fmt.Println("\nJob automatically stopped when energy ran out!")
}

func printInventory(inv *inventory.Inventory) {
	for _, stack := range inv.Stacks {
		fmt.Printf("  %s x%d\n", stack.Item, stack.Qty)
	}
}

func countItem(inv *inventory.Inventory, itemID inventory.ItemID) int {
	total := 0
	for _, stack := range inv.Stacks {
		if stack.Item == itemID {
			total += stack.Qty
		}
	}
	return total
}
