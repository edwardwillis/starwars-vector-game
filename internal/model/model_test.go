package model

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

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

func TestPrepareCompilesNormalsAndAdjacency(t *testing.T) {
	prepared := Prepare(Cube(2))
	if prepared.Topology == nil || len(prepared.Topology.FaceNormals) != 6 {
		t.Fatalf("topology was not compiled")
	}
	if len(prepared.Topology.Edges) != 12 {
		t.Fatalf("compiled edges=%d, want 12", len(prepared.Topology.Edges))
	}
	for index, normal := range prepared.Topology.FaceNormals {
		if normal.Length() < .99 || normal.Length() > 1.01 {
			t.Fatalf("face %d normal is not unit length: %v", index, normal)
		}
	}
	for index, face := range prepared.Faces {
		center := math3d.Vec3{}
		for _, vertex := range face.Vertices { center = center.Add(prepared.Verts[vertex]) }
		center = center.Scale(1 / float64(len(face.Vertices)))
		if prepared.Topology.FaceNormals[index].Dot(center) <= 0 { t.Fatalf("face %d normal is not outward: %v at %v", index, prepared.Topology.FaceNormals[index], center) }
	}
	for _, edge := range prepared.Topology.Edges {
		if edge.FaceA < 0 || edge.FaceB < 0 {
			t.Fatalf("cube edge lacks two adjacent faces: %+v", edge)
		}
		if len(edge.AdjacentFaces) != 2 {
			t.Fatalf("cube edge adjacency=%v, want two faces", edge.AdjacentFaces)
		}
	}
}

func TestPrepareDerivesBoundaryAdjacency(t *testing.T) {
	mesh := Prepare(Model{
		Verts: []math3d.Vec3{{X: -1}, {X: 1}, {Y: 1}},
		Faces: []Face{{Vertices: []int{0, 1, 2}}},
	})
	for _, edge := range mesh.Topology.Edges {
		if edge.FaceA < 0 || edge.FaceB >= 0 {
			t.Fatalf("triangle edge adjacency=%+v, want one incident face", edge)
		}
	}
}

func TestPrepareRetainsNonManifoldFaceAdjacency(t *testing.T) {
	mesh := Prepare(Model{
		Verts: []math3d.Vec3{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}, {X: 0, Z: 1}},
		Faces: []Face{
			{Vertices: []int{0, 1, 2}},
			{Vertices: []int{1, 0, 3}},
			{Vertices: []int{0, 1, 4}},
		},
	})
	for _, edge := range mesh.Topology.Edges {
		if edge.A == 0 && edge.B == 1 {
			if len(edge.AdjacentFaces) != 3 { t.Fatalf("shared edge adjacency=%v, want 3 faces", edge.AdjacentFaces) }
			return
		}
	}
	t.Fatal("did not find shared non-manifold edge")
}
