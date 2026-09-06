package model

import (
	"math"
	"testing"
)

func TestTIEInterceptorGeometryIsPreparedAndSurfaceBased(t *testing.T) {
	geometry := TIEInterceptorGeometryData()
	for name, mesh := range map[string]Model{
		"full":   geometry.Full,
		"core":   geometry.Core,
		"window": geometry.Window,
	} {
		if err := mesh.Validate(); err != nil {
			t.Fatalf("%s mesh invalid: %v", name, err)
		}
		if mesh.Topology == nil || len(mesh.Faces) == 0 {
			t.Fatalf("%s mesh has no prepared surface topology", name)
		}
	}
	for index, panel := range geometry.Panels {
		if err := panel.Validate(); err != nil {
			t.Fatalf("panel %d invalid: %v", index, err)
		}
		if len(panel.Faces) < 6 {
			t.Fatalf("panel %d has %d faces, want an extruded solid", index, len(panel.Faces))
		}
		if panel.SkipDepth {
			t.Fatalf("panel %d incorrectly skips depth", index)
		}
	}
	for index, panel := range geometry.EndPanels {
		if err := panel.Validate(); err != nil {
			t.Fatalf("outer panel %d invalid: %v", index, err)
		}
		if len(panel.Faces) != 6 {
			t.Fatalf("outer panel %d has %d faces, want a closed plate", index, len(panel.Faces))
		}
	}
}

func TestTIEInterceptorPanelsAreSymmetricAndLeaveGaps(t *testing.T) {
	geometry := TIEInterceptorGeometryData()
	for _, pair := range [][2]int{{0, 3}, {1, 2}} {
		first := geometry.Panels[pair[0]].Topology.BoundsCenter
		opposite := geometry.Panels[pair[1]].Topology.BoundsCenter
		if math.Abs(first.X+opposite.X) > 1e-9 || math.Abs(first.Y+opposite.Y) > 1e-9 || math.Abs(first.Z-opposite.Z) > 1e-9 {
			t.Fatalf("panels %d and %d are not opposite: %+v %+v", pair[0], pair[1], first, opposite)
		}
	}
	if geometry.Full.Topology.BoundsRadius <= geometry.Core.Topology.BoundsRadius {
		t.Fatal("full interceptor does not extend beyond command pod")
	}
}

func TestTIEInterceptorFragmentsAreRenderable(t *testing.T) {
	geometry := TIEInterceptorGeometryData()
	for index, fragment := range geometry.Fragments {
		if err := fragment.Validate(); err != nil {
			t.Fatalf("fragment %d invalid: %v", index, err)
		}
		if len(fragment.PolygonModels()) == 0 {
			t.Fatalf("fragment %d has no polygon shards", index)
		}
	}
}
