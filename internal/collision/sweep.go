// Package collision provides deterministic geometric collision queries.
package collision

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// SegmentSphere returns the first normalized time at which segment start-end
// intersects a sphere. A start inside the sphere is an immediate hit at zero.
func SegmentSphere(start, end, center math3d.Vec3, radius float64) (float64, bool) {
	if radius < 0 {
		return 0, false
	}
	segment := end.Sub(start)
	offset := start.Sub(center)
	c := offset.Dot(offset) - radius*radius
	if c <= 0 {
		return 0, true
	}
	a := segment.Dot(segment)
	if a == 0 {
		return 0, false
	}
	b := 2 * offset.Dot(segment)
	discriminant := b*b - 4*a*c
	if discriminant < 0 {
		return 0, false
	}
	t := (-b - math.Sqrt(discriminant)) / (2 * a)
	return t, t >= 0 && t <= 1
}
