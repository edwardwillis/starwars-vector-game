package environment

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestPanelSurfaceMappingSharesCoordinatesAcrossAdjacentPanels(t *testing.T) {
	mapping := PanelSurfaceMapping{
		Family: PanelFamily{ID: "test", TextureID: "test/panel", RepeatDistance: 20},
		Origin: math3d.Vec3{X: -10, Y: 3, Z: 7}, AxisU: math3d.Vec3{X: 1}, AxisV: math3d.Vec3{Z: 1},
	}
	left := mapping.UVs([]math3d.Vec3{{X: -30, Y: 3, Z: 7}, {X: 10, Y: 3, Z: 7}, {X: 10, Y: 3, Z: 47}})
	right := mapping.UVs([]math3d.Vec3{{X: 10, Y: 3, Z: 7}, {X: 50, Y: 3, Z: 7}, {X: 10, Y: 3, Z: 47}})
	if left[1] != right[0] || left[2] != right[2] {
		t.Fatalf("shared panel boundaries reset pattern: left=%+v right=%+v", left, right)
	}
}

func TestPanelSurfaceMappingSupportsArbitrarySurfaceOrientation(t *testing.T) {
	mapping := PanelSurfaceMapping{
		Family: PanelFamily{ID: "test", RepeatDistance: 10},
		Origin: math3d.Vec3{X: 5, Y: 2, Z: -4}, AxisU: math3d.Vec3{Z: 1}, AxisV: math3d.Vec3{Y: 1},
	}
	uv := mapping.UV(math3d.Vec3{X: 5, Y: 32, Z: 16})
	if math.Abs(uv.U-2) > 1e-9 || math.Abs(uv.V-3) > 1e-9 {
		t.Fatalf("wall-local UV=%+v, want {2 3}", uv)
	}
}
