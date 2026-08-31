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
	if len(cube.Faces) != 6 {
		t.Fatalf("cube has %d faces, want 6", len(cube.Faces))
	}
	if err := cube.Validate(); err != nil {
		t.Fatalf("Cube returned an invalid model: %v", err)
	}
}

func TestPolygonModelsReturnClosedFaceMeshes(t *testing.T) {
	polygons := Cube(2).PolygonModels()
	if len(polygons) != 6 {
		t.Fatalf("cube produced %d polygons, want 6", len(polygons))
	}
	for index, polygon := range polygons {
		if len(polygon.Verts) != 4 || len(polygon.Edges) != 4 || len(polygon.Faces) != 1 {
			t.Fatalf("polygon %d has unexpected topology", index)
		}
		if err := polygon.Validate(); err != nil {
			t.Fatalf("polygon %d is invalid: %v", index, err)
		}
	}
}

func TestValidateRejectsInvalidEdge(t *testing.T) {
	cube := Cube(2)
	cube.Edges[0].B = len(cube.Verts)
	if err := cube.Validate(); err == nil {
		t.Fatal("Validate did not reject an invalid edge")
	}
}
