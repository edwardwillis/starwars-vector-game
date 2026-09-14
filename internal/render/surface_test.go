package render

import (
	"image/color"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

func TestFlatTrianglesReusePreparedGeometryAndSortBackToFront(t *testing.T) {
	geometry := PreparedGeometry{Faces: []model.Face{{}, {}}, Triangles: []PreparedTriangle{
		{A: Point{Depth: 2}, B: Point{Depth: 2}, C: Point{Depth: 2}},
		{Face: 1, A: Point{Depth: 8}, B: Point{Depth: 8}, C: Point{Depth: 8}},
	}}
	fill := color.RGBA{R: 20, G: 40, B: 60, A: 255}
	triangles := AppendFlatTriangles(nil, &geometry, fill)
	if len(triangles) != 2 || triangles[0].Color != fill {
		t.Fatalf("flat triangles=%+v", triangles)
	}
	SortFlatTriangles(triangles)
	if triangles[0].A.Depth != 8 || triangles[1].A.Depth != 2 {
		t.Fatalf("triangle order=%+v", triangles)
	}
}

func TestFlatTrianglesRejectNonOpaqueFill(t *testing.T) {
	geometry := PreparedGeometry{Faces: []model.Face{{}}, Triangles: []PreparedTriangle{{}}}
	if got := AppendFlatTriangles(nil, &geometry, color.RGBA{A: 128}); len(got) != 0 {
		t.Fatalf("accepted translucent flat triangles: %+v", got)
	}
}

func TestFlatTrianglesDoNotDrawOccluderOnlyFaces(t *testing.T) {
	geometry := PreparedGeometry{
		Faces:     []model.Face{{OccluderOnly: true}, {}},
		Triangles: []PreparedTriangle{{Face: 0}, {Face: 1}},
	}
	triangles := AppendFlatTriangles(nil, &geometry, color.RGBA{A: 255})
	if len(triangles) != 1 {
		t.Fatalf("drew %d triangles, want only the visible material face", len(triangles))
	}
}
