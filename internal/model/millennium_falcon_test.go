package model

import (
	"math"
	"testing"
)

func TestMillenniumFalconGeometryHasPreparedSolidParts(t *testing.T) {
	geometry := MillenniumFalconGeometryData()
	parts := map[string]Model{
		"full": geometry.Full, "hull": geometry.Hull, "left mandible": geometry.LeftMandible,
		"right mandible": geometry.RightMandible, "corridor": geometry.Corridor, "cockpit": geometry.Cockpit,
		"window": geometry.Window, "turrets": geometry.Turrets, "engine": geometry.Engine,
	}
	for name, part := range parts {
		if err := part.Validate(); err != nil {
			t.Fatalf("%s invalid: %v", name, err)
		}
		if part.Topology == nil || len(part.Faces) == 0 {
			t.Fatalf("%s has no prepared surface topology", name)
		}
	}
	if geometry.Full.Topology.BoundsRadius <= geometry.Hull.Topology.BoundsRadius {
		t.Fatal("full Falcon does not extend beyond its hull")
	}
	for index, fragment := range geometry.Fragments {
		if err := fragment.Validate(); err != nil {
			t.Fatalf("fragment %d invalid: %v", index, err)
		}
		if len(fragment.PolygonModels()) == 0 {
			t.Fatalf("fragment %d has no polygon shards", index)
		}
	}
}

func TestMillenniumFalconMandiblesRemainMirroredWithForwardGap(t *testing.T) {
	geometry := MillenniumFalconGeometryData()
	left, right := geometry.LeftMandible.Topology.BoundsCenter, geometry.RightMandible.Topology.BoundsCenter
	if math.Abs(left.X+right.X) > 1e-9 || math.Abs(left.Y-right.Y) > 1e-9 || math.Abs(left.Z-right.Z) > 1e-9 {
		t.Fatalf("mandibles are not mirrored: left=%+v right=%+v", left, right)
	}
	if left.Z <= geometry.Hull.Topology.BoundsCenter.Z || right.Z <= geometry.Hull.Topology.BoundsCenter.Z {
		t.Fatal("mandibles do not extend forward of the hull")
	}
}

func TestMillenniumFalconHullDeepensTowardCentre(t *testing.T) {
	hull := MillenniumFalconGeometryData().Hull
	var centreDepth, rimDepth float64
	for _, vertex := range hull.Verts {
		radius := math.Hypot(vertex.X, vertex.Z)
		depth := math.Abs(vertex.Y)
		if radius < 0.1 && depth > centreDepth {
			centreDepth = depth
		}
		if radius > 4.0 && depth > rimDepth {
			rimDepth = depth
		}
	}
	if centreDepth <= rimDepth {
		t.Fatalf("hull centre is not deeper than rim: centre=%f rim=%f", centreDepth, rimDepth)
	}
}
