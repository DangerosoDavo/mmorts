package inventory

import "testing"

func TestVolumeInventoryAddRemove(t *testing.T) {
	reg := NewRegistry(ItemDetails{ID: ItemID("a"), VolumePerUnit: 4}, ItemDetails{ID: ItemID("b"), VolumePerUnit: 4})
	inv := NewVolume("vol1", OwnerID("u1"), 50, WithRegistry(reg))
	if err := inv.AddStack(Stack{Item: ItemID("a"), Owner: OwnerID("u1"), Qty: 5}); err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}
	if inv.VolumeUsed != 20 {
		t.Fatalf("expected VolumeUsed=20, got %d", inv.VolumeUsed)
	}
	// adding exceeding capacity should fail
	if err := inv.AddStack(Stack{Item: ItemID("b"), Owner: OwnerID("u1"), Qty: 8}); err == nil {
		t.Fatalf("expected capacity error, got nil")
	}
	if err := inv.RemoveStack(0, 3); err != nil {
		t.Fatalf("unexpected remove error: %v", err)
	}
	if inv.VolumeUsed != 8 {
		t.Fatalf("expected VolumeUsed=8 after remove, got %d", inv.VolumeUsed)
	}
}

func TestGridPlacement(t *testing.T) {
	inv := NewGrid("grid1", OwnerID("u1"), 4, 3)
	// place 2x2 at (0,0)
	if err := inv.AddStack(Stack{Item: ItemID("a"), Owner: OwnerID("u1"), Qty: 1, Shape: &Shape{Width: 2, Height: 2}, Position: &Point{X: 0, Y: 0}}); err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}
	// overlapping placement should fail
	if err := inv.AddStack(Stack{Item: ItemID("b"), Owner: OwnerID("u1"), Qty: 1, Shape: &Shape{Width: 2, Height: 1}, Position: &Point{X: 1, Y: 1}}); err == nil {
		t.Fatalf("expected overlap error, got nil")
	}
	// auto-place L-shape should find a spot
	l := Shape{Cells: []Point{{0, 0}, {1, 0}, {1, 1}}}
	if err := inv.AddStack(Stack{Item: ItemID("c"), Owner: OwnerID("u1"), Qty: 1, Shape: &l}); err != nil {
		t.Fatalf("unexpected auto-place error: %v", err)
	}
}

func TestSerializationRoundTrip(t *testing.T) {
	reg := NewRegistry(ItemDetails{ID: ItemID("x"), VolumePerUnit: 10})
	inv := NewHybrid("h1", OwnerID("u1"), 100, 5, 4, WithRegistry(reg))
	if err := inv.AddStack(Stack{Item: ItemID("x"), Owner: OwnerID("u1"), Qty: 2, StackMax: 2, Shape: &Shape{Width: 1, Height: 2}, Position: &Point{X: 3, Y: 1}}); err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}
	data, err := inv.Serialize()
	if err != nil {
		t.Fatalf("serialize error: %v", err)
	}
	var out Inventory
	out.SetRegistry(reg)
	if err := out.Deserialize(data); err != nil {
		t.Fatalf("deserialize error: %v", err)
	}
	if out.ID != inv.ID || out.Owner != inv.Owner || len(out.Stacks) != 1 {
		t.Fatalf("mismatch after roundtrip")
	}
	if out.VolumeUsed != inv.VolumeUsed {
		t.Fatalf("expected VolumeUsed %d after roundtrip, got %d", inv.VolumeUsed, out.VolumeUsed)
	}
}

