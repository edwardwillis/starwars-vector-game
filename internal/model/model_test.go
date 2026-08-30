package model

import "testing"

func TestCubeHasExpectedTopology(t *testing.T) {
	cube := Cube(2)
	if len(cube.Verts) != 8 {
		t.Fatalf("cube has %d vertices, want 8", len(cube.Verts))
	}
	if len(cube.Edges) != 12 {
		t.Fatalf("cube has %d edges, want 12", len(cube.Edges))
	}
	if err := cube.Validate(); err != nil {
		t.Fatalf("Cube returned an invalid model: %v", err)
	}
}

func TestValidateRejectsInvalidEdge(t *testing.T) {
	cube := Cube(2)
	cube.Edges[0].B = len(cube.Verts)
	if err := cube.Validate(); err == nil {
		t.Fatal("Validate did not reject an invalid edge")
	}
}
