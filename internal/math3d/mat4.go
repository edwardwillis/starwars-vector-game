package math3d

import "math"

// Mat4 is a row-major 4x4 matrix that multiplies column vectors.
type Mat4 [4][4]float64

func Identity() Mat4 {
	return Mat4{
		{1, 0, 0, 0},
		{0, 1, 0, 0},
		{0, 0, 1, 0},
		{0, 0, 0, 1},
	}
}

func Translation(x, y, z float64) Mat4 {
	result := Identity()
	result[0][3] = x
	result[1][3] = y
	result[2][3] = z
	return result
}

func Scaling(x, y, z float64) Mat4 {
	return Mat4{
		{x, 0, 0, 0},
		{0, y, 0, 0},
		{0, 0, z, 0},
		{0, 0, 0, 1},
	}
}

func RotationX(radians float64) Mat4 {
	sine, cosine := math.Sincos(radians)
	return Mat4{
		{1, 0, 0, 0},
		{0, cosine, -sine, 0},
		{0, sine, cosine, 0},
		{0, 0, 0, 1},
	}
}

func RotationY(radians float64) Mat4 {
	sine, cosine := math.Sincos(radians)
	return Mat4{
		{cosine, 0, sine, 0},
		{0, 1, 0, 0},
		{-sine, 0, cosine, 0},
		{0, 0, 0, 1},
	}
}

func RotationZ(radians float64) Mat4 {
	sine, cosine := math.Sincos(radians)
	return Mat4{
		{cosine, -sine, 0, 0},
		{sine, cosine, 0, 0},
		{0, 0, 1, 0},
		{0, 0, 0, 1},
	}
}

// Perspective constructs a right-handed perspective matrix for a camera that
// looks down negative Z. After perspective divide, visible depth maps to [-1,1].
func Perspective(verticalFOV, aspect, near, far float64) Mat4 {
	if verticalFOV <= 0 || verticalFOV >= math.Pi {
		panic("math3d: vertical field of view must be between 0 and pi")
	}
	if aspect <= 0 {
		panic("math3d: aspect ratio must be positive")
	}
	if near <= 0 || far <= near {
		panic("math3d: clipping planes must satisfy 0 < near < far")
	}

	focalLength := 1 / math.Tan(verticalFOV/2)
	depthRange := near - far
	return Mat4{
		{focalLength / aspect, 0, 0, 0},
		{0, focalLength, 0, 0},
		{0, 0, (far + near) / depthRange, (2 * far * near) / depthRange},
		{0, 0, -1, 0},
	}
}

// Mul composes two transforms. For column vectors, a.Mul(b) applies b first
// and then a.
func (a Mat4) Mul(b Mat4) Mat4 {
	var result Mat4
	for row := range 4 {
		for column := range 4 {
			for index := range 4 {
				result[row][column] += a[row][index] * b[index][column]
			}
		}
	}
	return result
}

func (m Mat4) Transform(v Vec4) Vec4 {
	values := [4]float64{v.X, v.Y, v.Z, v.W}
	var result [4]float64
	for row := range 4 {
		for column := range 4 {
			result[row] += m[row][column] * values[column]
		}
	}
	return Vec4{X: result[0], Y: result[1], Z: result[2], W: result[3]}
}

func (m Mat4) TransformPoint(point Vec3) Vec3 {
	return m.Transform(Vec4{X: point.X, Y: point.Y, Z: point.Z, W: 1}).PerspectiveDivide()
}

func (m Mat4) TransformDirection(direction Vec3) Vec3 {
	result := m.Transform(Vec4{X: direction.X, Y: direction.Y, Z: direction.Z})
	return Vec3{X: result.X, Y: result.Y, Z: result.Z}
}

// TransformNormal applies the inverse transpose of the affine transform's
// linear 3x3 component. Unlike TransformDirection, this remains perpendicular
// to surfaces under the non-uniform instance scaling used by shared geometry.
func (m Mat4) TransformNormal(normal Vec3) Vec3 {
	a, b, c := m[0][0], m[0][1], m[0][2]
	d, e, f := m[1][0], m[1][1], m[1][2]
	g, h, i := m[2][0], m[2][1], m[2][2]
	c00, c01, c02 := e*i-f*h, f*g-d*i, d*h-e*g
	c10, c11, c12 := c*h-b*i, a*i-c*g, b*g-a*h
	c20, c21, c22 := b*f-c*e, c*d-a*f, a*e-b*d
	determinant := a*c00 + b*c01 + c*c02
	if math.Abs(determinant) <= 1e-12 {
		return m.TransformDirection(normal)
	}
	return Vec3{
		X: (c00*normal.X + c01*normal.Y + c02*normal.Z) / determinant,
		Y: (c10*normal.X + c11*normal.Y + c12*normal.Z) / determinant,
		Z: (c20*normal.X + c21*normal.Y + c22*normal.Z) / determinant,
	}
}
