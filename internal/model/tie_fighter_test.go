package model

import (
	"math"
	"testing"
)

func TestTIEFighterIsValidAndSymmetrical(t *testing.T) {
	fighter := TIEFighter()
	if len(fighter.Verts) != 70 {
		t.Fatalf("fighter has %d vertices, want 70", len(fighter.Verts))
	}
	if len(fighter.Edges) != 148 {
		t.Fatalf("fighter has %d edges, want 148", len(fighter.Edges))
	}
	if err := fighter.Validate(); err != nil {
		t.Fatalf("TIEFighter returned an invalid model: %v", err)
	}

	minX, maxX := fighter.Verts[0].X, fighter.Verts[0].X
	for _, vertex := range fighter.Verts[1:] {
		minX = min(minX, vertex.X)
		maxX = max(maxX, vertex.X)
	}
	if minX != -maxX {
		t.Fatalf("fighter X bounds are [%v,%v], want symmetry", minX, maxX)
	}
	if math.Abs(maxX-1.61) > 1e-9 {
		t.Fatalf("fighter panel position is %v, want 1.61", maxX)
	}
}

func TestTIEFighterWindowIsValid(t *testing.T) {
	window := TIEFighterWindow()
	if len(window.Verts) != 9 {
		t.Fatalf("window has %d vertices, want 9", len(window.Verts))
	}
	if len(window.Edges) != 16 {
		t.Fatalf("window has %d edges, want 16", len(window.Edges))
	}
	if err := window.Validate(); err != nil {
		t.Fatalf("TIEFighterWindow returned an invalid model: %v", err)
	}
}

func TestTIEFighterGeometryDataIncludesSharedVariants(t *testing.T) {
	geometry := TIEFighterGeometryData()
	for name, mesh := range map[string]Model{
		"full":       geometry.Full,
		"core":       geometry.Core,
		"left foil":  geometry.LeftFoil,
		"right foil": geometry.RightFoil,
		"window":     geometry.Window,
	} {
		if err := mesh.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if len(geometry.Fragments) != 3 {
		t.Fatalf("fragments=%d, want 3", len(geometry.Fragments))
	}
}

func TestTIEFighterFragmentsReconstructEveryHullEdge(t *testing.T) {
	hull := TIEFighter()
	fragments := TIEFighterFragments()
	edgeCount := 0
	faceCount := 0
	for index, fragment := range fragments {
		if err := fragment.Validate(); err != nil {
			t.Fatalf("fragment %d is invalid: %v", index, err)
		}
		if len(fragment.Edges) == 0 {
			t.Fatalf("fragment %d contains no edges", index)
		}
		edgeCount += len(fragment.Edges)
		faceCount += len(fragment.Faces)
	}
	if edgeCount < len(hull.Edges) {
		t.Fatalf("fragments contain %d edges, fewer than hull's %d", edgeCount, len(hull.Edges))
	}
	wantFaces := len(hull.Faces) + 4 // two split planes, with a cap on each side
	if faceCount != wantFaces {
		t.Fatalf("fragments contain %d faces, want hull faces plus fracture caps (%d)", faceCount, wantFaces)
	}
}

func TestPanelIsARegularHexagon(t *testing.T) {
	mesh := Model{}
	appendPanel(&mesh, 0, nil)
	wantSide := mesh.Verts[0].Sub(mesh.Verts[1]).Length()
	for corner := 1; corner < 6; corner++ {
		next := (corner + 1) % 6
		gotSide := mesh.Verts[corner].Sub(mesh.Verts[next]).Length()
		if math.Abs(gotSide-wantSide) > 1e-9 {
			t.Fatalf("panel side %d has length %v, want %v", corner, gotSide, wantSide)
		}
	}
}
