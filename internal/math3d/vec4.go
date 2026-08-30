package math3d

// Vec4 is a homogeneous four-dimensional vector. Points conventionally use
// W=1, while directions use W=0.
type Vec4 struct {
	X float64
	Y float64
	Z float64
	W float64
}

func (v Vec4) PerspectiveDivide() Vec3 {
	if v.W == 0 {
		return Vec3{X: v.X, Y: v.Y, Z: v.Z}
	}
	return Vec3{X: v.X / v.W, Y: v.Y / v.W, Z: v.Z / v.W}
}
