package path

import (
    "container/heap"
    "math"

    "github.com/gravitas-015/hexcore/hex"
)

// AStar computes a shortest path using the A* algorithm.
// - start, goal: axial coordinates
// - h: admissible heuristic (e.g., hex.DistanceAxial to goal)
// - neighbors: returns adjacent axial coordinates to explore
// - cost: edge cost between two adjacent axial coordinates (must be >=1)
// Returns the path including start and goal, or nil if no path exists.
func AStar(start, goal hex.Axial,
    h func(a hex.Axial) int,
    neighbors func(a hex.Axial) []hex.Axial,
    cost func(a, b hex.Axial) int,
) []hex.Axial {
    if start == goal {
        return []hex.Axial{start}
    }
    // priority queue of nodes by fScore
    open := &nodePQ{}
    heap.Init(open)
    push := func(a hex.Axial, f float64) { heap.Push(open, &pqNode{a: a, f: f}) }

    // maps for gScore and cameFrom
    type key struct{ Q, R int }
    toKey := func(a hex.Axial) key { return key{a.Q, a.R} }

    g := map[key]int{toKey(start): 0}
    came := map[key]key{}
    push(start, float64(h(start)))

    closed := map[key]bool{}
    goalK := toKey(goal)

    for open.Len() > 0 {
        cur := heap.Pop(open).(*pqNode).a
        ck := toKey(cur)
        if closed[ck] { continue }
        closed[ck] = true
        if ck == goalK {
            // reconstruct
            path := []hex.Axial{goal}
            k := goalK
            startK := toKey(start)
            for k != startK {
                k = came[k]
                path = append(path, hex.Axial{Q: k.Q, R: k.R})
            }
            // reverse
            for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
                path[i], path[j] = path[j], path[i]
            }
            return path
        }
        for _, nb := range neighbors(cur) {
            nk := toKey(nb)
            if closed[nk] { continue }
            step := cost(cur, nb)
            if step <= 0 { step = 1 }
            tentative := g[ck] + step
            old, ok := g[nk]
            if !ok || tentative < old {
                g[nk] = tentative
                came[nk] = ck
                f := float64(tentative + h(nb))
                // guard against NaN/Inf
                if math.IsNaN(f) || math.IsInf(f, 0) { f = float64(tentative) }
                push(nb, f)
            }
        }
    }
    return nil
}

// PQ implementation
type pqNode struct {
    a hex.Axial
    f float64
    idx int
}

type nodePQ []*pqNode

func (p nodePQ) Len() int           { return len(p) }
func (p nodePQ) Less(i, j int) bool { return p[i].f < p[j].f }
func (p nodePQ) Swap(i, j int)      { p[i], p[j] = p[j], p[i]; p[i].idx = i; p[j].idx = j }
func (p *nodePQ) Push(x any)        { *p = append(*p, x.(*pqNode)) }
func (p *nodePQ) Pop() any          { old := *p; n := len(old); x := old[n-1]; *p = old[:n-1]; return x }

// Convenience: hex distance heuristic
func HeuristicTo(goal hex.Axial) func(a hex.Axial) int {
    return func(a hex.Axial) int { return hex.DistanceAxial(a, goal) }
}

// Convenience: neighbors limited to disc radius R around center.
func NeighborsWithinDisc(center hex.Axial, R int) func(a hex.Axial) []hex.Axial {
    return func(a hex.Axial) []hex.Axial {
        out := make([]hex.Axial, 0, 6)
        for _, d := range hex.Directions {
            b := a.Add(d)
            if hex.DistanceAxial(center, b) <= R {
                out = append(out, b)
            }
        }
        return out
    }
}

// Convenience: passable-by-set neighbor filter across an arbitrary union of cells.
// passable returns true if axial is traversable; neighbors are the six axial neighbors
// that exist in the union and are passable.
func NeighborsFromUnion(union map[hex.Axial]bool, passable func(a hex.Axial) bool) func(a hex.Axial) []hex.Axial {
    return func(a hex.Axial) []hex.Axial {
        out := make([]hex.Axial, 0, 6)
        for _, d := range hex.Directions {
            b := a.Add(d)
            if union[b] && passable(b) {
                out = append(out, b)
            }
        }
        return out
    }
}
