package camera

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestCycleVisitsEveryMode(t *testing.T) {
	camera := New(1)
	for _, want := range []Mode{Chase, Cockpit, Orbit, Fixed} {
		camera.Cycle()
		if camera.Mode != want {
			t.Fatalf("Cycle selected %v, want %v", camera.Mode, want)
		}
	}
}

func TestCockpitLooksAlongObjectForwardAxis(t *testing.T) {
	object := catalog.TwinPanelFighter(1, kinematics.Pose{Orientation: math3d.IdentityQuaternion()})
	camera := New(object.ID)
	camera.Mode = Cockpit
	viewPoint := camera.View([]scene.Object{object}).TransformPoint(math3d.Vec3{Z: 2})
	if viewPoint.Z >= 0 {
		t.Fatalf("forward point has camera Z %v, want negative", viewPoint.Z)
	}
}

func TestMissingTargetFallsBackToFixed(t *testing.T) {
	camera := New(99)
	camera.Mode = Chase
	view := camera.View(nil)
	if camera.Mode != Fixed {
		t.Fatalf("missing target left mode at %v, want Fixed", camera.Mode)
	}
	if view != math3d.Identity() {
		t.Fatalf("fallback view is %+v, want identity", view)
	}
}

func TestZoomIsStoredPerMode(t *testing.T) {
	camera := New(1)
	camera.AdjustZoom(1)
	camera.Mode = Chase
	if camera.Zoom() != 0 {
		t.Fatalf("chase inherited fixed zoom %v", camera.Zoom())
	}
	camera.AdjustZoom(-1)
	camera.Mode = Fixed
	if camera.Zoom() != 1 {
		t.Fatalf("fixed zoom changed to %v, want 1", camera.Zoom())
	}
}
