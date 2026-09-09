package model

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestMillenniumFalconGeometryHasPreparedSolidParts(t *testing.T) {
	geometry := MillenniumFalconGeometryData()
	parts := map[string]Model{
		"full": geometry.Full, "hull": geometry.Hull, "left mandible": geometry.LeftMandible,
		"right mandible": geometry.RightMandible, "cargo ramp": geometry.CargoRamp, "corridor": geometry.Corridor,
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

func TestMillenniumFalconHullWeldsMandibleRootVertices(t *testing.T) {
	hull := MillenniumFalconGeometryData().Hull
	roots := []math3d.Vec3{}
	for _, side := range []float64{1, -1} {
		for _, angle := range []float64{falconMandibleInnerAngle, falconMandibleOuterAngle} {
			point := falconHullPointAtAngle(angle)
			point.X *= side
			roots = append(roots,
				math3d.Vec3{X: point.X, Y: falconHullDiskHalfDepth, Z: point.Z},
				math3d.Vec3{X: point.X, Y: -falconHullDiskHalfDepth, Z: point.Z},
			)
		}
	}
	for _, root := range roots {
		matches := 0
		for _, vertex := range hull.Verts {
			if vertex.Sub(root).Length() < 1e-9 {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("hull root %v has %d welded vertex matches, want 1", root, matches)
		}
	}
}

func TestMillenniumFalconHullOwnsCargoAssemblyTopology(t *testing.T) {
	geometry := MillenniumFalconGeometryData()
	if len(geometry.Hull.Verts) < len(geometry.CargoRamp.Verts) {
		t.Fatalf("composite hull has fewer vertices than cargo assembly: hull=%d cargo=%d", len(geometry.Hull.Verts), len(geometry.CargoRamp.Verts))
	}
	// Cargo is now part of the hull's single depth owner. Every authored cargo
	// vertex must therefore be present in the composite mesh, while the
	// semantic CargoRamp source remains available for destruction fragments.
	for _, cargoVertex := range geometry.CargoRamp.Verts {
		found := false
		for _, hullVertex := range geometry.Hull.Verts {
			if hullVertex.Sub(cargoVertex).Length() < 1e-9 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("cargo vertex %v is missing from composite hull", cargoVertex)
		}
	}
	rampJoin := falconInnerHullPointAtX(0.52)
	roofJoin := falconInnerHullPointAtX(0.52 * 0.82)
	for _, point := range []math3d.Vec3{
		{X: -0.52, Y: -falconInnerHullHalfDepth, Z: rampJoin.Z},
		{X: 0.52, Y: -falconInnerHullHalfDepth, Z: rampJoin.Z},
		{X: -0.52 * 0.82, Y: falconInnerHullHalfDepth, Z: roofJoin.Z},
		{X: 0.52 * 0.82, Y: falconInnerHullHalfDepth, Z: roofJoin.Z},
	} {
		matches := 0
		for _, vertex := range geometry.Hull.Verts {
			if vertex.Sub(point).Length() < 1e-9 {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("cargo/hull attachment point %v has %d composite vertices, want one welded point", point, matches)
		}
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

func TestMillenniumFalconSensorDishBowlFacesOpenForward(t *testing.T) {
	dish := parabolicSensorDish()
	// Two face bands per segment form the concave bowl; the third is the rim
	// wall. The bowl normals must point toward the +Z opening.
	for faceIndex := 0; faceIndex < 36; faceIndex++ {
		if faceIndex%3 != 2 && dish.Faces[faceIndex].Normal.Z <= 0 {
			t.Fatalf("dish bowl face %d normal=%v does not point toward +Z opening", faceIndex, dish.Faces[faceIndex].Normal)
		}
	}
}
