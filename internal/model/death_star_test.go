package model

import "testing"

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
