package math3d

import (
	"math"
	"testing"
)

func TestQuaternionRotatesVector(t *testing.T) {
	yaw := QuaternionFromAxisAngle(Vec3{Y: 1}, math.Pi/2)
	assertVec3(t, yaw.Rotate(Vec3{Z: 1}), Vec3{X: 1})
}

func TestQuaternionMatrixMatchesRotationMatrix(t *testing.T) {
	rotation := QuaternionFromAxisAngle(Vec3{X: 1}, math.Pi/3)
	point := Vec3{X: 1, Y: 2, Z: 3}
	assertVec3(t, rotation.Matrix().TransformPoint(point), RotationX(math.Pi/3).TransformPoint(point))
}

func TestQuaternionComposition(t *testing.T) {
	rotation := QuaternionFromYawPitchRoll(math.Pi/2, 0, math.Pi/2)
	assertVec3(t, rotation.Rotate(Vec3{X: 1}), Vec3{Y: 1})
	assertClose(t, rotation.Length(), 1)
}

func TestZeroQuaternionNormalizesToIdentity(t *testing.T) {
	assertVec3(t, (Quaternion{}).Normalize().Rotate(Vec3{X: 1}), Vec3{X: 1})
}

func TestSlerpUsesShortestArc(t *testing.T) {
	first := IdentityQuaternion()
	second := QuaternionFromAxisAngle(Vec3{Y: 1}, 3*math.Pi/2)
	middle := Slerp(first, second, 0.5)
	if math.Abs(middle.Length()-1) > 1e-9 {
		t.Fatalf("slerp length=%v", middle.Length())
	}
	if middle.Rotate(Vec3{Z: 1}).X > -0.5 {
		t.Fatalf("slerp did not take shortest arc: %+v", middle.Rotate(Vec3{Z: 1}))
	}
}
