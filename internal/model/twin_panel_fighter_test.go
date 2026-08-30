package model

import (
	"math"
	"testing"
)

func TestTwinPanelFighterIsValidAndSymmetrical(t *testing.T) {
	fighter := TwinPanelFighter()
	if len(fighter.Verts) != 56 {
		t.Fatalf("fighter has %d vertices, want 56", len(fighter.Verts))
	}
	if len(fighter.Edges) != 112 {
		t.Fatalf("fighter has %d edges, want 112", len(fighter.Edges))
	}
	if err := fighter.Validate(); err != nil {
		t.Fatalf("TwinPanelFighter returned an invalid model: %v", err)
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

func TestTwinPanelFighterWindowIsValid(t *testing.T) {
	window := TwinPanelFighterWindow()
	if len(window.Verts) != 9 {
		t.Fatalf("window has %d vertices, want 9", len(window.Verts))
	}
	if len(window.Edges) != 16 {
		t.Fatalf("window has %d edges, want 16", len(window.Edges))
	}
	if err := window.Validate(); err != nil {
		t.Fatalf("TwinPanelFighterWindow returned an invalid model: %v", err)
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
