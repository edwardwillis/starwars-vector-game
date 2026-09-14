package render

import (
	"image/color"
	"sort"
)

// FlatTriangle is a screen-space opaque primitive derived from already
// transformed, back-face-classified, and near/far-clipped model geometry.
type FlatTriangle struct {
	A, B, C Point
	Color   color.RGBA
}

func (triangle FlatTriangle) averageDepth() float64 {
	return (triangle.A.Depth + triangle.B.Depth + triangle.C.Depth) / 3
}

// AppendFlatTriangles reuses prepared surface results; it performs no model
// traversal, transformation, face classification, or clipping.
func AppendFlatTriangles(destination []FlatTriangle, geometry *PreparedGeometry, fill color.RGBA) []FlatTriangle {
	if geometry == nil || fill.A != 255 {
		return destination
	}
	for _, triangle := range geometry.Triangles {
		if triangle.Face < 0 || triangle.Face >= len(geometry.Faces) || geometry.Faces[triangle.Face].OccluderOnly {
			continue
		}
		destination = append(destination, FlatTriangle{
			A: triangle.A, B: triangle.B, C: triangle.C, Color: fill,
		})
	}
	return destination
}

// SortFlatTriangles applies a stable painter order only to explicitly filled
// geometry. Ordinary vector-only frames pay none of this cost.
func SortFlatTriangles(triangles []FlatTriangle) {
	sort.SliceStable(triangles, func(first, second int) bool {
		return triangles[first].averageDepth() > triangles[second].averageDepth()
	})
}
