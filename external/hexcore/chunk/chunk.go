package chunk

import (
    "github.com/gravitas-015/hexcore"
    "github.com/gravitas-015/hexcore/hex"
    "github.com/gravitas-015/mapgen/generator"
)

type EdgeDirection int // 0..5

// EdgeMask uses one bit per cell along an edge at ring R.
type EdgeMask uint64 // enough to hold up to R<=63 safely

type HexChunk struct {
    Coord   hex.Axial
    Radius  int
    Cells   map[hex.Axial]hexcore.HexState
    EdgeSig map[EdgeDirection]EdgeMask
    Seed    int64
    Ruleset string
}

// BuildChunk builds a chunk at center using CA and computes edge signatures.
func BuildChunk(center hex.Axial, radius int, seed int64, params generator.Params) HexChunk {
    cells := generator.GenerateChunkCells(center, seed, params)
    sig := make(map[EdgeDirection]EdgeMask, 6)
    for s := 0; s < 6; s++ {
        var m EdgeMask
        edge := hex.Edge(center, radius, s)
        for i, a := range edge {
            if cells[a] == hexcore.Space {
                m |= 1 << i
            }
        }
        sig[EdgeDirection(s)] = m
    }
    return HexChunk{Coord: center, Radius: radius, Cells: cells, EdgeSig: sig, Seed: seed, Ruleset: "default"}
}

// NeighborChunkCenter returns the neighbor chunk center at side s with offset 2R.
func NeighborChunkCenter(center hex.Axial, radius int, side int) hex.Axial {
    // Chunk grid uses a "flat-top" orientation when the underlying hexes are pointy-top.
    // Use diagonal step: V = (dir[side] + dir[side-1]) * R ⇒ center distance = 2R.
    // This aligns chunk sides without triangular gaps and matches the user's intent
    // of "18 hexes in each axial direction" for R=9.
    d := hex.Directions[side]
    dprev := hex.Directions[(side+5)%6]
    v := hex.Axial{Q: d.Q + dprev.Q, R: d.R + dprev.R}
    return center.Add(v.Mul(radius))
}

// Pocket represents the union of 7 chunks (center + 6 neighbors) with unified axial cells.
type Pocket struct {
    Center hex.Axial
    Radius int
    Cells  map[hex.Axial]hexcore.HexState
}

// BuildPocket constructs a HexPocket from center chunk and its six neighbors using the same params/seed.
func BuildPocket(center hex.Axial, radius int, seed int64, params generator.Params) Pocket {
    union := make(map[hex.Axial]hexcore.HexState, 7*(1+3*radius*(radius+1)))
    // center
    c := BuildChunk(center, radius, seed, params)
    for a, st := range c.Cells { union[a] = st }
    // 6 neighbors
    for s := 0; s < 6; s++ {
        nCenter := NeighborChunkCenter(center, radius, s)
        nc := BuildChunk(nCenter, radius, seed, params)
        for a, st := range nc.Cells { union[a] = st }
    }
    return Pocket{Center: center, Radius: radius, Cells: union}
}
