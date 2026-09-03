package collision

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestSweepSpherePlaneReturnsContact(t *testing.T) {
	plane := FinitePlane{Normal: math3d.Vec3{Y: 1}, AxisU: math3d.Vec3{X: 1}, HalfU: 10, HalfV: 10, FeatureID: "deck"}
	hit, ok := SweepSpherePlane(math3d.Vec3{Y: 5}, math3d.Vec3{Y: -5}, 1, plane)
	if !ok || math.Abs(hit.Time-0.4) > 1e-9 || hit.Point.Y != 0 || hit.FeatureID != "deck" {
		t.Fatalf("hit=%+v ok=%v", hit, ok)
	}
}

func TestSweepSpherePlaneHonorsFiniteBounds(t *testing.T) {
	plane := FinitePlane{Normal: math3d.Vec3{Y: 1}, AxisU: math3d.Vec3{X: 1}, HalfU: 1, HalfV: 1}
	if _, ok := SweepSpherePlane(math3d.Vec3{X: 2, Y: 5}, math3d.Vec3{X: 2, Y: -5}, 0.5, plane); ok {
		t.Fatal("out-of-bounds plane hit")
	}
}

func TestSweepSphereOrientedBox(t *testing.T) {
	box := OrientedBox{Orientation: math3d.IdentityQuaternion(), HalfExtents: math3d.Vec3{X: 1, Y: 1, Z: 1}, FeatureID: "tower"}
	hit, ok := SweepSphereBox(math3d.Vec3{X: -5}, math3d.Vec3{X: 5}, 0.5, box)
	if !ok || math.Abs(hit.Time-0.35) > 1e-9 || hit.Normal.X != -1 {
		t.Fatalf("hit=%+v ok=%v", hit, ok)
	}
}
