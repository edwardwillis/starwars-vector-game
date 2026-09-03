package math3d

import "math"

// Quaternion represents a 3D orientation. Unit quaternions are used for all
// rotations; W is the scalar component.
type Quaternion struct {
	W float64
	X float64
	Y float64
	Z float64
}

func IdentityQuaternion() Quaternion {
	return Quaternion{W: 1}
}

func (q Quaternion) Length() float64 {
	return math.Sqrt(q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z)
}

// Normalize returns an equivalent unit quaternion. The zero value represents
// no usable rotation and is normalized to the identity orientation.
func (q Quaternion) Normalize() Quaternion {
	length := q.Length()
	if length == 0 {
		return IdentityQuaternion()
	}
	return Quaternion{
		W: q.W / length,
		X: q.X / length,
		Y: q.Y / length,
		Z: q.Z / length,
	}
}

// Mul composes rotations. q.Mul(other) applies other first and then q.
func (q Quaternion) Mul(other Quaternion) Quaternion {
	return Quaternion{
		W: q.W*other.W - q.X*other.X - q.Y*other.Y - q.Z*other.Z,
		X: q.W*other.X + q.X*other.W + q.Y*other.Z - q.Z*other.Y,
		Y: q.W*other.Y - q.X*other.Z + q.Y*other.W + q.Z*other.X,
		Z: q.W*other.Z + q.X*other.Y - q.Y*other.X + q.Z*other.W,
	}
}

func (q Quaternion) Conjugate() Quaternion {
	return Quaternion{W: q.W, X: -q.X, Y: -q.Y, Z: -q.Z}
}

func QuaternionFromAxisAngle(axis Vec3, radians float64) Quaternion {
	axis = axis.Normalize()
	if axis == (Vec3{}) {
		return IdentityQuaternion()
	}
	sine, cosine := math.Sincos(radians / 2)
	return Quaternion{
		W: cosine,
		X: axis.X * sine,
		Y: axis.Y * sine,
		Z: axis.Z * sine,
	}.Normalize()
}

// QuaternionFromYawPitchRoll constructs local yaw (+Y), pitch (+X), and roll
// (+Z) rotations. For column vectors, roll is applied first, then pitch, then yaw.
func QuaternionFromYawPitchRoll(yaw, pitch, roll float64) Quaternion {
	yawRotation := QuaternionFromAxisAngle(Vec3{Y: 1}, yaw)
	pitchRotation := QuaternionFromAxisAngle(Vec3{X: 1}, pitch)
	rollRotation := QuaternionFromAxisAngle(Vec3{Z: 1}, roll)
	return yawRotation.Mul(pitchRotation).Mul(rollRotation).Normalize()
}

func (q Quaternion) Rotate(v Vec3) Vec3 {
	q = q.Normalize()
	vector := Quaternion{X: v.X, Y: v.Y, Z: v.Z}
	conjugate := Quaternion{W: q.W, X: -q.X, Y: -q.Y, Z: -q.Z}
	result := q.Mul(vector).Mul(conjugate)
	return Vec3{X: result.X, Y: result.Y, Z: result.Z}
}

func (q Quaternion) Matrix() Mat4 {
	q = q.Normalize()
	xx, yy, zz := q.X*q.X, q.Y*q.Y, q.Z*q.Z
	xy, xz, yz := q.X*q.Y, q.X*q.Z, q.Y*q.Z
	wx, wy, wz := q.W*q.X, q.W*q.Y, q.W*q.Z
	return Mat4{
		{1 - 2*(yy+zz), 2 * (xy - wz), 2 * (xz + wy), 0},
		{2 * (xy + wz), 1 - 2*(xx+zz), 2 * (yz - wx), 0},
		{2 * (xz - wy), 2 * (yz + wx), 1 - 2*(xx+yy), 0},
		{0, 0, 0, 1},
	}
}

// Slerp interpolates between orientations along the shortest arc.
func Slerp(first, second Quaternion, amount float64) Quaternion {
	first = first.Normalize()
	second = second.Normalize()
	if amount <= 0 {
		return first
	}
	if amount >= 1 {
		return second
	}
	dot := first.W*second.W + first.X*second.X + first.Y*second.Y + first.Z*second.Z
	if dot < 0 {
		second = Quaternion{W: -second.W, X: -second.X, Y: -second.Y, Z: -second.Z}
		dot = -dot
	}
	if dot > 0.9995 {
		return Quaternion{
			W: first.W + amount*(second.W-first.W),
			X: first.X + amount*(second.X-first.X),
			Y: first.Y + amount*(second.Y-first.Y),
			Z: first.Z + amount*(second.Z-first.Z),
		}.Normalize()
	}
	theta := math.Acos(max(-1, min(1, dot)))
	sine := math.Sin(theta)
	firstWeight := math.Sin((1-amount)*theta) / sine
	secondWeight := math.Sin(amount*theta) / sine
	return Quaternion{
		W: first.W*firstWeight + second.W*secondWeight,
		X: first.X*firstWeight + second.X*secondWeight,
		Y: first.Y*firstWeight + second.Y*secondWeight,
		Z: first.Z*firstWeight + second.Z*secondWeight,
	}.Normalize()
}
