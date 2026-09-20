package render

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

func TestClipLineToConvexPolygonKeepsOnlyDoorwaySpan(t *testing.T) {
	boundary := []Point{{X: 10, Y: 10}, {X: 30, Y: 10}, {X: 30, Y: 30}, {X: 10, Y: 30}}
	for _, polygon := range [][]Point{boundary, {boundary[3], boundary[2], boundary[1], boundary[0]}} {
		line, ok := ClipLineToConvexPolygon(Line{X1: 0, Y1: 20, X2: 40, Y2: 20}, polygon)
		if !ok || math.Abs(line.X1-10) > 1e-9 || math.Abs(line.X2-30) > 1e-9 {
			t.Fatalf("crossing line not clipped to doorway: %+v, %v", line, ok)
		}
		if _, ok := ClipLineToConvexPolygon(Line{X1: 0, Y1: 0, X2: 40, Y2: 0}, polygon); ok {
			t.Fatal("outside line leaked through portal")
		}
	}
}

func TestClipPreparedTrianglesToConvexConfinesDepthAndPreservesPerspective(t *testing.T) {
	boundary := []Point{{X: 10, Y: 10}, {X: 30, Y: 10}, {X: 30, Y: 30}, {X: 10, Y: 30}}
	geometry := PreparedGeometry{Triangles: []PreparedTriangle{{
		Face: 0,
		A:    Point{X: 0, Y: 20, Depth: 2}, B: Point{X: 20, Y: 0, Depth: 4}, C: Point{X: 40, Y: 40, Depth: 4},
		UVs: [3]model.UV{{U: 0}, {U: 1}, {U: 1}},
	}}}
	ClipPreparedTrianglesToConvex(&geometry, boundary)
	if len(geometry.Triangles) == 0 {
		t.Fatal("partially visible face was lost")
	}
	for _, triangle := range geometry.Triangles {
		for _, point := range [...]Point{triangle.A, triangle.B, triangle.C} {
			if point.X < 10-1e-8 || point.X > 30+1e-8 || point.Y < 10-1e-8 || point.Y > 30+1e-8 {
				t.Fatalf("clipped face extends beyond doorway: %+v", point)
			}
			if point.Depth <= 0 || math.IsNaN(point.Depth) {
				t.Fatalf("invalid interpolated depth: %+v", point)
			}
		}
	}
	// At a halfway screen-space intersection, perspective depth is harmonic,
	// not the arithmetic mean of the endpoint depths.
	interpolated := interpolatePortalVertex(portalVertex{point: Point{Depth: 2}}, portalVertex{point: Point{Depth: 4}}, .5)
	if math.Abs(interpolated.point.Depth-8.0/3.0) > 1e-9 {
		t.Fatalf("incorrect perspective depth: %v", interpolated.point.Depth)
	}
	ClipPreparedTrianglesToConvex(&geometry, []Point{{X: 100, Y: 100}, {X: 110, Y: 100}, {X: 110, Y: 110}, {X: 100, Y: 110}})
	if len(geometry.Triangles) != 0 {
		t.Fatal("fully outside face retained")
	}
}
