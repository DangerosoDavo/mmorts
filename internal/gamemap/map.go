package gamemap

import (
	"fmt"
	"log"

	"github.com/gravitas-015/hexcore/hex"
)

// GameMap represents the game world map
type GameMap struct {
	Chunks      map[hex.Axial]*HexChunk
	ChunkRadius int // Number of chunks from origin
}

// New creates a new game map with the specified chunk radius
func New(chunkRadius int) (*GameMap, error) {
	log.Printf("Generating game map with chunk radius %d", chunkRadius)

	gm := &GameMap{
		Chunks:      make(map[hex.Axial]*HexChunk),
		ChunkRadius: chunkRadius,
	}

	// Generate initial chunks in a hex pattern around origin
	if err := gm.generateChunks(); err != nil {
		return nil, err
	}

	log.Printf("Game map generated with %d chunks", len(gm.Chunks))
	return gm, nil
}

// generateChunks creates hex chunks in a radius around the origin
func (gm *GameMap) generateChunks() error {
	// Generate chunks in hex radius pattern
	for q := -gm.ChunkRadius; q <= gm.ChunkRadius; q++ {
		r1 := maxInt(-gm.ChunkRadius, -q-gm.ChunkRadius)
		r2 := minInt(gm.ChunkRadius, -q+gm.ChunkRadius)

		for r := r1; r <= r2; r++ {
			chunkPos := hex.Axial{Q: q, R: r}
			chunk := NewHexChunk(chunkPos)
			gm.Chunks[chunkPos] = chunk
		}
	}

	log.Printf("Generated %d chunks around origin", len(gm.Chunks))
	return nil
}

// GetChunk retrieves a chunk at the specified position
func (gm *GameMap) GetChunk(pos hex.Axial) (*HexChunk, bool) {
	chunk, exists := gm.Chunks[pos]
	return chunk, exists
}

// GetHex retrieves a hex at the specified world position
// This converts world coordinates to chunk + local coordinates
func (gm *GameMap) GetHex(worldPos hex.Axial) (*Hex, error) {
	// TODO: Implement world -> chunk coordinate conversion
	// For now, just a placeholder
	return nil, fmt.Errorf("not implemented")
}