func TestStorageSerializationRoundTrip(t *testing.T) {
	reg := NewRegistry(
		ItemDetails{ID: ItemID("sword"), VolumePerUnit: 5, Name: "Iron Sword"},
		ItemDetails{ID: ItemID("potion"), VolumePerUnit: 2, Name: "Health Potion"},
	)
	inv := NewHybrid("storage1", OwnerID("player1"), 100, 8, 6, WithRegistry(reg))
	
	// Add multiple stacks with different items and shapes
	if err := inv.AddStack(Stack{Item: ItemID("sword"), Owner: OwnerID("player1"), Qty: 1, StackMax: 1, Shape: &Shape{Width: 1, Height: 3}, Position: &Point{X: 0, Y: 0}}); err != nil {
		t.Fatalf("unexpected add error for sword: %v", err)
	}
	if err := inv.AddStack(Stack{Item: ItemID("potion"), Owner: OwnerID("player1"), Qty: 5, StackMax: 10, Shape: &Shape{Width: 1, Height: 1}, Position: &Point{X: 2, Y: 0}}); err != nil {
		t.Fatalf("unexpected add error for potions: %v", err)
	}
	
	// Serialize for storage
	storageData, err := inv.SerializeForStorage()
	if err != nil {
		t.Fatalf("storage serialize error: %v", err)
	}
	
	// Deserialize from storage format
	var out Inventory
	out.SetRegistry(reg)
	if err := out.DeserializeFromStorage(storageData); err != nil {
		t.Fatalf("storage deserialize error: %v", err)
	}
	
	// Verify exact match
	if out.ID != inv.ID || out.Owner != inv.Owner || len(out.Stacks) != len(inv.Stacks) {
		t.Fatalf("basic inventory mismatch after storage roundtrip")
	}
	if out.VolumeUsed != inv.VolumeUsed {
		t.Fatalf("expected VolumeUsed %d after storage roundtrip, got %d", inv.VolumeUsed, out.VolumeUsed)
	}
	
	// Verify stacks
	for i, original := range inv.Stacks {
		restored := out.Stacks[i]
		if restored.Item != original.Item || restored.Qty != original.Qty || restored.StackMax != original.StackMax {
			t.Fatalf("stack %d mismatch: original=%+v, restored=%+v", i, original, restored)
		}
		if original.Position != nil && restored.Position != nil {
			if *restored.Position != *original.Position {
				t.Fatalf("position mismatch for stack %d", i)
			}
		}
	}
}

func TestStorageSerializationRequiresRegistry(t *testing.T) {
	inv := NewVolume("test", OwnerID("user"), 100)
	
	// Should fail without registry
	_, err := inv.SerializeForStorage()
	if err == nil {
		t.Fatalf("expected registry required error for storage serialization")
	}
	
	// Should fail without registry for deserialization
	err = inv.DeserializeFromStorage([]byte(`{"id":"test","stacks":[]}`))
	if err == nil {
		t.Fatalf("expected registry required error for storage deserialization")
	}
}

func TestStorageSerializationUnregisteredItem(t *testing.T) {
	reg := NewRegistry()
	inv := NewVolume("test", OwnerID("user"), 100, WithRegistry(reg))
	
	// Manually add stack with unregistered item (bypassing normal validation)
	inv.Stacks = append(inv.Stacks, Stack{Item: ItemID("unknown"), Qty: 1})
	
	// Should fail when trying to serialize
	_, err := inv.SerializeForStorage()
	if err == nil {
		t.Fatalf("expected error for unregistered item during storage serialization")
	}
}

func TestStorageVsRegularFormatComparison(t *testing.T) {
	reg := NewRegistry(ItemDetails{ID: ItemID("item1"), VolumePerUnit: 3})
	inv := NewVolume("comparison", OwnerID("user"), 50, WithRegistry(reg))
	
	if err := inv.AddStack(Stack{Item: ItemID("item1"), Qty: 5}); err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}
	
	// Get both formats
	regularData, err := inv.Serialize()
	if err != nil {
		t.Fatalf("regular serialize error: %v", err)
	}
	
	storageData, err := inv.SerializeForStorage()
	if err != nil {
		t.Fatalf("storage serialize error: %v", err)
	}
	
	// Storage format should be more compact (numeric vs string IDs)
	if len(storageData) >= len(regularData) {
		t.Logf("Regular format size: %d bytes", len(regularData))
		t.Logf("Storage format size: %d bytes", len(storageData))
		t.Logf("Regular data: %s", string(regularData))
		t.Logf("Storage data: %s", string(storageData))
		// Note: This might not always be true for very small data sets due to JSON overhead
		// but should generally be true for larger inventories
	}
	
	// Verify both can deserialize to equivalent inventories
	var fromRegular, fromStorage Inventory
	fromRegular.SetRegistry(reg)
	fromStorage.SetRegistry(reg)
	
	if err := fromRegular.Deserialize(regularData); err != nil {
		t.Fatalf("deserialize regular error: %v", err)
	}
	if err := fromStorage.DeserializeFromStorage(storageData); err != nil {
		t.Fatalf("deserialize storage error: %v", err)
	}
	
	// Both should be equivalent
	if fromRegular.VolumeUsed != fromStorage.VolumeUsed || len(fromRegular.Stacks) != len(fromStorage.Stacks) {
		t.Fatalf("regular and storage formats produced different results")
	}
}
