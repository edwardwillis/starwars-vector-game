package math3d

import (
	"math"
	"testing"
)

func TestVec3Arithmetic(t *testing.T) {
	a := Vec3{X: 1, Y: 2, Z: 3}
	b := Vec3{X: 4, Y: -2, Z: 1}

	assertVec3(t, a.Add(b), Vec3{X: 5, Y: 0, Z: 4})
	assertVec3(t, a.Sub(b), Vec3{X: -3, Y: 4, Z: 2})
	assertVec3(t, a.Scale(2), Vec3{X: 2, Y: 4, Z: 6})
	assertClose(t, a.Dot(b), 3)
	assertVec3(t, a.Cross(b), Vec3{X: 8, Y: 11, Z: -10})
}

func TestVec3Normalize(t *testing.T) {
	assertVec3(t, (Vec3{X: 3, Y: 4}).Normalize(), Vec3{X: 0.6, Y: 0.8})
	assertVec3(t, (Vec3{}).Normalize(), Vec3{})
	assertClose(t, (Vec3{X: 2, Y: -3, Z: 6}).Normalize().Length(), 1)
}

func assertVec3(t *testing.T, got, want Vec3) {
	t.Helper()
	assertClose(t, got.X, want.X)
	assertClose(t, got.Y, want.Y)
	assertClose(t, got.Z, want.Z)
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}
