package hex

// Ring returns the axial coordinates at exact distance k from center c,
// starting from direction 4 (south-east) and proceeding counter-clockwise.
// If k==0, returns [c].
func Ring(c Axial, k int) []Axial {
    if k == 0 {
        return []Axial{c}
    }
    res := make([]Axial, 0, 6*k)
    // start position: c + dir[4]*k (arbitrary but consistent)
    cur := c.Add(Directions[4].Mul(k))
    // traverse 6 sides
    for side := 0; side < 6; side++ {
        for step := 0; step < k; step++ {
            res = append(res, cur)
            cur = cur.Add(Directions[side])
        }
    }
    return res
}

// Disk returns all axial coordinates at distance <= r from center c.
func Disk(c Axial, r int) []Axial {
    size := 1 + 3*r*(r+1)
    res := make([]Axial, 0, size)
    for q := -r; q <= r; q++ {
        for r2 := max(-r, -q-r); r2 <= min(r, -q+r); r2++ {
            res = append(res, c.Add(Axial{q, r2}))
        }
    }
    return res
}

// Edge returns the R axial coordinates on the ring at distance R belonging
// to the specified side (0..5). The order is along the side.
func Edge(c Axial, R int, side int) []Axial {
    // Define side s as the set of ring cells whose outward neighbor
    // in direction s would move farther from the center (distance R+1).
    // This is robust regardless of ring starting offset.
    if R <= 0 { return []Axial{c} }
    r := Ring(c, R)
    seg := make([]Axial, 0, R)
    // Gather indices on this side
    idxs := make([]int, 0, R)
    for i, a := range r {
        out := a.Add(Directions[side])
        if DistanceAxial(c, out) == R+1 { // a is on side s
            idxs = append(idxs, i)
        }
    }
    if len(idxs) != R {
        // Fallback: keep previous segmentation to avoid panic
        start := (side * R) % len(r)
        for i := 0; i < R; i++ { seg = append(seg, r[(start+i)%len(r)]) }
        return seg
    }
    // Ensure indices are in ring order and contiguous (wrap around allowed)
    // Find the smallest gap to choose a continuous segment start
    startIdx := 0
    minGap := 1<<31 - 1
    L := len(r)
    for i := 0; i < len(idxs); i++ {
        j := (i + 1) % len(idxs)
        gap := (idxs[j] - idxs[i] + L) % L
        if gap < minGap { minGap = gap; startIdx = j }
    }
    for k := 0; k < R; k++ {
        i := (startIdx + k) % len(idxs)
        seg = append(seg, r[idxs[i]])
    }
    return seg
}

func min(a, b int) int { if a < b { return a }; return b }
func max(a, b int) int { if a > b { return a }; return b }
