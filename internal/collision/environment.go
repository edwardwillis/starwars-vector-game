package collision

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

type FeatureID string

type Hit struct {
	Time      float64
	Point     math3d.Vec3
	Normal    math3d.Vec3
	FeatureID FeatureID
}

type FinitePlane struct {
	Center    math3d.Vec3
	Normal    math3d.Vec3
	AxisU     math3d.Vec3
	HalfU     float64
	HalfV     float64
	FeatureID FeatureID
}

// SweepSpherePlane detects a moving sphere against the front of a finite plane.
func SweepSpherePlane(start, end math3d.Vec3, radius float64, plane FinitePlane) (Hit, bool) {
	normal := plane.Normal.Normalize()
	axisU := plane.AxisU.Normalize()
	axisV := normal.Cross(axisU).Normalize()
	if radius < 0 || normal == (math3d.Vec3{}) || axisU == (math3d.Vec3{}) || axisV == (math3d.Vec3{}) {
		return Hit{}, false
	}
	d0 := start.Sub(plane.Center).Dot(normal)
	d1 := end.Sub(plane.Center).Dot(normal)
	if d0 < radius || d1 > radius || d0 == d1 {
		return Hit{}, false
	}
	t := (d0 - radius) / (d0 - d1)
	if t < 0 || t > 1 {
		return Hit{}, false
	}
	center := start.Add(end.Sub(start).Scale(t))
	contact := center.Sub(normal.Scale(radius))
	offset := contact.Sub(plane.Center)
	if math.Abs(offset.Dot(axisU)) > plane.HalfU || math.Abs(offset.Dot(axisV)) > plane.HalfV {
		return Hit{}, false
	}
	return Hit{Time: t, Point: contact, Normal: normal, FeatureID: plane.FeatureID}, true
}

type OrientedBox struct {
	Center      math3d.Vec3
	Orientation math3d.Quaternion
	HalfExtents math3d.Vec3
	FeatureID   FeatureID
}

// SweepSphereBox performs a slab query against a box expanded by sphere radius.
func SweepSphereBox(start, end math3d.Vec3, radius float64, box OrientedBox) (Hit, bool) {
	if radius < 0 || box.HalfExtents.X < 0 || box.HalfExtents.Y < 0 || box.HalfExtents.Z < 0 {
		return Hit{}, false
	}
	orientation := box.Orientation.Normalize()
	inverse := orientation.Conjugate()
	localStart := inverse.Rotate(start.Sub(box.Center))
	localEnd := inverse.Rotate(end.Sub(box.Center))
	direction := localEnd.Sub(localStart)
	extent := box.HalfExtents.Add(math3d.Vec3{X: radius, Y: radius, Z: radius})
	tEnter, tExit := 0.0, 1.0
	axis, sign := -1, 0.0
	starts := [3]float64{localStart.X, localStart.Y, localStart.Z}
	deltas := [3]float64{direction.X, direction.Y, direction.Z}
	extents := [3]float64{extent.X, extent.Y, extent.Z}
	for index := range 3 {
		if math.Abs(deltas[index]) < 1e-12 {
			if starts[index] < -extents[index] || starts[index] > extents[index] {
				return Hit{}, false
			}
			continue
		}
		near := (-extents[index] - starts[index]) / deltas[index]
		far := (extents[index] - starts[index]) / deltas[index]
		nearSign := -1.0
		if near > far {
			near, far, nearSign = far, near, 1
		}
		if near > tEnter {
			tEnter, axis, sign = near, index, nearSign
		}
		tExit = math.Min(tExit, far)
		if tEnter > tExit {
			return Hit{}, false
		}
	}
	if tEnter < 0 || tEnter > 1 {
		return Hit{}, false
	}
	localNormal := math3d.Vec3{}
	switch axis {
	case 0:
		localNormal.X = sign
	case 1:
		localNormal.Y = sign
	case 2:
		localNormal.Z = sign
	default:
		return Hit{}, false
	}
	normal := orientation.Rotate(localNormal).Normalize()
	center := start.Add(end.Sub(start).Scale(tEnter))
	return Hit{Time: tEnter, Point: center.Sub(normal.Scale(radius)), Normal: normal, FeatureID: box.FeatureID}, true
}

func Earliest(hits ...Hit) (Hit, bool) {
	best := Hit{Time: math.Inf(1)}
	for _, hit := range hits {
		if hit.Time >= 0 && hit.Time <= 1 && hit.Time < best.Time {
			best = hit
		}
	}
	return best, !math.IsInf(best.Time, 1)
}
