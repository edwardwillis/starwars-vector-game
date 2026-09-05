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
