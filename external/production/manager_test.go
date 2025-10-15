package production

import (
	"testing"
	"time"

	"github.com/gravitas-015/inventory"
)

func TestBasicProduction(t *testing.T) {
	// Setup registry
	registry := NewRecipeRegistry()

	// Register a simple recipe
	err := registry.Register(&Recipe{
		ID:   "iron_sword",
		Name: "Iron Sword",
		Inputs: []ItemRequirement{
			{Item: "iron_ingot", Quantity: 3, Consume: true},
		},
		Outputs: []ItemYield{
			{Item: "iron_sword", Quantity: 1, Probability: 1.0},
		},
		Duration: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to register recipe: %v", err)
	}

	// Setup inventory
	invProvider := NewSimpleInventoryProvider()
	inv := inventory.NewVolume("test_inv", "player1", 1000)

	// Add iron ingots to inventory
	err = inv.AddStack(inventory.Stack{
		Item:  "iron_ingot",
		Owner: "player1",
		Qty:   10,
	})
	if err != nil {
		t.Fatalf("Failed to add items to inventory: %v", err)
	}

	invProvider.AddInventory(inv)

	// Setup event bus
	eventBus := NewSimpleEventBus()
	completedChan := make(chan Event, 1)

	eventBus.Subscribe("player1", func(e Event) {
		if e.Type == EventJobCompleted {
			completedChan <- e
		}
	})

	// Create manager
	mgr := NewManager(
		"test_manager",
		registry,
		invProvider,
		eventBus,
		nil, // No modifiers
	)

	// Start production
	jobID, err := mgr.StartProduction("iron_sword", "player1", "test_inv")
	if err != nil {
		t.Fatalf("Failed to start production: %v", err)
	}

	// Verify items were consumed immediately
	remainingIron := 0
	for _, stack := range inv.Stacks {
		if stack.Item == "iron_ingot" {
			remainingIron += stack.Qty
		}
	}
	if remainingIron != 7 {
		t.Errorf("Expected 7 iron ingots remaining, got %d", remainingIron)
	}

	// Verify job exists and is running
	job := mgr.GetJob(jobID)
	if job == nil {
		t.Fatal("Job not found")
	}
	if job.State != JobRunning {
		t.Errorf("Expected job state Running, got %s", job.State)
	}

	// Wait for job to complete
	time.Sleep(150 * time.Millisecond)

	// Update manager to process completed jobs
	mgr.Update(time.Now())

	// Wait for event
	select {
	case event := <-completedChan:
		if event.Job.ID != jobID {
			t.Errorf("Expected job ID %s, got %s", jobID, event.Job.ID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for job completion event")
	}

	// Verify iron sword was added to inventory
	hasSword := false
	for _, stack := range inv.Stacks {
		if stack.Item == "iron_sword" && stack.Qty > 0 {
			hasSword = true
			break
		}
	}
	if !hasSword {
		t.Error("Iron sword was not added to inventory")
	}

	// Verify job no longer exists in manager
	job = mgr.GetJob(jobID)
	if job != nil {
		t.Error("Job should be removed after completion")
	}
}

func TestInsufficientResources(t *testing.T) {
	// Setup
	registry := NewRecipeRegistry()
	registry.Register(&Recipe{
		ID: "expensive_item",
		Inputs: []ItemRequirement{
			{Item: "rare_material", Quantity: 100, Consume: true},
		},
		Outputs: []ItemYield{
			{Item: "super_sword", Quantity: 1, Probability: 1.0},
		},
		Duration: 1 * time.Second,
	})

	invProvider := NewSimpleInventoryProvider()
	inv := inventory.NewVolume("test_inv", "player1", 1000)

	// Add only 10 rare materials (need 100)
	inv.AddStack(inventory.Stack{
		Item:  "rare_material",
		Owner: "player1",
		Qty:   10,
	})

	invProvider.AddInventory(inv)

	mgr := NewManager(
		"test_manager",
		registry,
		invProvider,
		NewNullEventBus(),
		nil,
	)

	// Attempt production - should fail
	_, err := mgr.StartProduction("expensive_item", "player1", "test_inv")
	if err == nil {
		t.Fatal("Expected error for insufficient resources")
	}

	// Verify no items were consumed
	remaining := 0
	for _, stack := range inv.Stacks {
		if stack.Item == "rare_material" {
			remaining += stack.Qty
		}
	}
	if remaining != 10 {
		t.Errorf("Expected 10 rare materials remaining, got %d", remaining)
	}
}

func TestNonConsumedItems(t *testing.T) {
	// Setup recipe with tool (non-consumed)
	registry := NewRecipeRegistry()
	registry.Register(&Recipe{
		ID: "crafted_item",
		Inputs: []ItemRequirement{
			{Item: "wood", Quantity: 2, Consume: true},
			{Item: "hammer", Quantity: 1, Consume: false}, // Tool
		},
		Outputs: []ItemYield{
			{Item: "plank", Quantity: 4, Probability: 1.0},
		},
		Duration: 50 * time.Millisecond,
	})

	invProvider := NewSimpleInventoryProvider()
	inv := inventory.NewVolume("test_inv", "player1", 1000)

	// Add materials and tool
	inv.AddStack(inventory.Stack{Item: "wood", Owner: "player1", Qty: 5})
	inv.AddStack(inventory.Stack{Item: "hammer", Owner: "player1", Qty: 1})

	invProvider.AddInventory(inv)

	mgr := NewManager(
		"test_manager",
		registry,
		invProvider,
		NewNullEventBus(),
		nil,
	)

	// Start production
	_, err := mgr.StartProduction("crafted_item", "player1", "test_inv")
	if err != nil {
		t.Fatalf("Failed to start production: %v", err)
	}

	// Verify wood was consumed but hammer wasn't
	woodCount := 0
	hammerCount := 0
	for _, stack := range inv.Stacks {
		if stack.Item == "wood" {
			woodCount += stack.Qty
		}
		if stack.Item == "hammer" {
			hammerCount += stack.Qty
		}
	}

	if woodCount != 3 {
		t.Errorf("Expected 3 wood remaining, got %d", woodCount)
	}
	if hammerCount != 1 {
		t.Errorf("Expected 1 hammer remaining (tool not consumed), got %d", hammerCount)
	}
}
