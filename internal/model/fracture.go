package model

import (
	"math"
	"sort"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// addFractureCaps adds sparse, outward-facing cut surfaces on both sides of
// each split plane. The caps are ordinary model faces, so all existing
// back-face, depth, clipping, and hidden-line stages apply automatically.
func addFractureCaps(source Model, fragments *[3]Model, planes []float64) {
	if fragments == nil {
		return
	}
	for index := range fragments {
		fragments[index].Verts = append([]math3d.Vec3(nil), fragments[index].Verts...)
	}
	for splitIndex, plane := range planes {
		polygon := fractureSection(source, plane)
		if len(polygon) < 3 || splitIndex+1 >= len(fragments) {
			continue
		}
		addFractureCap(&fragments[splitIndex], polygon, -1)
		addFractureCap(&fragments[splitIndex+1], polygon, 1)
	}
}

func addFractureCap(fragment *Model, polygon []math3d.Vec3, normalSign float64) {
	if fragment == nil || len(polygon) < 3 {
		return
	}
	base := len(fragment.Verts)
	fragment.Verts = append(fragment.Verts, polygon...)
	indices := make([]int, len(polygon))
	for index := range polygon {
		indices[index] = base + index
		fragment.Edges = append(fragment.Edges, Edge{A: base + index, B: base + (index+1)%len(polygon), Kind: EdgeStructural, Importance: 1})
	}
	normal := polygon[1].Sub(polygon[0]).Cross(polygon[2].Sub(polygon[0]))
	if normal.X*normalSign < 0 {
		for left, right := 0, len(indices)-1; left < right; left, right = left+1, right-1 {
			indices[left], indices[right] = indices[right], indices[left]
		}
	}
	fragment.Faces = append(fragment.Faces, Face{Vertices: indices})
}

// fractureSection returns a convex hull of source-edge intersections with an
// X split plane. The Y/Z hull keeps caps tight to the actual break profile
// rather than introducing a large rectangular surface across the model.
func fractureSection(source Model, plane float64) []math3d.Vec3 {
	points := make([]math3d.Vec3, 0)
	for _, edge := range source.Edges {
		if edge.A < 0 || edge.A >= len(source.Verts) || edge.B < 0 || edge.B >= len(source.Verts) {
			continue
		}
		a, b := source.Verts[edge.A], source.Verts[edge.B]
		da, db := a.X-plane, b.X-plane
		if math.Abs(da) <= 1e-9 {
			appendUniqueSectionPoint(&points, a)
		}
		if da*db < -1e-12 {
			t := da / (da - db)
			appendUniqueSectionPoint(&points, a.Add(b.Sub(a).Scale(t)))
		}
		if math.Abs(db) <= 1e-9 {
			appendUniqueSectionPoint(&points, b)
		}
	}
	if len(points) < 3 {
		return nil
	}
	sort.Slice(points, func(first, second int) bool {
		if points[first].Y == points[second].Y {
			return points[first].Z < points[second].Z
		}
		return points[first].Y < points[second].Y
	})
	orient := func(a, b, c math3d.Vec3) float64 {
		return (b.Y-a.Y)*(c.Z-a.Z) - (b.Z-a.Z)*(c.Y-a.Y)
	}
	lower := make([]math3d.Vec3, 0, len(points))
	for _, point := range points {
		for len(lower) >= 2 && orient(lower[len(lower)-2], lower[len(lower)-1], point) <= 1e-9 {
			lower = lower[:len(lower)-1]
		}
		lower = append(lower, point)
	}
	upper := make([]math3d.Vec3, 0, len(points))
	for index := len(points) - 1; index >= 0; index-- {
		point := points[index]
		for len(upper) >= 2 && orient(upper[len(upper)-2], upper[len(upper)-1], point) <= 1e-9 {
			upper = upper[:len(upper)-1]
		}
		upper = append(upper, point)
	}
	hull := append(lower[:len(lower)-1], upper[:len(upper)-1]...)
	for index := range hull {
		hull[index].X = plane
	}
	return hull
}

func appendUniqueSectionPoint(points *[]math3d.Vec3, point math3d.Vec3) {
	for _, existing := range *points {
		if math.Abs(existing.Y-point.Y) <= 1e-8 && math.Abs(existing.Z-point.Z) <= 1e-8 {
			return
		}
	}
	*points = append(*points, point)
}
