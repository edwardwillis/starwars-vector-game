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
