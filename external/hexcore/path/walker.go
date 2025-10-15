package path

import (
    "math/rand"

    "github.com/gravitas-015/hexcore"
    "github.com/gravitas-015/hexcore/hex"
)

// SelectPortalRandom picks a random cell along the edge (ring R) for the given side.
// Returns the axial coordinate and its index along that side.
func SelectPortalRandom(center hex.Axial, R int, side int, rng *rand.Rand) (hex.Axial, int) {
    edge := hex.Edge(center, R, side)
    idx := 0
    if len(edge) > 1 {
        idx = rng.Intn(len(edge))
    }
    return edge[idx], idx
}

// SelectPortalDeterministic picks a stable cell along the edge using a seed.
// It hashes each axial coordinate with the seed and selects the minimum hash;
// because the underlying set of coordinates is identical from both sides of a
// shared boundary, adjacent chunks will agree on the same portal cell.
func SelectPortalDeterministic(center hex.Axial, R int, side int, seed int64) (hex.Axial, int) {
    edge := hex.Edge(center, R, side)
    if len(edge) == 0 {
        return center, 0
    }
    bestIdx := 0
    best := uint64(^uint64(0))
    for i, a := range edge {
        h := hashCoordWithSeed(seed, a)
        if h < best {
            best = h
            bestIdx = i
        }
    }
    return edge[bestIdx], bestIdx
}

func hashCoordWithSeed(seed int64, a hex.Axial) uint64 {
    // splitmix-like integer hashing mixed with axial coords
    x := uint64(seed)
    x ^= uint64(uint32(a.Q)) * 0x9E3779B97F4A7C15
    x ^= uint64(uint32(a.R)) * 0xBF58476D1CE4E5B9
    x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
    x = (x ^ (x >> 27)) * 0x94D049BB133111EB
    x ^= x >> 31
    return x
}

// neighborsWithinDisc returns axial neighbors within the disc of radius R around center.
func neighborsWithinDisc(a, center hex.Axial, R int) []hex.Axial {
    ns := make([]hex.Axial, 0, 6)
    for _, d := range hex.Directions {
        b := a.Add(d)
        if hex.DistanceAxial(center, b) <= R {
            ns = append(ns, b)
        }
    }
    return ns
}

// bfsPath finds a shortest path within the disc from start to goal, randomizing neighbor order.
func BFSPath(center hex.Axial, R int, start, goal hex.Axial, rng *rand.Rand) []hex.Axial {
    if start == goal {
        return []hex.Axial{start}
    }
    // shuffle direction indices once to introduce randomness in tie breaks
    order := []int{0, 1, 2, 3, 4, 5}
    rng.Shuffle(6, func(i, j int) { order[i], order[j] = order[j], order[i] })

    // classic BFS
    type key struct{ Q, R int }
    toKey := func(a hex.Axial) key { return key{a.Q, a.R} }

    prev := make(map[key]key)
    visited := make(map[key]bool)
    q := []hex.Axial{start}
    visited[toKey(start)] = true
    found := false
    for len(q) > 0 && !found {
        cur := q[0]
        q = q[1:]
        // visit neighbors in randomized order
        for _, idx := range order {
            d := hex.Directions[idx]
            nxt := cur.Add(d)
            if hex.DistanceAxial(center, nxt) > R { continue }
            k := toKey(nxt)
            if visited[k] { continue }
            visited[k] = true
            prev[k] = toKey(cur)
            if nxt == goal {
                found = true
                break
            }
            q = append(q, nxt)
        }
    }
    if !found {
        return nil
    }
    // reconstruct path
    path := []hex.Axial{goal}
    cur := toKey(goal)
    startK := toKey(start)
    for cur != startK {
        cur = prev[cur]
        path = append(path, hex.Axial{Q: cur.Q, R: cur.R})
    }
    // reverse
    for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
        path[i], path[j] = path[j], path[i]
    }
    return path
}

// CarvePath sets the cells along the path to Space and marks them locked.
func CarvePath(cells map[hex.Axial]hexcore.HexState, path []hex.Axial, locked map[hex.Axial]bool) {
    for _, a := range path {
        cells[a] = hexcore.Space
        locked[a] = true
    }
}
