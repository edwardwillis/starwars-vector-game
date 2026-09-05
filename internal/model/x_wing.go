package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// XWing returns a sparse Rebel fighter assembled from a hull, four reusable
// S-foil assemblies, engines, and wingtip cannons. +Z is the nose direction.
func XWing() Model {
	hull := xWingFuselage()
	canopy := xWingCanopy()
	wing := xWingCanonicalWing()
	parts := []Model{hull, canopy}
	for _, roll := range []float64{math.Pi * 22 / 180, math.Pi * 158 / 180, math.Pi * 202 / 180, math.Pi * 338 / 180} {
		parts = append(parts, Transform(wing, math3d.RotationZ(roll)))
	}
	return Merge(parts...)
}

// XWingWindow is a separate raised canopy/window layer for contrasting color.
func XWingWindow() Model {
	return Transform(xWingCanopy(), math3d.Translation(0, 0, 0.015))
}

func xWingFuselage() Model {
	mesh := Model{}
	sections := []struct{ z, rx, ry float64 }{{-1.45, 0.62, 0.28}, {-1.10, 0.78, 0.35}, {-0.35, 0.80, 0.36}, {0.35, 0.58, 0.27}, {1.25, 0.34, 0.18}, {2.5, 0.04, 0.025}}
	const segments = 6
	for _, section := range sections {
		for i := 0; i < segments; i++ {
			a := 2 * math.Pi * float64(i) / segments
			s, c := math.Sincos(a)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: section.rx * c, Y: section.ry * s, Z: section.z})
		}
	}
	for ring := 0; ring < len(sections); ring++ {
		base := ring * segments
		for i := 0; i < segments; i++ {
			n := (i + 1) % segments
			mesh.Edges = append(mesh.Edges, Edge{base + i, base + n})
			if ring+1 < len(sections) {
				mesh.Edges = append(mesh.Edges, Edge{base + i, base + segments + i})
				mesh.Faces = append(mesh.Faces, Face{Vertices: []int{base + i, base + n, base + segments + n, base + segments + i}})
			}
		}
	}
	for _, ring := range []int{0, len(sections) - 1} {
		center := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts, math3d.Vec3{Z: sections[ring].z})
		base := ring * segments
		for i := 0; i < segments; i++ {
			next := (i + 1) % segments
			mesh.Edges = append(mesh.Edges, Edge{A: center, B: base + i})
			vertices := []int{center, base + i, base + next}
			if ring == len(sections)-1 { vertices = []int{center, base + next, base + i} }
			mesh.Faces = append(mesh.Faces, Face{Vertices: vertices})
		}
	}
	return mesh
}

func xWingCanopy() Model {
	// Faceted canopy: shallow at the forward (+Z) edge and raised toward the
	// aft (-Z) edge, integrated into the upper fuselage rather than standing
	// vertically beside it.
	mesh := Model{Verts: []math3d.Vec3{
		{X: -.24, Y: .18, Z: 1.08}, {X: .24, Y: .18, Z: 1.08},
		{X: .24, Y: .25, Z: 1.08}, {X: -.24, Y: .25, Z: 1.08},
		{X: -.30, Y: .18, Z: .25}, {X: .30, Y: .18, Z: .25},
		{X: .27, Y: .38, Z: .25}, {X: -.27, Y: .38, Z: .25},
	}}
	for i := 0; i < 4; i++ {
		next := (i + 1) % 4
		mesh.Edges = append(mesh.Edges, Edge{A: i, B: next}, Edge{A: 4 + i, B: 4 + next}, Edge{A: i, B: 4 + i})
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{i, next, 4 + next, 4 + i}})
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: []int{0, 1, 2, 3}}, Face{Vertices: []int{7, 6, 5, 4}})
	return mesh
}

func xWingCanonicalWing() Model {
	wing := wingSlab()
	engine := Transform(xWingEngine(), math3d.Translation(1.02, .29, -.55))
	cannon := Transform(xWingCannon(), math3d.Translation(3.25, .54, .02))
	return Merge(wing, engine, cannon)
}

// wingSlab builds one canonical upper-right S-foil. Its broad Z chord runs
// from behind the cockpit toward the compact rear body; rotation around Z
// supplies the other three wings while preserving a thin physical thickness.
func wingSlab() Model {
	// The tip chord is deliberately narrower than the broad root chord; the
	// rear edge sweeps forward independently of the leading edge.
	profile := []struct {
		x, y, frontZ, rearZ float64
	}{
		{x: .02, y: .10, frontZ: -.42, rearZ: .25},
		{x: 2.65, y: .60, frontZ: -.15, rearZ: .14},
		{x: 3.25, y: .48, frontZ: -.13, rearZ: .12},
		{x: .58, y: .02, frontZ: -1.25, rearZ: -.55},
	}
	mesh := Model{}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: point.x, Y: point.y, Z: point.frontZ})
	}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: point.x, Y: point.y, Z: point.rearZ})
	}
	for i := range profile {
		next := (i + 1) % len(profile)
		mesh.Edges = append(mesh.Edges,
			Edge{A: i, B: next}, Edge{A: 4 + i, B: 4 + next}, Edge{A: i, B: 4 + i})
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{i, next, 4 + next, 4 + i}})
	}
	mesh.Faces = append(mesh.Faces,
		Face{Vertices: []int{0, 1, 2, 3}},
		Face{Vertices: []int{7, 6, 5, 4}},
	)
	return mesh
}

