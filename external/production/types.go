package production

import (
	"time"

	"github.com/gravitas-015/inventory"
)

// RecipeID uniquely identifies a recipe.
type RecipeID string

// JobID uniquely identifies a production job.
type JobID string

// Recipe defines the transformation rules for production.
type Recipe struct {
	ID       RecipeID            `json:"id"`
	Name     string              `json:"name"`
	Category string              `json:"category,omitempty"`
	Inputs   []ItemRequirement   `json:"inputs"`
	Outputs  []ItemYield         `json:"outputs"`
	Duration time.Duration       `json:"duration"`
	Metadata map[string]any      `json:"metadata,omitempty"`
}

// ItemRequirement specifies an input item for a recipe.
type ItemRequirement struct {
	Item     inventory.ItemID `json:"item"`
	Quantity int              `json:"quantity"`
	Consume  bool             `json:"consume"` // If false, validates presence but doesn't remove (tools)
}

// ItemYield specifies an output item from a recipe.
type ItemYield struct {
	Item        inventory.ItemID `json:"item"`
	Quantity    int              `json:"quantity"`
	Probability float64          `json:"probability"` // 0.0-1.0, default 1.0 (always)
}

// JobState represents the current state of a production job.
type JobState int

const (
	// JobPending is unused - jobs start immediately when created
	JobPending JobState = iota
	// JobRunning indicates the job is in progress (inputs already consumed)
	JobRunning
	// JobComplete indicates the job finished successfully
	JobComplete
	// JobFailed indicates the job failed (e.g., inventory full for outputs)
	JobFailed
	// JobCancelled indicates the job was manually cancelled
	JobCancelled
)

// String returns a human-readable representation of the job state.
func (s JobState) String() string {
	switch s {
	case JobPending:
		return "Pending"
	case JobRunning:
		return "Running"
	case JobComplete:
		return "Complete"
	case JobFailed:
		return "Failed"
	case JobCancelled:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

// Job represents a single production instance in progress.
type Job struct {
	ID                JobID             `json:"id"`
	Recipe            RecipeID          `json:"recipe"`
	Owner             inventory.OwnerID `json:"owner"`
	InventoryID       string            `json:"inventoryId"`
	State             JobState          `json:"state"`
	Progress          float64           `json:"progress"` // 0.0-1.0
	StartTime         time.Time         `json:"startTime"`
	EndTime           time.Time         `json:"endTime"`
	InputSnapshot     []ItemRequirement `json:"inputSnapshot"`     // What was consumed at job start
	Modifiers         Modifiers         `json:"modifiers"`
	EffectiveInputs   []ItemRequirement `json:"effectiveInputs"`   // Inputs after modifiers
	EffectiveOutputs  []ItemYield       `json:"effectiveOutputs"`  // Outputs after modifiers
	EffectiveDuration time.Duration     `json:"effectiveDuration"` // Duration after modifiers
	Repeat            bool              `json:"repeat"`            // If true, job automatically restarts on completion
	CyclesCompleted   int               `json:"cyclesCompleted"`   // Number of cycles completed (for repeating jobs)
	Context           map[string]any    `json:"context,omitempty"`
}

// CalculateProgress returns the current progress (0.0 to 1.0) based on time elapsed.
func (j *Job) CalculateProgress(now time.Time) float64 {
	if j.State != JobRunning {
		if j.State == JobComplete {
			return 1.0
		}
		return 0.0
	}

	if now.Before(j.StartTime) {
		return 0.0
	}

	if now.After(j.EndTime) {
		return 1.0
	}

	elapsed := now.Sub(j.StartTime)
	total := j.EndTime.Sub(j.StartTime)
	if total <= 0 {
		return 1.0
	}

	progress := float64(elapsed) / float64(total)
	if progress < 0.0 {
		return 0.0
	}
	if progress > 1.0 {
		return 1.0
	}
	return progress
}

// Modifiers represents efficiency adjustments applied to production.
type Modifiers struct {
	InputCost   float64  `json:"inputCost"`   // Multiplier for input quantities (0.8 = 20% reduction)
	OutputYield float64  `json:"outputYield"` // Multiplier for output quantities (1.2 = 20% bonus)
	TimeSpeed   float64  `json:"timeSpeed"`   // Multiplier for duration (0.5 = 50% faster, 2.0 = 2x slower)
	Source      string   `json:"source"`      // Description of modifier source
	Tags        []string `json:"tags"`        // Searchable tags for buff management
}

// Default returns identity modifiers (no change).
func (m Modifiers) Default() Modifiers {
	return Modifiers{
		InputCost:   1.0,
		OutputYield: 1.0,
		TimeSpeed:   1.0,
	}
}

// Combine stacks multiple modifiers multiplicatively.
func (m Modifiers) Combine(other Modifiers) Modifiers {
	tags := make([]string, 0, len(m.Tags)+len(other.Tags))
	tags = append(tags, m.Tags...)
	tags = append(tags, other.Tags...)

	source := m.Source
	if source != "" && other.Source != "" {
		source = source + "+" + other.Source
	} else if other.Source != "" {
		source = other.Source
	}

	return Modifiers{
		InputCost:   m.InputCost * other.InputCost,
		OutputYield: m.OutputYield * other.OutputYield,
		TimeSpeed:   m.TimeSpeed * other.TimeSpeed,
		Source:      source,
		Tags:        tags,
	}
}

// ModifierSource provides modifiers for production jobs.
// Implementations can represent building upgrades, character skills, active buffs, etc.
type ModifierSource interface {
	GetModifiers(owner inventory.OwnerID, recipe RecipeID) Modifiers
}

// InventoryProvider abstracts inventory access for the production system.
type InventoryProvider interface {
	// GetInventory retrieves an inventory by ID.
	GetInventory(id string) (*inventory.Inventory, error)

	// ConsumeItems atomically removes items from inventory.
	// Must be atomic - either all items consumed or none.
	// Only consumes items where ItemRequirement.Consume == true.
	// For non-consumed items (tools), validates existence but doesn't remove.
	// Returns error if insufficient quantity of any item.
	ConsumeItems(inv *inventory.Inventory, items []ItemRequirement) error

	// AddItems adds produced items to inventory.
	// May fail if inventory full (volume/grid constraints).
	AddItems(inv *inventory.Inventory, items []ItemYield) error
}
