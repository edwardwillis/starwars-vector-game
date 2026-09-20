package render

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

// ClipLineToConvexPolygon confines a projected line to a portal aperture.
// The boundary may have either winding; no framebuffer mask is required.
func ClipLineToConvexPolygon(line Line, boundary []Point) (Line, bool) {
	if len(boundary) < 3 {
		return Line{}, false
	}
	sign := polygonWinding(boundary)
	if sign == 0 {
		return Line{}, false
	}
	start, end := 0.0, 1.0
	for index, a := range boundary {
		b := boundary[(index+1)%len(boundary)]
		dx, dy := b.X-a.X, b.Y-a.Y
		value := sign * (dx*(line.Y1-a.Y) - dy*(line.X1-a.X))
		delta := sign * (dx*(line.Y2-line.Y1) - dy*(line.X2-line.X1))
		if math.Abs(delta) < 1e-12 {
			if value < -1e-8 {
				return Line{}, false
			}
			continue
		}
		limit := -value / delta
		if delta > 0 {
			start = max(start, limit)
		} else {
			end = min(end, limit)
		}
		if start > end {
			return Line{}, false
		}
	}
	return Line{
		X1: line.X1 + (line.X2-line.X1)*start,
		Y1: line.Y1 + (line.Y2-line.Y1)*start,
		X2: line.X1 + (line.X2-line.X1)*end,
		Y2: line.Y1 + (line.Y2-line.Y1)*end,
	}, true
}

func polygonWinding(boundary []Point) float64 {
	area := 0.0
	for index, a := range boundary {
		b := boundary[(index+1)%len(boundary)]
		area += a.X*b.Y - b.X*a.Y
	}
	if math.Abs(area) < 1e-8 {
		return 0
	}
	return math.Copysign(1, area)
}

type portalVertex struct {
	point Point
	uv    model.UV
}

// ClipPreparedTrianglesToConvex clips projected physical faces before they
// reach depth rasterization, point occlusion, or filled-surface submission.
// Depth and UV interpolation are perspective-correct at new intersections.
func ClipPreparedTrianglesToConvex(geometry *PreparedGeometry, boundary []Point) {
	if geometry == nil || len(boundary) < 3 || polygonWinding(boundary) == 0 {
		return
	}
	sign := polygonWinding(boundary)
	destination := geometry.clipScratch[:0]
	var inputFixed, outputFixed [16]portalVertex
	input, output := inputFixed[:], outputFixed[:]
	if len(boundary)+3 > len(inputFixed) {
		input = make([]portalVertex, len(boundary)+3)
		output = make([]portalVertex, len(boundary)+3)
	}
	for _, triangle := range geometry.Triangles {
		input[0] = portalVertex{triangle.A, triangle.UVs[0]}
		input[1] = portalVertex{triangle.B, triangle.UVs[1]}
		input[2] = portalVertex{triangle.C, triangle.UVs[2]}
		count := 3
		for index, a := range boundary {
			b := boundary[(index+1)%len(boundary)]
			dx, dy := b.X-a.X, b.Y-a.Y
			outCount := 0
			previous := input[count-1]
			previousSide := sign * (dx*(previous.point.Y-a.Y) - dy*(previous.point.X-a.X))
			for next := 0; next < count; next++ {
				current := input[next]
				currentSide := sign * (dx*(current.point.Y-a.Y) - dy*(current.point.X-a.X))
				if (previousSide < 0) != (currentSide < 0) {
					fraction := previousSide / (previousSide - currentSide)
					output[outCount] = interpolatePortalVertex(previous, current, fraction)
					outCount++
				}
				if currentSide >= 0 {
					output[outCount] = current
					outCount++
				}
				previous, previousSide = current, currentSide
			}
			if outCount == 0 {
				count = 0
				break
			}
			input, output = output, input
			count = outCount
		}
		for index := 1; index+1 < count; index++ {
			destination = append(destination, PreparedTriangle{
				Face: triangle.Face,
				A:    input[0].point, B: input[index].point, C: input[index+1].point,
				UVs: [3]model.UV{input[0].uv, input[index].uv, input[index+1].uv},
			})
		}
	}
	geometry.Triangles, geometry.clipScratch = destination, geometry.Triangles[:0]
}

func interpolatePortalVertex(a, b portalVertex, fraction float64) portalVertex {
	point := Point{X: a.point.X + (b.point.X-a.point.X)*fraction, Y: a.point.Y + (b.point.Y-a.point.Y)*fraction}
	inverseA, inverseB := 1/a.point.Depth, 1/b.point.Depth
	inverseDepth := inverseA + (inverseB-inverseA)*fraction
	point.Depth = 1 / inverseDepth
	weightA := inverseA * (1 - fraction) / inverseDepth
	weightB := inverseB * fraction / inverseDepth
	return portalVertex{point: point, uv: model.UV{U: a.uv.U*weightA + b.uv.U*weightB, V: a.uv.V*weightA + b.uv.V*weightB}}
}