func xWingEngine() Model {
	return nacelle(0.27, 2.00, 8)
}

func xWingCannon() Model {
	barrel := cylinder(0.045, 1.15, 6)
	mount := prism([]math3d.Vec3{{X: -.09, Y: -.09, Z: -.08}, {X: .09, Y: -.09, Z: -.08}, {X: .09, Y: .09, Z: -.08}, {X: -.09, Y: .09, Z: -.08}}, .14)
	return Merge(Transform(barrel, math3d.Translation(0, 0, .48)), mount)
}

func cylinder(radius, length float64, segments int) Model {
	mesh := Model{}
	for _, z := range []float64{-length / 2, length / 2} {
		for i := 0; i < segments; i++ {
			a := 2 * math.Pi * float64(i) / float64(segments)
			s, c := math.Sincos(a)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: radius * c, Y: radius * s, Z: z})
		}
	}
	for i := 0; i < segments; i++ {
		n := (i + 1) % segments
		mesh.Edges = append(mesh.Edges, Edge{i, n}, Edge{i, segments + i})
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{i, n, segments + n, segments + i}})
	}
	return mesh
}

// nacelle is a fuller, closed three-section engine pod. With +Z forward, the
// aft half is narrow and the forward half is enlarged with an instantaneous
// shoulder at the midpoint, matching the chunky X-Wing engine silhouette.
func nacelle(radius, length float64, segments int) Model {
	mesh := Model{}
	// Duplicate the midpoint ring so the diameter change is a hard shoulder,
	// not a diagonal/tapered connection between unequal radii.
	radii := []float64{radius * 0.52, radius * 0.52, radius * 1.12, radius * 1.12}
	// The larger forward section begins about two-thirds along the wing chord.
	shoulderZ := length * 0.08
	for section, z := range []float64{-length / 2, shoulderZ, shoulderZ, length / 2} {
		for i := 0; i < segments; i++ {
			a := 2 * math.Pi * float64(i) / float64(segments)
			s, c := math.Sincos(a)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: radii[section] * c, Y: radii[section] * s, Z: z})
		}
	}
	for ring := 0; ring < 3; ring++ {
		base := ring * segments
		nextBase := (ring + 1) * segments
		for i := 0; i < segments; i++ {
			next := (i + 1) % segments
			mesh.Edges = append(mesh.Edges,
				Edge{A: base + i, B: base + next},
				Edge{A: nextBase + i, B: nextBase + next},
				Edge{A: base + i, B: nextBase + i},
			)
			mesh.Faces = append(mesh.Faces, Face{Vertices: []int{base + i, base + next, nextBase + next, nextBase + i}})
		}
	}
	frontCenter := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Z: -length / 2})
	rearCenter := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Z: length / 2})
	for i := 0; i < segments; i++ {
		next := (i + 1) % segments
		mesh.Edges = append(mesh.Edges,
			Edge{A: frontCenter, B: i},
			Edge{A: rearCenter, B: 3*segments + i},
		)
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{frontCenter, next, i}},
			Face{Vertices: []int{rearCenter, 3*segments + i, 3*segments + next}},
		)
	}
	return mesh
}

func prism(profile []math3d.Vec3, depth float64) Model {
	mesh := Model{}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Add(math3d.Vec3{Z: -depth / 2}))
	}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Add(math3d.Vec3{Z: depth / 2}))
	}
	n := len(profile)
	for i := 0; i < n; i++ {
		next := (i + 1) % n
		mesh.Edges = append(mesh.Edges, Edge{i, next}, Edge{n + i, n + next}, Edge{i, n + i})
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{i, next, n + next, n + i}})
	}
	front := make([]int, n)
	rear := make([]int, n)
	for i := 0; i < n; i++ {
		front[i] = i
		rear[n-1-i] = n + i
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: front}, Face{Vertices: rear})
	return mesh
}

// XWingFragments partitions the model into three spatial debris groups.
func XWingFragments() [3]Model {
	hull := XWing()
	fragments := [3]Model{{Verts: hull.Verts}, {Verts: hull.Verts}, {Verts: hull.Verts}}
	for _, edge := range hull.Edges {
		mid := (hull.Verts[edge.A].X + hull.Verts[edge.B].X) / 2
		index := 1
		if mid < -1 {
			index = 0
		} else if mid > 1 {
			index = 2
		}
		fragments[index].Edges = append(fragments[index].Edges, edge)
	}
	for _, face := range hull.Faces {
		centroid := 0.0
		for _, vertex := range face.Vertices { centroid += hull.Verts[vertex].X }
		index := 1
		if centroid/float64(len(face.Vertices)) < -1 { index = 0 } else if centroid/float64(len(face.Vertices)) > 1 { index = 2 }
		fragments[index].Faces = append(fragments[index].Faces, face)
	}
	return fragments
}
