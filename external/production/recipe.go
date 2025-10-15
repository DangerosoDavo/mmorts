package production

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gravitas-015/inventory"
)

// RecipeRegistry stores recipes with efficient lookup and thread-safe access.
type RecipeRegistry struct {
	mu         sync.RWMutex
	recipes    map[RecipeID]*Recipe
	byCategory map[string][]RecipeID
	byOutput   map[inventory.ItemID][]RecipeID
}

// NewRecipeRegistry creates an empty recipe registry.
func NewRecipeRegistry() *RecipeRegistry {
	return &RecipeRegistry{
		recipes:    make(map[RecipeID]*Recipe),
		byCategory: make(map[string][]RecipeID),
		byOutput:   make(map[inventory.ItemID][]RecipeID),
	}
}

// Register adds or updates a recipe in the registry.
// Returns an error if the recipe is invalid.
func (r *RecipeRegistry) Register(recipe *Recipe) error {
	if recipe == nil {
		return errors.New("recipe cannot be nil")
	}
	if recipe.ID == "" {
		return errors.New("recipe ID cannot be empty")
	}
	// Note: Outputs can be empty for "effect-only" recipes (e.g., building construction)
	// where external systems handle the completion via events

	// Validate inputs
	for i, input := range recipe.Inputs {
		if input.Item == "" {
			return fmt.Errorf("input %d: item ID cannot be empty", i)
		}
		if input.Quantity <= 0 {
			return fmt.Errorf("input %d: quantity must be positive", i)
		}
	}

	// Validate outputs
	for i, output := range recipe.Outputs {
		if output.Item == "" {
			return fmt.Errorf("output %d: item ID cannot be empty", i)
		}
		if output.Quantity < 0 {
			return fmt.Errorf("output %d: quantity cannot be negative", i)
		}
		if output.Probability < 0.0 || output.Probability > 1.0 {
			return fmt.Errorf("output %d: probability must be between 0.0 and 1.0", i)
		}
		// Default probability to 1.0 if not set
		if output.Probability == 0.0 {
			recipe.Outputs[i].Probability = 1.0
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove old indices if updating existing recipe
	if existing, exists := r.recipes[recipe.ID]; exists {
		r.removeIndices(existing)
	}

	// Store recipe
	r.recipes[recipe.ID] = recipe

	// Index by category
	if recipe.Category != "" {
		r.byCategory[recipe.Category] = append(r.byCategory[recipe.Category], recipe.ID)
	}

	// Index by output items
	for _, output := range recipe.Outputs {
		r.byOutput[output.Item] = append(r.byOutput[output.Item], recipe.ID)
	}

	return nil
}

// removeIndices removes a recipe from secondary indices (caller must hold lock).
func (r *RecipeRegistry) removeIndices(recipe *Recipe) {
	// Remove from category index
	if recipe.Category != "" {
		if ids, exists := r.byCategory[recipe.Category]; exists {
			r.byCategory[recipe.Category] = removeRecipeID(ids, recipe.ID)
			if len(r.byCategory[recipe.Category]) == 0 {
				delete(r.byCategory, recipe.Category)
			}
		}
	}

	// Remove from output index
	for _, output := range recipe.Outputs {
		if ids, exists := r.byOutput[output.Item]; exists {
			r.byOutput[output.Item] = removeRecipeID(ids, recipe.ID)
			if len(r.byOutput[output.Item]) == 0 {
				delete(r.byOutput, output.Item)
			}
		}
	}
}

// removeRecipeID removes a recipe ID from a slice.
func removeRecipeID(ids []RecipeID, target RecipeID) []RecipeID {
	result := make([]RecipeID, 0, len(ids))
	for _, id := range ids {
		if id != target {
			result = append(result, id)
		}
	}
	return result
}

// Lookup retrieves a recipe by ID. Returns nil if not found.
func (r *RecipeRegistry) Lookup(id RecipeID) *Recipe {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.recipes[id]
}

// GetByCategory returns all recipe IDs in a category.
func (r *RecipeRegistry) GetByCategory(category string) []RecipeID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byCategory[category]
	if ids == nil {
		return nil
	}

	// Return a copy to prevent external modification
	result := make([]RecipeID, len(ids))
	copy(result, ids)
	return result
}

// GetByOutput returns all recipe IDs that produce a given item.
func (r *RecipeRegistry) GetByOutput(item inventory.ItemID) []RecipeID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byOutput[item]
	if ids == nil {
		return nil
	}

	// Return a copy to prevent external modification
	result := make([]RecipeID, len(ids))
	copy(result, ids)
	return result
}

// GetAll returns all recipes in the registry.
func (r *RecipeRegistry) GetAll() []*Recipe {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Recipe, 0, len(r.recipes))
	for _, recipe := range r.recipes {
		result = append(result, recipe)
	}
	return result
}

// Count returns the number of recipes in the registry.
func (r *RecipeRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.recipes)
}

// Remove deletes a recipe from the registry. Returns true if recipe existed.
func (r *RecipeRegistry) Remove(id RecipeID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	recipe, exists := r.recipes[id]
	if !exists {
		return false
	}

	r.removeIndices(recipe)
	delete(r.recipes, id)
	return true
}

// Clear removes all recipes from the registry.
func (r *RecipeRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.recipes = make(map[RecipeID]*Recipe)
	r.byCategory = make(map[string][]RecipeID)
	r.byOutput = make(map[inventory.ItemID][]RecipeID)
}
