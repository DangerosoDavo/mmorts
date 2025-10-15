package gamemap

import (
	"github.com/gravitas-015/hexcore/hex"
)

// HexChunk represents a chunk of hexes in the game world
// Each chunk is a hex-shaped region with a configurable radius
type HexChunk struct {
	ChunkPos  hex.Axial          // Position in chunk grid
	Hexes     map[hex.Axial]*Hex // Local hex positions within chunk
	Generated bool
	Radius    int // Hex radius of this chunk (default 9)
}

// Hex represents a single hex cell in the world
type Hex struct {
	WorldPos hex.Axial // World position
	Terrain  string    // Terrain type: "plains", "forest", etc.
}

// NewHexChunk creates a new hex chunk at the specified position
func NewHexChunk(chunkPos hex.Axial) *HexChunk {
	const defaultRadius = 9 // From architecture: radius 9 = 169 hexes

	chunk := &HexChunk{
		ChunkPos:  chunkPos,
		Hexes:     make(map[hex.Axial]*Hex),
		Generated: false,
		Radius:    defaultRadius,
	}

	// Generate blank hexes for this chunk
	chunk.generateHexes()

	return chunk
}

// generateHexes creates all hexes within this chunk
// For Phase 1, all hexes are blank "plains"
func (c *HexChunk) generateHexes() {
	// Generate hexes in a hex radius pattern
	for q := -c.Radius; q <= c.Radius; q++ {
		r1 := maxInt(-c.Radius, -q-c.Radius)
		r2 := minInt(c.Radius, -q+c.Radius)

		for r := r1; r <= r2; r++ {
			localPos := hex.Axial{Q: q, R: r}

			// Calculate world position
			// TODO: Proper world coordinate calculation based on chunk position
			worldPos := hex.Axial{
				Q: c.ChunkPos.Q*c.Radius + localPos.Q,
				R: c.ChunkPos.R*c.Radius + localPos.R,
			}

			c.Hexes[localPos] = &Hex{
				WorldPos: worldPos,
				Terrain:  "plains", // All blank for Phase 1
			}
		}
	}

	c.Generated = true
}

// GetHex retrieves a hex at the local position within this chunk
func (c *HexChunk) GetHex(localPos hex.Axial) (*Hex, bool) {
	hex, exists := c.Hexes[localPos]
	return hex, exists
}

// HexCount returns the number of hexes in this chunk
func (c *HexChunk) HexCount() int {
	return len(c.Hexes)
}

// Utility functions
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
