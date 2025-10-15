package inventory

// SampleInventory returns a small, sci‑fi flavored inventory showcasing
// volume and grid modes using the placeholder item catalog IDs. A registry
// is provided so clients can resolve item metadata separately from the
// serialized inventory contents.
func SampleInventory() (*Inventory, *Inventory, *Registry) {
	reg := NewRegistry(
		ItemDetails{ID: ItemID("smartmatter"), NumericID: 1, Name: "Smart Matter Feedstock", Category: "resource", VolumePerUnit: 2},
		ItemDetails{ID: ItemID("diamondite"), NumericID: 2, Name: "Diamondite Bulk", Category: "resource", VolumePerUnit: 8},
		ItemDetails{ID: ItemID("energy-cell"), NumericID: 3, Name: "Energy Cell Pack", Category: "resource", VolumePerUnit: 1},
		ItemDetails{ID: ItemID("nanoforge"), NumericID: 4, Name: "Nanoforge Unit", Category: "module"},
		ItemDetails{ID: ItemID("knife-missile"), NumericID: 5, Name: "Knife Missile", Category: "weapon"},
		ItemDetails{ID: ItemID("field-projector"), NumericID: 6, Name: "Field Projector", Category: "module"},
		ItemDetails{ID: ItemID("gridfire-projector"), NumericID: 7, Name: "Gridfire Projector", Category: "weapon"},
		ItemDetails{ID: ItemID("drone-bay"), NumericID: 8, Name: "Drone Bay", Category: "module"},
	)

	owner := OwnerID("demo")

	// Volume-only: capacity 100 units
	vol := NewVolume("inv-vol-1", owner, 100, WithRegistry(reg))
	_ = vol.AddStack(Stack{Item: ItemID("smartmatter"), Owner: owner, Qty: 10}) // 20
	_ = vol.AddStack(Stack{Item: ItemID("diamondite"), Owner: owner, Qty: 3})   // +24 => 44
	_ = vol.AddStack(Stack{Item: ItemID("energy-cell"), Owner: owner, Qty: 20}) // +20 => 64

	// Grid-only: 6x4 grid with a few placed shapes
	grid := NewGrid("inv-grid-1", owner, 6, 4, WithRegistry(reg))
	// 3x2 Nanoforge at (0,0)
	_ = grid.AddStack(Stack{Item: ItemID("nanoforge"), Owner: owner, Qty: 1, Shape: &Shape{Width: 3, Height: 2}, Position: &Point{X: 0, Y: 0}})
	// Knife Missile (1x2) at (3,0)
	_ = grid.AddStack(Stack{Item: ItemID("knife-missile"), Owner: owner, Qty: 1, Shape: &Shape{Width: 1, Height: 2}, Position: &Point{X: 3, Y: 0}})
	// Field Projector (2x1) at (4,2)
	_ = grid.AddStack(Stack{Item: ItemID("field-projector"), Owner: owner, Qty: 1, Shape: &Shape{Width: 2, Height: 1}, Position: &Point{X: 4, Y: 2}})
	// Gridfire Projector (2x2) at (0,2)
	_ = grid.AddStack(Stack{Item: ItemID("gridfire-projector"), Owner: owner, Qty: 1, Shape: &Shape{Width: 2, Height: 2}, Position: &Point{X: 0, Y: 2}})
	// Drone Bay: L shape, auto-placed
	lShape := Shape{Cells: []Point{{0, 0}, {0, 1}, {1, 1}}}
	_ = grid.AddStack(Stack{Item: ItemID("drone-bay"), Owner: owner, Qty: 1, Shape: &lShape})

	return vol, grid, reg
}
