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

func TestOpaqueTrianglesRetainTextureAndUV(t *testing.T) {
	uvs := [3]model.UV{{U: 0, V: 0}, {U: 1, V: 0}, {U: 0, V: 1}}
	geometry := PreparedGeometry{
		Faces:     []model.Face{{}},
		Triangles: []PreparedTriangle{{Face: 0, UVs: uvs}},
	}
	got := AppendOpaqueTriangles(nil, &geometry, color.RGBA{A: 255}, "test/panel")
	if len(got) != 1 || got[0].TextureID != "test/panel" || got[0].UVs != uvs {
		t.Fatalf("textured surface lost material coordinates: %+v", got)
	}
}

func TestTranslucentTrianglesReusePreparedGeometryAndPainterOrder(t *testing.T) {
	uvs := [3]model.UV{{}, {U: 1}, {V: 1}}
	geometry := PreparedGeometry{Faces: []model.Face{{}, {OccluderOnly: true}}, Triangles: []PreparedTriangle{
		{Face: 0, A: Point{Depth: 3}, B: Point{Depth: 3}, C: Point{Depth: 3}, UVs: uvs},
		{Face: 1, A: Point{Depth: 9}, B: Point{Depth: 9}, C: Point{Depth: 9}},
	}}
	triangles := AppendTranslucentTriangles(nil, &geometry, color.RGBA{B: 255, A: 96}, "test/glass")
	if len(triangles) != 1 || !triangles[0].Translucent || triangles[0].TextureID != "test/glass" || triangles[0].UVs != uvs {
		t.Fatalf("translucent triangles=%+v", triangles)
	}
	triangles = append(triangles, FlatTriangle{A: Point{Depth: 5}, B: Point{Depth: 5}, C: Point{Depth: 5}})
	SortFlatTriangles(triangles)
	if triangles[0].Translucent || !triangles[1].Translucent {
		t.Fatalf("near glass should follow far opaque surface: %+v", triangles)
	}
	if got := AppendTranslucentTriangles(nil, &geometry, color.RGBA{A: 255}, ""); len(got) != 0 {
		t.Fatalf("accepted opaque material in translucent path: %+v", got)
	}
}
