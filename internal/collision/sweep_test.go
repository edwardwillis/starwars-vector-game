package collision

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestSegmentSphereDetectsSweptHit(t *testing.T) {
	hitTime, hit := SegmentSphere(
		math3d.Vec3{X: -5},
		math3d.Vec3{X: 5},
		math3d.Vec3{},
		1,
	)
	if !hit || math.Abs(hitTime-0.4) > 1e-9 {
		t.Fatalf("SegmentSphere returned time=%v hit=%v, want 0.4 true", hitTime, hit)
	}
}

func TestSegmentSphereRejectsMissAndDetectsInitialOverlap(t *testing.T) {
	if _, hit := SegmentSphere(math3d.Vec3{X: -5, Y: 2}, math3d.Vec3{X: 5, Y: 2}, math3d.Vec3{}, 1); hit {
		t.Fatal("SegmentSphere reported a miss as a hit")
	}
	if hitTime, hit := SegmentSphere(math3d.Vec3{X: 0.5}, math3d.Vec3{X: 2}, math3d.Vec3{}, 1); !hit || hitTime != 0 {
		t.Fatalf("initial overlap returned time=%v hit=%v, want 0 true", hitTime, hit)
	}
}
