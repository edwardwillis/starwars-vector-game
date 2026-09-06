package render

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

// OccluderKind identifies the deliberately small set of point-occlusion
// mechanisms used by sparse background fields.
type OccluderKind uint8

const (
	OccluderAnalytic OccluderKind = iota
	OccluderGeometry
)

// PointOccluder answers whether a projected background point lies behind a
// visible surface. It is intentionally independent of scene object types.
type PointOccluder interface {
	OccluderKind() OccluderKind
	OccludesPoint(Point) bool
}

// PointOccluderSet combines the active screen-space occluders for one frame.
// It is a value type so callers can reuse its backing slice without per-frame
// allocations in the starfield path.
type PointOccluderSet struct {
	Items []PointOccluder
	Stats *Stats
}

func (set *PointOccluderSet) Reset() {
	if set != nil {
		set.Items = set.Items[:0]
	}
}

func (set *PointOccluderSet) Add(occluder PointOccluder) {
	if set != nil && occluder != nil {
		set.Items = append(set.Items, occluder)
	}
}

func (set *PointOccluderSet) OccluderKind() OccluderKind { return OccluderGeometry }

func (set *PointOccluderSet) OccludesPoint(point Point) bool {
	if set == nil {
		return false
	}
	for _, occluder := range set.Items {
		if occluder == nil || !occluder.OccludesPoint(point) {
			continue
		}
		if set.Stats != nil {
			switch occluder.OccluderKind() {
			case OccluderAnalytic:
				set.Stats.StarsAnalyticRejected++
			case OccluderGeometry:
				set.Stats.StarsGeometryRejected++
			}
		}
		return true
	}
	return false
}

// CircleOccluder is the projected silhouette of a spherical opaque volume.
// Radius is in screen pixels and Depth is the camera-space depth of the
// sphere centre. SphereRadius allows a more accurate front-surface depth test
// than treating the whole projected circle as a flat disk.
type CircleOccluder struct {
	CenterX, CenterY float64
	Radius           float64
	Depth            float64
	SphereRadius     float64
}

func (CircleOccluder) OccluderKind() OccluderKind { return OccluderAnalytic }

func (occluder CircleOccluder) OccludesPoint(point Point) bool {
	if occluder.Radius <= 0 || occluder.Depth <= 0 {
		return false
	}
	dx, dy := point.X-occluder.CenterX, point.Y-occluder.CenterY
	distanceSquared := dx*dx + dy*dy
	if distanceSquared > occluder.Radius*occluder.Radius {
		return false
	}
	// Approximate the front hemisphere depth at this screen position. The
	// centre-depth fallback remains useful for callers that only have a
	// projected circle and no world-space radius.
	surfaceDepth := occluder.Depth
	if occluder.SphereRadius > 0 {
		ratio := math.Sqrt(math.Max(0, 1-distanceSquared/(occluder.Radius*occluder.Radius)))
		surfaceDepth = occluder.Depth - occluder.SphereRadius*ratio
	}
	return point.Depth > surfaceDepth+1e-6
}

// TriangleOccluder is one projected, camera-facing solid face. Depth is
// interpolated perspective-correctly, matching the depth rasterizer.
type TriangleOccluder struct {
	A, B, C Point
}

func (TriangleOccluder) OccluderKind() OccluderKind { return OccluderGeometry }

func (triangle TriangleOccluder) OccludesPoint(point Point) bool {
	denominator := (triangle.B.Y-triangle.C.Y)*(triangle.A.X-triangle.C.X) +
		(triangle.C.X-triangle.B.X)*(triangle.A.Y-triangle.C.Y)
	if math.Abs(denominator) <= 1e-12 {
		return false
	}
	u := ((triangle.B.Y-triangle.C.Y)*(point.X-triangle.C.X) +
		(triangle.C.X-triangle.B.X)*(point.Y-triangle.C.Y)) / denominator
	v := ((triangle.C.Y-triangle.A.Y)*(point.X-triangle.C.X) +
		(triangle.A.X-triangle.C.X)*(point.Y-triangle.C.Y)) / denominator
	w := 1 - u - v
	const epsilon = 1e-9
	if u < -epsilon || v < -epsilon || w < -epsilon {
		return false
	}
	interpolatedInverseDepth := u/(triangle.A.Depth) + v/(triangle.B.Depth) + w/(triangle.C.Depth)
	if interpolatedInverseDepth <= 0 {
		return false
	}
	surfaceDepth := 1 / interpolatedInverseDepth
	return point.Depth > surfaceDepth+1e-6
}

// ProjectSolidOccluders reuses the model's authored faces and the renderer's
// camera/frustum conventions to produce sparse point-test geometry. Faces are
// back-face culled before they become occluders, then clipped against the
// near/far planes and triangulated only in screen space.
func (p Pipeline) ProjectSolidOccluders(mesh model.Model, world math3d.Mat4) []TriangleOccluder {
	if p.Width <= 0 || p.Height <= 0 || p.Near <= 0 || len(mesh.Faces) == 0 {
		return nil
	}
	prepared := model.Prepare(mesh)
	viewWorld := p.View.Mul(world)
	occluders := make([]TriangleOccluder, 0, len(prepared.Faces)*2)
	for _, face := range prepared.Faces {
		if len(face.Vertices) < 3 || face.DoubleSided == false && face.Normal.Length() <= 1e-9 {
			continue
		}
		polygon := make([]math3d.Vec3, 0, len(face.Vertices))
		for _, index := range face.Vertices {
			if index < 0 || index >= len(prepared.Verts) {
				polygon = nil
				break
			}
			polygon = append(polygon, viewWorld.TransformPoint(prepared.Verts[index]))
		}
		if len(polygon) < 3 {
			continue
		}
		if !face.DoubleSided {
			normal := viewWorld.TransformDirection(face.Normal).Normalize()
			center := math3d.Vec3{}
			for _, vertex := range polygon {
				center = center.Add(vertex)
			}
			center = center.Scale(1 / float64(len(polygon)))
			if normal.Dot(center.Scale(-1)) <= 1e-9 {
				continue
			}
		}
		polygon = clipPolygonZ(polygon, -p.Near, true)
		if p.Far > p.Near {
			polygon = clipPolygonZ(polygon, -p.Far, false)
		}
		for index := 1; index+1 < len(polygon); index++ {
			vertices := [3]Point{}
			valid := true
			for vertexIndex, vertex := range [3]math3d.Vec3{polygon[0], polygon[index], polygon[index+1]} {
				if vertex.Z >= -p.Near || (p.Far > p.Near && vertex.Z <= -p.Far) {
					valid = false
					break
				}
				projected := p.Projection.TransformPoint(vertex)
				vertices[vertexIndex] = Point{
					X:     (projected.X + 1) * 0.5 * float64(p.Width),
					Y:     (1 - projected.Y) * 0.5 * float64(p.Height),
					Depth: -vertex.Z,
				}
			}
			if valid {
				occluders = append(occluders, TriangleOccluder{A: vertices[0], B: vertices[1], C: vertices[2]})
			}
		}
	}
	return occluders
}
