package chunk

import (
    "math/rand"

    "github.com/gravitas-015/hexcore"
    "github.com/gravitas-015/hexcore/hex"
)

// Spur describes an outbound marker from a boundary cell in a given axial direction.
type Spur struct {
    Origin hex.Axial
    Dir    int // 0..5
}

// Link connects two axial cells inside the pocket.
type Link struct {
    From hex.Axial
    To   hex.Axial
}

// ComputeBoundarySpurs finds edge Space cells (ring R) on open sides of each chunk center
// that have fewer than 6 existing neighbors in the current union. For each such cell, it
// selects an outward direction that does not point towards an existing cell and returns
// a spur descriptor.
//
// Inputs:
// - plan: list of chunk centers generated
// - R: chunk radius
// - unionState: map of axial -> state (0=Dead,1=Space,2=Overlay/Yellow) for all generated cells
//
// Output:
// - list of spurs; duplicates by origin are suppressed.
func ComputeBoundarySpurs(plan []hex.Axial, R int, unionState map[hex.Axial]int) []Spur {
    // Build plan set for quick membership
    planSet := make(map[hex.Axial]bool, len(plan))
    for _, c := range plan { planSet[c] = true }
    // Presence map regardless of state
    present := make(map[hex.Axial]bool, len(unionState))
    for a := range unionState { present[a] = true }

    // helper: count present neighbors (any state)
    neighborPresentCount := func(a hex.Axial) int {
        n := 0
        for _, d := range hex.Directions {
            if present[a.Add(d)] { n++ }
        }
        return n
    }

    dedup := make(map[hex.Axial]bool)
    out := make([]Spur, 0, 64)

    for _, c := range plan {
        for s := 0; s < 6; s++ {
            // open side: no neighbor chunk center on this side
            nb := NeighborChunkCenter(c, R, s)
            if planSet[nb] { continue }
            // examine edge cells on this side
            edge := hex.Edge(c, R, s)
            for _, a := range edge {
                st, ok := unionState[a]
                if !ok || st != int(hexcore.Space) { continue }
                if neighborPresentCount(a) >= 6 { continue }
                if dedup[a] { continue }
                // Prefer outward direction s if absent; else find any absent dir
                chosen := -1
                if !present[a.Add(hex.Directions[s])] {
                    chosen = s
                } else {
                    for d := 0; d < 6; d++ {
                        if !present[a.Add(hex.Directions[d])] {
                            chosen = d
                            break
                        }
                    }
                }
                if chosen >= 0 {
                    out = append(out, Spur{Origin: a, Dir: chosen})
                    dedup[a] = true
                }
            }
        }
    }

    return out
}

// ComputeInternalLinks casts rays from each spur origin along its direction until
// it hits any existing cell in the current pocket union. If the hit is a green cell,
// or a grey cell that is adjacent to at least one green cell, a link is produced from
// the spur origin to the hit cell.
// EvaluateInternalLinks returns accepted links and spurs that collided with the pocket
// but were rejected (i.e., hit a cell that didn't satisfy the criteria to become a link).
func EvaluateInternalLinks(spurs []Spur, R int, unionState map[hex.Axial]int) (links []Link, rejected []hex.Axial) {
    present := make(map[hex.Axial]bool, len(unionState))
    for a := range unionState { present[a] = true }
    // conservative upper bound for ray length across a pocket
    maxSteps := 8 * R
    links = make([]Link, 0, len(spurs))
    rejected = make([]hex.Axial, 0)
    for _, sp := range spurs {
        a := sp.Origin
        d := hex.Directions[sp.Dir]
        cur := a
        hit := hex.Axial{}
        ok := false
        for step := 0; step < maxSteps; step++ {
            cur = cur.Add(d)
            if present[cur] {
                hit = cur
                ok = true
                break
            }
        }
        if !ok { continue }
        st := unionState[hit]
        isGreen := st == int(hexcore.Space)
        isGreyAdjGreen := st == int(hexcore.Dead) && hasAdjacentGreen(hit, unionState)
        if isGreen || isGreyAdjGreen {
            links = append(links, Link{From: a, To: hit})
        } else {
            rejected = append(rejected, a)
        }
    }
    return links, rejected
}

func hasAdjacentGreen(a hex.Axial, unionState map[hex.Axial]int) bool {
    for _, d := range hex.Directions {
        if unionState[a.Add(d)] == int(hexcore.Space) {
            return true
        }
    }
    return false
}

// ChunkOf returns the chunk center from plan that contains cell a within radius R.
func ChunkOf(a hex.Axial, plan []hex.Axial, R int) (hex.Axial, bool) {
    for _, c := range plan {
        if hex.DistanceAxial(c, a) <= R { return c, true }
    }
    return hex.Axial{}, false
}

// SelectLinksOnePerChunk returns a subset of links such that each chunk in `plan`
// participates in at most one link (as a From- or To- endpoint). Links are shuffled
// and greedily selected; the result is deterministic given rng.
func SelectLinksOnePerChunk(cand []Link, plan []hex.Axial, R int, rng *rand.Rand) []Link {
    if len(cand) <= 1 { return cand }
    idxs := make([]int, len(cand))
    for i := range idxs { idxs[i] = i }
    rng.Shuffle(len(idxs), func(i, j int) { idxs[i], idxs[j] = idxs[j], idxs[i] })
    used := make(map[hex.Axial]bool)
    out := make([]Link, 0, len(cand))
    for _, i := range idxs {
        ln := cand[i]
        oc, ok1 := ChunkOf(ln.From, plan, R)
        tc, ok2 := ChunkOf(ln.To, plan, R)
        if !ok1 || !ok2 { continue }
        if used[oc] || used[tc] { continue }
        used[oc] = true
        used[tc] = true
        out = append(out, ln)
    }
    return out
}
