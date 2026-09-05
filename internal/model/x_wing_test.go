package model

import (
	"math"
	"testing"
)

func TestXWingGeometry(t *testing.T) {
	ship := XWing()
	if err := ship.Validate(); err != nil { t.Fatal(err) }
	if len(ship.Verts) == 0 || len(ship.Edges) == 0 || len(ship.Faces) == 0 { t.Fatal("X-Wing geometry is empty") }
	if len(ship.Verts) < 100 { t.Fatalf("vertices=%d, want substantial composed geometry", len(ship.Verts)) }
}

func TestXWingFacesForwardAlongPositiveZ(t *testing.T) {
	ship := XWing()
	minZ, maxZ := math.Inf(1), math.Inf(-1)
	for _, vertex := range ship.Verts { minZ = math.Min(minZ, vertex.Z); maxZ = math.Max(maxZ, vertex.Z) }
	if maxZ-minZ < 3.5 || maxZ <= 0 { t.Fatalf("unexpected longitudinal extent: %v..%v", minZ, maxZ) }
}

func TestXWingHasSymmetricSeparatedFoils(t *testing.T) {
	ship := XWing()
	upper, lower := false, false
	for _, vertex := range ship.Verts {
		if vertex.Y > 0.65 { upper = true }
		if vertex.Y < -0.65 { lower = true }
	}
	if !upper || !lower { t.Fatal("S-foils lack meaningful vertical separation") }
	minX, maxX := math.Inf(1), math.Inf(-1)
	for _, vertex := range ship.Verts { minX = math.Min(minX, vertex.X); maxX = math.Max(maxX, vertex.X) }
	if math.Abs(minX+maxX) > 1e-9 { t.Fatalf("left/right bounds are asymmetric: %v..%v", minX, maxX) }
}

func TestXWingFragmentsValidate(t *testing.T) {
	for index, fragment := range XWingFragments() {
		if err := fragment.Validate(); err != nil { t.Fatalf("fragment %d: %v", index, err) }
	}
}

func TestXWingWingSlabUsesPlanarSurfaceFaces(t *testing.T) {
	wing := wingSlab()
	if len(wing.Faces) != 12 {
		t.Fatalf("wing slab faces = %d, want 12 triangles", len(wing.Faces))
	}
	for index, face := range wing.Faces {
		if len(face.Vertices) != 3 {
			t.Fatalf("wing face %d has %d vertices, want triangle", index, len(face.Vertices))
		}
		if face.Normal.Length() <= 1e-9 {
			t.Fatalf("wing face %d has no usable normal", index)
		}
	}
	for pair := 0; pair < len(wing.Faces); pair += 2 {
		if wing.Faces[pair].Normal.Dot(wing.Faces[pair+1].Normal) < 0.999 {
			t.Fatalf("wing triangle pair %d/%d is not coplanar: normals %+v and %+v", pair, pair+1, wing.Faces[pair].Normal, wing.Faces[pair+1].Normal)
		}
	}
	for _, edge := range wing.Edges {
		if edge.Kind == EdgeInternal && len(edge.AdjacentFaces) != 2 {
			t.Fatalf("internal wing edge %d-%d has adjacency %v", edge.A, edge.B, edge.AdjacentFaces)
		}
	}
	minZ := math.Inf(1)
	for _, vertex := range wing.Verts { minZ = math.Min(minZ, vertex.Z) }
	if minZ > -0.75 {
		t.Fatalf("wing trailing edge does not sweep aft enough: minimum Z=%v", minZ)
	}
	if thickness := math.Abs(wing.Verts[0].Y - wing.Verts[4].Y); math.Abs(thickness-.06) > 1e-9 {
		t.Fatalf("wing thickness=%v, want 0.06", thickness)
	}
	tipChord := math.Abs(wing.Verts[1].Z - wing.Verts[2].Z)
	if tipChord >= 1 {
		t.Fatalf("wing tip chord=%v, want shortened chord below 1", tipChord)
	}
	if wing.Verts[0].Z > .25 || wing.Verts[1].Z > .25 {
		t.Fatalf("wing leading edge starts too far forward: root=%v tip=%v", wing.Verts[0].Z, wing.Verts[1].Z)
	}
	if math.Abs(wing.Verts[0].X-.55) > 1e-9 || math.Abs(wing.Verts[0].Y-.15) > 1e-9 {
		t.Fatalf("wing root is too close to centerline: (%v,%v)", wing.Verts[0].X, wing.Verts[0].Y)
	}
}

func TestXWingFoilPartsHaveIndependentEngineSections(t *testing.T) {
	parts := XWingFoilParts()
	if len(parts) != 16 {
		t.Fatalf("foil parts = %d, want 16 (4 assemblies × 4 components)", len(parts))
	}
	for index, part := range parts {
		if err := part.Validate(); err != nil {
			t.Fatalf("foil part %d: %v", index, err)
		}
	}
	// The first assembly's nacelles are mounted outboard of the fuselage, not
	// left at the canonical wing origin after component splitting.
	if parts[1].Topology.BoundsCenter.X < 0.5 || parts[2].Topology.BoundsCenter.X < 0.5 {
		t.Fatalf("engine sections are not mounted on the wing: centers %+v and %+v", parts[1].Topology.BoundsCenter, parts[2].Topology.BoundsCenter)
	}
}

func TestXWingCorePartsAreIndependentlyOccluding(t *testing.T) {
	parts := XWingCoreParts()
	if len(parts) != 2 {
		t.Fatalf("core parts = %d, want fuselage and canopy", len(parts))
	}
	for index, part := range parts {
		if err := part.Validate(); err != nil {
			t.Fatalf("core part %d: %v", index, err)
		}
	}
}

func TestXWingNacelleSectionsAreClosedSolids(t *testing.T) {
	for index, section := range xWingEngineParts() {
		if err := section.Validate(); err != nil {
			t.Fatalf("engine section %d: %v", index, err)
		}
		if len(section.Faces) < 8 {
			t.Fatalf("engine section %d has %d faces, want capped solid", index, len(section.Faces))
		}
	}
}
