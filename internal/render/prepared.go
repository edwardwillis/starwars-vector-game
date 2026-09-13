package render

import (
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

// PreparedTriangle is a camera-clipped, projected physical face triangle.
// Depth rasterization and sparse background occlusion consume the same data.
type PreparedTriangle struct {
	Face    int
	A, B, C Point
}

// PreparedGeometry owns the camera-relative results shared by every visual
// pass for one model instance in one frame. Model topology remains immutable;
// these slices are reusable frame scratch and must not outlive the frame.
type PreparedGeometry struct {
	Mesh        model.Model
	Vertices    []math3d.Vec3
	Faces       []model.Face
	FaceFront   []bool
	Edges       []model.Edge
	Triangles   []PreparedTriangle
	edgeScratch []model.Edge
}

// PrepareGeometry transforms and classifies a model instance once. Setting
// prepareSurfaces builds clipped/projected triangles only when depth or point
// occlusion will consume them.
func (p Pipeline) PrepareGeometry(mesh model.Model, world math3d.Mat4, prepareSurfaces bool) PreparedGeometry {
	var result PreparedGeometry
	p.PrepareGeometryInto(&result, mesh, world, prepareSurfaces)
	return result
}

// PrepareGeometryInto reuses the destination's scratch storage across frames.
func (p Pipeline) PrepareGeometryInto(result *PreparedGeometry, mesh model.Model, world math3d.Mat4, prepareSurfaces bool) {
	if result == nil {
		return
	}
	prepared := model.Prepare(mesh)
	result.Mesh = prepared
	result.Triangles = result.Triangles[:0]
	result.Vertices = resizeVec3(result.Vertices, len(prepared.Verts))
	result.Faces = resizeFaces(result.Faces, len(prepared.Faces))
	result.FaceFront = resizeBools(result.FaceFront, len(prepared.Faces))
	viewWorld := p.View.Mul(world)
	for index, vertex := range prepared.Verts {
		result.Vertices[index] = viewWorld.TransformPoint(vertex)
	}
	copy(result.Faces, prepared.Faces)
	for index := range result.Faces {
		result.Faces[index].Normal = viewWorld.TransformNormal(prepared.Faces[index].Normal).Normalize()
	}
	classifyFaceVisibility(result.Vertices, result.Faces, result.FaceFront)

	edges := prepared.Edges
	if p.Culler != nil {
		edges = p.Culler.Cull(result.Vertices, edges)
	}
	result.Edges = result.Edges[:0]
	stageMesh := prepared
	stageMesh.Faces = result.Faces
	for _, stage := range p.Stages {
		if stage == nil {
			continue
		}
		before := len(edges)
		switch stage.Name() {
		case "backface-culling":
			result.Edges = backfaceCullClassified(prepared, result.FaceFront, edges, result.Edges[:0])
			edges = result.Edges
		case "hidden-line-removal":
			out := result.edgeScratch[:0]
			for _, edge := range edges {
				if edge.Kind != model.EdgeInternal {
					out = append(out, edge)
				}
			}
			result.edgeScratch = out
			edges = out
		case "depth-cue":
			// Depth cue currently changes policy only; it has no topology work.
		default:
			edges = stage.Process(result.Vertices, stageMesh, edges)
		}
		if p.Stats != nil && before > len(edges) {
			if stage.Name() == "backface-culling" {
				p.Stats.BackfaceRejected += before - len(edges)
			} else {
				p.Stats.PolicyRejected += before - len(edges)
			}
		}
	}
	result.Edges = append(result.Edges[:0], edges...)

	if prepareSurfaces {
		p.prepareSurfaceTriangles(result)
	}
	if p.Stats != nil {
		p.Stats.GeometryPreparations++
		p.Stats.InputVertices += len(prepared.Verts)
		p.Stats.TransformedVertices += len(prepared.Verts)
		p.Stats.InputFaces += len(prepared.Faces)
		p.Stats.FacesClassified += len(prepared.Faces)
		p.Stats.InputEdges += len(prepared.Edges)
		p.Stats.PreparedTriangles += len(result.Triangles)
	}
}

func classifyFaceVisibility(vertices []math3d.Vec3, faces []model.Face, front []bool) {
	for faceIndex, face := range faces {
		front[faceIndex] = false
		if len(face.Vertices) < 3 {
			continue
		}
		center := math3d.Vec3{}
		valid := true
		for _, vertex := range face.Vertices {
			if vertex < 0 || vertex >= len(vertices) {
				valid = false
				break
			}
			center = center.Add(vertices[vertex])
		}
		if !valid {
			continue
		}
		center = center.Scale(1 / float64(len(face.Vertices)))
		front[faceIndex] = face.DoubleSided || face.Normal.Dot(center.Scale(-1)) > 1e-9
	}
}

func backfaceCullClassified(mesh model.Model, front []bool, edges []model.Edge, out []model.Edge) []model.Edge {
	if len(mesh.Faces) == 0 {
		return append(out, edges...)
	}
	for _, edge := range edges {
		if edge.Kind == model.EdgeInternal {
			continue
		}
		adjacent := edge.AdjacentFaces
		if len(adjacent) == 0 && edge.FaceA < 0 && edge.FaceB < 0 {
			out = append(out, edge)
			continue
		}
		visible, renderable := false, false
		for _, faceIndex := range adjacent {
			if faceIndex < 0 || faceIndex >= len(front) || !front[faceIndex] {
				continue
			}
			visible = true
			if !mesh.Faces[faceIndex].OccluderOnly {
				renderable = true
			}
		}
		if visible && !renderable {
			continue
		}
		if len(adjacent) == 0 {
			visible = edge.FaceA >= 0 && edge.FaceA < len(front) && front[edge.FaceA] ||
				edge.FaceB >= 0 && edge.FaceB < len(front) && front[edge.FaceB]
		}
		if visible {
			out = append(out, edge)
		}
	}
	return out
}

func (p Pipeline) prepareSurfaceTriangles(geometry *PreparedGeometry) {
	topology := geometry.Mesh.Topology
	for faceIndex, face := range geometry.Faces {
		if faceIndex >= len(geometry.FaceFront) || !geometry.FaceFront[faceIndex] || len(face.Vertices) < 3 {
			continue
		}
		inside := true
		for _, index := range face.Vertices {
			if index < 0 || index >= len(geometry.Vertices) {
				inside = false
				break
			}
			vertex := geometry.Vertices[index]
			if vertex.Z > -p.Near || p.Far > p.Near && vertex.Z < -p.Far {
				inside = false
			}
		}
		if inside && topology != nil && faceIndex+1 < len(topology.FaceTriangleOffsets) {
			start, end := topology.FaceTriangleOffsets[faceIndex], topology.FaceTriangleOffsets[faceIndex+1]
			for _, triangle := range topology.FaceTriangles[start:end] {
				p.appendPreparedTriangle(geometry, faceIndex, geometry.Vertices[triangle.A], geometry.Vertices[triangle.B], geometry.Vertices[triangle.C])
			}
			continue
		}
		polygon := make([]math3d.Vec3, 0, len(face.Vertices)+2)
		for _, index := range face.Vertices {
			if index < 0 || index >= len(geometry.Vertices) {
				polygon = nil
				break
			}
			polygon = append(polygon, geometry.Vertices[index])
		}
		polygon = clipPolygonZ(polygon, -p.Near, true)
		if p.Far > p.Near {
			polygon = clipPolygonZ(polygon, -p.Far, false)
		}
		for index := 1; index+1 < len(polygon); index++ {
			p.appendPreparedTriangle(geometry, faceIndex, polygon[0], polygon[index], polygon[index+1])
		}
	}
}

func (p Pipeline) appendPreparedTriangle(geometry *PreparedGeometry, face int, a, b, c math3d.Vec3) {
	project := func(vertex math3d.Vec3) Point {
		projected := p.Projection.TransformPoint(vertex)
		return Point{
			X: (projected.X + 1) * 0.5 * float64(p.Width), Y: (1 - projected.Y) * 0.5 * float64(p.Height), Depth: -vertex.Z,
		}
	}
	geometry.Triangles = append(geometry.Triangles, PreparedTriangle{Face: face, A: project(a), B: project(b), C: project(c)})
}

func resizeVec3(values []math3d.Vec3, length int) []math3d.Vec3 {
	if cap(values) < length {
		return make([]math3d.Vec3, length)
	}
	return values[:length]
}

func resizeFaces(values []model.Face, length int) []model.Face {
	if cap(values) < length {
		return make([]model.Face, length)
	}
	return values[:length]
}

func resizeBools(values []bool, length int) []bool {
	if cap(values) < length {
		return make([]bool, length)
	}
	return values[:length]
}
