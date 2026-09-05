// Package model defines indexed wireframe geometry.
package model

import (
	"fmt"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// Edge joins two vertices by index.
type Edge struct {
	A int
	B int

	// FaceA and FaceB are derived by Prepare. A value of -1 means the edge is
	// not attached to a surface face (decorative line art).
	FaceA int
	FaceB int
	// AdjacentFaces retains every incident face, including non-manifold third
	// faces. FaceA and FaceB are convenient fast-path slots for the common case.
	AdjacentFaces []int
	Kind  EdgeKind
	// Importance allows rendering profiles to retain the most useful edges
	// when a model is reduced for projected-size LOD.
	Importance float32
}

type EdgeKind uint8

const (
	EdgeStructural EdgeKind = iota
	EdgeDecorative
	EdgeInternal
)

// Face records an ordered polygon boundary. Its cached normal and plane support
// visibility/depth processing while the boundary remains available for
// topology-aware fracture and edge classification.
type Face struct {
	Vertices []int
	Normal   math3d.Vec3
	PlaneD   float64
	DoubleSided bool
}

// Topology is immutable derived data shared by model instances. It is built
// once from the authored vertices, faces and edges and reused by visibility
// stages.
type Topology struct {
	Edges        []Edge
	FaceNormals  []math3d.Vec3
	FacePlaneD   []float64
	BoundsCenter math3d.Vec3
	BoundsRadius float64
}

// Model is a wireframe mesh made from vertices and the edges between them.
type Model struct {
	Verts []math3d.Vec3
	Edges []Edge
	Faces []Face
	Topology *Topology
}

// Prepare compiles face normals, edge adjacency and a conservative local
// bounding sphere. The returned model owns immutable derived topology; source
// slices remain untouched.
func Prepare(source Model) Model {
	if source.Topology != nil {
		return source
	}
	result := source
	result.Edges = append([]Edge(nil), source.Edges...)
	result.Faces = make([]Face, len(source.Faces))
	for index, face := range source.Faces {
		result.Faces[index] = Face{Vertices: append([]int(nil), face.Vertices...), DoubleSided: face.DoubleSided}
	}
	result.Topology = compileTopology(result.Verts, result.Edges, result.Faces)
	for index := range result.Faces {
		if index < len(result.Topology.FaceNormals) {
			result.Faces[index].Normal = result.Topology.FaceNormals[index]
			result.Faces[index].PlaneD = result.Topology.FacePlaneD[index]
		}
	}
	result.Edges = append([]Edge(nil), result.Topology.Edges...)
	return result
}

// OrientOutward repairs winding for a convex or near-convex generated
// component using its local vertex centroid. It is intended for procedural
// solids such as hull sections, nacelles, panels, and cockpit canopies; it is
// not a substitute for authored winding on concave assemblies.
func OrientOutward(source Model) Model {
	if len(source.Faces) == 0 || len(source.Verts) == 0 { return Prepare(source) }
	center := math3d.Vec3{}
	for _, vertex := range source.Verts { center = center.Add(vertex) }
	center = center.Scale(1 / float64(len(source.Verts)))
	result := source
	result.Topology = nil
	result.Faces = make([]Face, len(source.Faces))
	for index, face := range source.Faces {
		vertices := append([]int(nil), face.Vertices...)
		normal, _ := faceNormal(source.Verts, vertices)
		faceCenter := math3d.Vec3{}
		for _, vertex := range vertices { faceCenter = faceCenter.Add(source.Verts[vertex]) }
		faceCenter = faceCenter.Scale(1 / float64(len(vertices)))
		if normal.Dot(faceCenter.Sub(center)) < 0 { reverseFaceIndices(vertices) }
		result.Faces[index] = Face{Vertices: vertices, DoubleSided: face.DoubleSided}
	}
	return Prepare(result)
}

func reverseFaceIndices(indices []int) {
	for left, right := 0, len(indices)-1; left < right; left, right = left+1, right-1 { indices[left], indices[right] = indices[right], indices[left] }
}

func compileTopology(verts []math3d.Vec3, authored []Edge, faces []Face) *Topology {
	topology := &Topology{FaceNormals: make([]math3d.Vec3, len(faces)), FacePlaneD: make([]float64, len(faces))}
	if len(verts) > 0 {
		min, max := verts[0], verts[0]
		for _, vertex := range verts[1:] {
			min.X, min.Y, min.Z = mathMin(min.X, vertex.X), mathMin(min.Y, vertex.Y), mathMin(min.Z, vertex.Z)
			max.X, max.Y, max.Z = mathMax(max.X, vertex.X), mathMax(max.Y, vertex.Y), mathMax(max.Z, vertex.Z)
		}
		topology.BoundsCenter = min.Add(max).Scale(.5)
		for _, vertex := range verts {
			distance := vertex.Sub(topology.BoundsCenter).Length()
			if distance > topology.BoundsRadius { topology.BoundsRadius = distance }
		}
	}
	keys := make(map[edgeKey]int, len(authored)+len(faces)*3)
	for index, edge := range authored {
		edge.FaceA, edge.FaceB, edge.AdjacentFaces = -1, -1, nil
		if edge.A > edge.B { edge.A, edge.B = edge.B, edge.A }
		keys[edgeKey{edge.A, edge.B}] = index
		topology.Edges = append(topology.Edges, edge)
	}
	for faceIndex, face := range faces {
		normal, d := faceNormal(verts, face.Vertices)
		topology.FaceNormals[faceIndex], topology.FacePlaneD[faceIndex] = normal, d
		for i, a := range face.Vertices {
			b := face.Vertices[(i+1)%len(face.Vertices)]
			key := edgeKey{a, b}; if key.A > key.B { key.A, key.B = key.B, key.A }
			edgeIndex, exists := keys[key]
			if !exists {
				edgeIndex = len(topology.Edges); keys[key] = edgeIndex
				topology.Edges = append(topology.Edges, Edge{A: key.A, B: key.B, FaceA: -1, FaceB: -1, Importance: 1})
			}
			edge := topology.Edges[edgeIndex]
			if edge.FaceA < 0 { edge.FaceA = faceIndex } else if edge.FaceB < 0 { edge.FaceB = faceIndex }
			edge.AdjacentFaces = append(edge.AdjacentFaces, faceIndex)
			if edge.Kind == EdgeStructural && edge.FaceA >= 0 && edge.FaceB >= 0 {
				dot := topology.FaceNormals[edge.FaceA].Dot(topology.FaceNormals[edge.FaceB])
				if dot > 0.999 { edge.Kind = EdgeInternal }
			}
			topology.Edges[edgeIndex] = edge
		}
	}
	return topology
}

type edgeKey struct{ A, B int }

func faceNormal(verts []math3d.Vec3, indices []int) (math3d.Vec3, float64) {
	if len(indices) < 3 { return math3d.Vec3{}, 0 }
	// Newell's method remains stable for convex and mildly non-planar polygons.
	n := math3d.Vec3{}
	for i, current := range indices {
		next := indices[(i+1)%len(indices)]
		if current < 0 || current >= len(verts) || next < 0 || next >= len(verts) { return math3d.Vec3{}, 0 }
		a, b := verts[current], verts[next]
		n.X += (a.Y-b.Y)*(a.Z+b.Z); n.Y += (a.Z-b.Z)*(a.X+b.X); n.Z += (a.X-b.X)*(a.Y+b.Y)
	}
	length := n.Length(); if length <= 1e-12 { return math3d.Vec3{}, 0 }
	n = n.Scale(1 / length)
	return n, -n.Dot(verts[indices[0]])
}

func mathMin(a, b float64) float64 { if a < b { return a }; return b }
func mathMax(a, b float64) float64 { if a > b { return a }; return b }

// Validate reports malformed edge indices before a model reaches the renderer.
func (m Model) Validate() error {
	for index, vertex := range m.Verts {
		if math.IsNaN(vertex.X) || math.IsInf(vertex.X, 0) || math.IsNaN(vertex.Y) || math.IsInf(vertex.Y, 0) || math.IsNaN(vertex.Z) || math.IsInf(vertex.Z, 0) {
			return fmt.Errorf("vertex %d contains a non-finite coordinate", index)
		}
	}
	for index, edge := range m.Edges {
		if edge.A < 0 || edge.A >= len(m.Verts) {
			return fmt.Errorf("edge %d: vertex A index %d out of range", index, edge.A)
		}
		if edge.B < 0 || edge.B >= len(m.Verts) {
			return fmt.Errorf("edge %d: vertex B index %d out of range", index, edge.B)
		}
		if edge.A == edge.B {
			return fmt.Errorf("edge %d: endpoints must differ", index)
		}
	}
	for faceIndex, face := range m.Faces {
		if len(face.Vertices) < 3 {
			return fmt.Errorf("face %d: has %d vertices, want at least 3", faceIndex, len(face.Vertices))
		}
		for _, vertex := range face.Vertices {
			if vertex < 0 || vertex >= len(m.Verts) {
				return fmt.Errorf("face %d: vertex index %d out of range", faceIndex, vertex)
			}
		}
		unique := make(map[int]bool, len(face.Vertices))
		for _, vertex := range face.Vertices { if unique[vertex] { return fmt.Errorf("face %d: repeated vertex %d", faceIndex, vertex) }; unique[vertex] = true }
		if normal, _ := faceNormal(m.Verts, face.Vertices); normal.Length() <= 1e-9 { return fmt.Errorf("face %d: degenerate area", faceIndex) }
	}
	return nil
}

// PolygonModels returns one closed wireframe model for every explicit face.
func (m Model) PolygonModels() []Model {
	polygons := make([]Model, 0, len(m.Faces))
	for _, face := range m.Faces {
		polygon := Model{Verts: make([]math3d.Vec3, len(face.Vertices))}
		for index, vertex := range face.Vertices {
			polygon.Verts[index] = m.Verts[vertex]
			polygon.Edges = append(polygon.Edges, Edge{A: index, B: (index + 1) % len(face.Vertices), Kind: EdgeStructural, Importance: 1})
		}
		polygon.Faces = []Face{{Vertices: polygonVertexIndices(len(polygon.Verts))}}
		polygons = append(polygons, Prepare(polygon))
	}
	return polygons
}

func polygonVertexIndices(count int) []int {
	indices := make([]int, count)
	for index := range indices {
		indices[index] = index
	}
	return indices
}

// Transform returns a copy of a model transformed into another local frame.
func Transform(source Model, transform math3d.Mat4) Model {
	result := Model{
		Verts: make([]math3d.Vec3, len(source.Verts)),
		Edges: append([]Edge(nil), source.Edges...),
		Faces: make([]Face, len(source.Faces)),
	}
	for index, vertex := range source.Verts {
		result.Verts[index] = transform.TransformPoint(vertex)
	}
	for index, face := range source.Faces {
		result.Faces[index] = Face{Vertices: append([]int(nil), face.Vertices...), DoubleSided: face.DoubleSided}
	}
	result.Topology = nil
	result = Prepare(result)
	return result
}

// Merge combines models into one index space without sharing mutable slices.
func Merge(models ...Model) Model {
	var result Model
	for _, source := range models {
		base := len(result.Verts)
		result.Verts = append(result.Verts, source.Verts...)
		for _, edge := range source.Edges {
			result.Edges = append(result.Edges, Edge{A: base + edge.A, B: base + edge.B, Kind: edge.Kind, Importance: edge.Importance})
		}
		for _, face := range source.Faces {
			vertices := make([]int, len(face.Vertices))
			for index, vertex := range face.Vertices {
				vertices[index] = base + vertex
			}
			result.Faces = append(result.Faces, Face{Vertices: vertices, DoubleSided: face.DoubleSided})
		}
	}
	return Prepare(result)
}
