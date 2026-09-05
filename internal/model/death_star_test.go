package model

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestDeathStarGeometryIsValidAndSparse(t *testing.T) {
	geometry := DeathStar(15)
	models := map[string]Model{"sphere": geometry.Sphere, "dish": geometry.Dish}
	totalEdges := 0
	for name, mesh := range models {
		if err := mesh.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(mesh.Edges) == 0 {
			t.Fatalf("%s has no edges", name)
		}
		totalEdges += len(mesh.Edges)
	}
	for index, face := range geometry.Sphere.Faces {
		center := math3d.Vec3{}
		for _, vertex := range face.Vertices { center = center.Add(geometry.Sphere.Verts[vertex]) }
		center = center.Scale(1 / float64(len(face.Vertices)))
		if geometry.Sphere.Faces[index].Normal.Dot(center) <= 0 {
			t.Fatalf("sphere face %d normal points inward", index)
		}
	}
	if totalEdges > 250 {
		t.Fatalf("orbital Death Star uses %d edges, want at most 250", totalEdges)
	}
}

func TestDeathStarDishOccupiesUpperNearHemisphere(t *testing.T) {
	geometry := DeathStar(15)
	for _, vertex := range geometry.Dish.Verts {
		if vertex.Y > 0 && vertex.Z < 0 {
			return
		}
	}
	t.Fatal("dish has no geometry on upper near hemisphere")
}
