// Package math3d provides the vector and matrix operations used by the
// wireframe rendering pipeline.
package math3d

import "math"

// Vec3 is a three-dimensional vector.
type Vec3 struct {
	X float64
	Y float64
	Z float64
}

func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{X: v.X + other.X, Y: v.Y + other.Y, Z: v.Z + other.Z}
}

func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{X: v.X - other.X, Y: v.Y - other.Y, Z: v.Z - other.Z}
}

func (v Vec3) Scale(factor float64) Vec3 {
	return Vec3{X: v.X * factor, Y: v.Y * factor, Z: v.Z * factor}
}

func (v Vec3) Dot(other Vec3) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

func (v Vec3) Cross(other Vec3) Vec3 {
	return Vec3{
		X: v.Y*other.Z - v.Z*other.Y,
		Y: v.Z*other.X - v.X*other.Z,
		Z: v.X*other.Y - v.Y*other.X,
	}
}

func (v Vec3) Length() float64 {
	return math.Sqrt(v.Dot(v))
}

// Normalize returns a unit vector in the same direction. A zero vector cannot
// define a direction, so it is returned unchanged.
func (v Vec3) Normalize() Vec3 {
	length := v.Length()
	if length == 0 {
		return Vec3{}
	}
	return v.Scale(1 / length)
}
