package render

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

func TestCircleOccluderDepthAndSilhouette(t *testing.T) {
	occluder := CircleOccluder{CenterX: 100, CenterY: 100, Radius: 20, Depth: 10}
	if !occluder.OccludesPoint(Point{X: 100, Y: 100, Depth: 11}) {
		t.Fatal("point inside circle and behind sphere was retained")
	}
	if occluder.OccludesPoint(Point{X: 100, Y: 100, Depth: 9}) {
		t.Fatal("point in front of sphere was rejected")
	}
	if occluder.OccludesPoint(Point{X: 121, Y: 100, Depth: 11}) {
		t.Fatal("point outside circle was rejected")
	}
}

func TestCircleOccluderUsesFrontSurfaceDepth(t *testing.T) {
	occluder := CircleOccluder{CenterX: 0, CenterY: 0, Radius: 10, Depth: 10, SphereRadius: 5}
	if !occluder.OccludesPoint(Point{X: 0, Y: 0, Depth: 6}) {
		t.Fatal("point behind spherical front surface was retained")
	}
	if occluder.OccludesPoint(Point{X: 0, Y: 0, Depth: 4}) {
		t.Fatal("point in front of spherical surface was rejected")
	}
}

func TestTriangleOccluderDepthAndEdges(t *testing.T) {
	triangle := TriangleOccluder{
		A: Point{X: 0, Y: 0, Depth: 5},
		B: Point{X: 10, Y: 0, Depth: 5},
		C: Point{X: 0, Y: 10, Depth: 5},
	}
	if !triangle.OccludesPoint(Point{X: 2, Y: 2, Depth: 6}) {
		t.Fatal("point inside triangle and behind face was retained")
	}
	if triangle.OccludesPoint(Point{X: 2, Y: 2, Depth: 4}) {
		t.Fatal("point in front of face was rejected")
	}
	if triangle.OccludesPoint(Point{X: 8, Y: 8, Depth: 6}) {
		t.Fatal("point outside triangle was rejected")
	}
}

func TestProjectSolidOccludersCullsBackFaces(t *testing.T) {
	mesh := model.Prepare(model.Model{
		Verts: []math3d.Vec3{{X: 0, Y: 0, Z: -5}, {X: 1, Y: 0, Z: -5}, {X: 0, Y: 1, Z: -5}},
		Faces: []model.Face{{Vertices: []int{0, 1, 2}}},
	})
	pipeline := NewPipeline(100, 100, math.Pi/2, .1, 100)
	if got := pipeline.ProjectSolidOccluders(mesh, math3d.Identity()); len(got) != 1 {
		t.Fatalf("front-facing triangle produced %d occluders, want 1", len(got))
	}
	back := model.Prepare(model.Model{
		Verts: mesh.Verts,
		Faces: []model.Face{{Vertices: []int{0, 2, 1}}},
	})
	if got := pipeline.ProjectSolidOccluders(back, math3d.Identity()); len(got) != 0 {
		t.Fatalf("back-facing triangle produced %d occluders, want 0", len(got))
	}
}
