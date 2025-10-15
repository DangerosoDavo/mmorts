package production

import (
	"math"
)

// applyInputModifiers applies cost modifier to inputs.
// Rounds up to prevent zero-cost exploits and ensures minimum 1 item.
// Only applies to consumed items (tools are not affected).
func applyInputModifiers(inputs []ItemRequirement, modifier float64) []ItemRequirement {
	if len(inputs) == 0 {
		return nil
	}

	result := make([]ItemRequirement, len(inputs))
	for i, req := range inputs {
		result[i] = req
		if req.Consume {
			// Apply modifier and round up
			adjusted := float64(req.Quantity) * modifier
			result[i].Quantity = int(math.Ceil(adjusted))

			// Ensure minimum 1 item
			if result[i].Quantity < 1 {
				result[i].Quantity = 1
			}
		}
	}
	return result
}

// applyOutputModifiers applies yield modifier to outputs.
// Rounds down (conservative) to prevent infinite duplication.
func applyOutputModifiers(outputs []ItemYield, modifier float64) []ItemYield {
	if len(outputs) == 0 {
		return nil
	}

	result := make([]ItemYield, len(outputs))
	for i, yield := range outputs {
		result[i] = yield

		// Apply modifier and round down
		adjusted := float64(yield.Quantity) * modifier
		result[i].Quantity = int(math.Floor(adjusted))

		// Edge case protection
		if result[i].Quantity < 0 {
			result[i].Quantity = 0
		}
	}
	return result
}

// applyDurationModifier applies time speed modifier to duration.
func applyDurationModifier(duration int64, modifier float64) int64 {
	if duration <= 0 {
		return 0
	}

	adjusted := float64(duration) * modifier
	result := int64(math.Round(adjusted))

	// Ensure non-negative
	if result < 0 {
		return 0
	}

	return result
}

// DefaultModifiers returns identity modifiers (no effect).
func DefaultModifiers() Modifiers {
	return Modifiers{
		InputCost:   1.0,
		OutputYield: 1.0,
		TimeSpeed:   1.0,
	}
}
