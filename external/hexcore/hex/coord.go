package hex

import "math"

// Axial represents axial coordinates (q, r) for pointy-top orientation.
type Axial struct {
    Q int
    R int
}

// Cube represents cube coordinates (x, y, z) with x+y+z=0.
type Cube struct {
    X int
    Y int
    Z int
}

// Directions for axial neighbors in pointy-top orientation.
var Directions = []Axial{
    {+1, 0}, {+1, -1}, {0, -1}, {-1, 0}, {-1, +1}, {0, +1},
}

// Add returns a+b in axial space.
func (a Axial) Add(b Axial) Axial { return Axial{a.Q + b.Q, a.R + b.R} }

// Mul scales an axial vector by k.
func (a Axial) Mul(k int) Axial { return Axial{a.Q * k, a.R * k} }

// ToCube converts axial to cube.
func (a Axial) ToCube() Cube {
    x := a.Q
    z := a.R
    y := -x - z
    return Cube{X: x, Y: y, Z: z}
}

// ToAxial converts cube to axial.
func (c Cube) ToAxial() Axial { return Axial{Q: c.X, R: c.Z} }

// DistanceAxial returns hex distance between two axial coords.
func DistanceAxial(a, b Axial) int {
    return DistanceCube(a.ToCube(), b.ToCube())
}

// DistanceCube returns hex distance between two cube coords.
func DistanceCube(a, b Cube) int {
    dx := int(math.Abs(float64(a.X - b.X)))
    dy := int(math.Abs(float64(a.Y - b.Y)))
    dz := int(math.Abs(float64(a.Z - b.Z)))
    if dx > dy && dx > dz {
        return dx
    }
    if dy > dz {
        return dy
    }
    return dz
}

// AxialToPixel converts axial to pixel coordinates for pointy-top layout.
// size is the hex radius (corner to center) in pixels.
func AxialToPixel(a Axial, size float64) (x, y float64) {
    // pointy-top: x = size*sqrt(3)*(q + r/2); y = size*3/2*r
    x = size * math.Sqrt(3) * (float64(a.Q) + float64(a.R)/2.0)
    y = size * 1.5 * float64(a.R)
    return
}
