package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// TIEFighter returns a deliberately simple, original wireframe fighter:
// a faceted cockpit, two box pylons, and two tall framed panels. It is inspired
// by the broad geometry of classic twin-panel space fighters without copying a
// production asset.
func TIEFighter() Model {
	mesh := Model{}
	leftPylon, rightPylon := appendTIEFighterCore(&mesh)
	const panelX = 1.61
	appendPanel(&mesh, -panelX, []int{leftPylon, leftPylon + 3, leftPylon + 4, leftPylon + 7})
	appendPanel(&mesh, panelX, []int{rightPylon + 1, rightPylon + 2, rightPylon + 5, rightPylon + 6})
	return OrientOutward(mesh)
}

// TIEFighterCore is the cockpit and pylon assembly without the solar-panel
// foils. Keeping the physical assemblies separate lets the renderer assign
// independent depth ownership, so a foil can occlude cockpit lines while each
// part's own structural edges remain visible.
func TIEFighterCore() Model {
	mesh := Model{}
	appendTIEFighterCore(&mesh)
	return OrientOutward(mesh)
}

func appendTIEFighterCore(mesh *Model) (int, int) {
	appendCockpit(mesh)
	const (
		panelX      = 1.61
		pylonInnerX = 0.65
		pylonWidth  = panelX - pylonInnerX
		pylonCenter = (panelX + pylonInnerX) / 2
	)
	leftPylon := appendBox(mesh, -pylonCenter, 0, 0, pylonWidth, 0.32, 0.32)
	rightPylon := appendBox(mesh, pylonCenter, 0, 0, pylonWidth, 0.32, 0.32)
	return leftPylon, rightPylon
}

// TIEFighterFoils contains the two finite-thickness solar panels. The support
// box faces in TIEFighterCore meet the inner panel planes, so the two models
// still form one connected visible fighter when rendered as separate parts.
func TIEFighterFoils() Model {
	return Merge(TIEFighterFoil(-1), TIEFighterFoil(1))
}

// TIEFighterFoil returns one independently occluding solar-panel assembly.
// side must be -1 (left) or +1 (right).
func TIEFighterFoil(side int) Model {
	if side != -1 && side != 1 {
		panic("model: TIE foil side must be -1 or +1")
	}
	mesh := Model{}
	appendPanel(&mesh, 1.61*float64(side), nil)
	return OrientOutward(mesh)
}

func appendCockpit(mesh *Model) {
	const segments = 8
	// Three rings and two shallow end caps create a rounded, low-poly hull.
	rings := []struct {
		x      float64
		radius float64
	}{
		{x: -0.42, radius: 0.49},
		{x: 0, radius: 0.68},
		{x: 0.42, radius: 0.49},
	}

	for _, ring := range rings {
		for segment := range segments {
			angle := 2 * math.Pi * float64(segment) / segments
			sine, cosine := math.Sincos(angle)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{
				X: ring.x,
				Y: ring.radius * cosine,
				Z: ring.radius * sine,
			})
		}
	}
	for ring := range rings {
		base := ring * segments
		for segment := range segments {
			next := (segment + 1) % segments
			mesh.Edges = append(mesh.Edges, Edge{A: base + segment, B: base + next})
			if ring < len(rings)-1 {
				mesh.Edges = append(mesh.Edges, Edge{A: base + segment, B: base + segments + segment})
				mesh.Faces = append(mesh.Faces, Face{Vertices: []int{
					base + segment,
					base + next,
					base + segments + next,
					base + segments + segment,
				}})
			}
		}
	}

	leftCap := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{X: -0.68})
	rightCap := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{X: 0.68})
	for segment := range segments {
		mesh.Edges = append(mesh.Edges,
			Edge{A: leftCap, B: segment},
			Edge{A: rightCap, B: (len(rings)-1)*segments + segment},
		)
		next := (segment + 1) % segments
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{leftCap, next, segment}},
			Face{Vertices: []int{rightCap, (len(rings)-1)*segments + segment, (len(rings)-1)*segments + next}},
		)
	}

}

// TIEFighterWindow returns the inset forward window as a separate model
// so the game can draw it in a contrasting vector color.
func TIEFighterWindow() Model {
	const (
		segments      = 8
		windowRadius  = 0.29
		cockpitRadius = 0.68
	)
	windowZ := math.Sqrt(cockpitRadius*cockpitRadius-windowRadius*windowRadius) + 0.015
	window := Model{}
	for segment := range segments {
		angle := 2 * math.Pi * float64(segment) / segments
		sine, cosine := math.Sincos(angle)
		window.Verts = append(window.Verts, math3d.Vec3{
			X: windowRadius * cosine,
			Y: windowRadius * sine,
			Z: windowZ,
		})
	}
	center := len(window.Verts)
	window.Verts = append(window.Verts, math3d.Vec3{Z: cockpitRadius + 0.015})
	for segment := range segments {
		next := (segment + 1) % segments
		window.Edges = append(window.Edges,
			Edge{A: segment, B: next},
			Edge{A: segment, B: center},
		)
		window.Faces = append(window.Faces, Face{Vertices: []int{segment, next, center}})
	}
	return Prepare(window)
}

// TIEFighterFragments partitions the fighter's edges into left, center,
// and right debris meshes. Together the three fragments reconstruct the hull.
func TIEFighterFragments() [3]Model {
	hull := TIEFighter()
	fragments := [3]Model{
		{Verts: hull.Verts},
		{Verts: hull.Verts},
		{Verts: hull.Verts},
	}
	for _, edge := range hull.Edges {
		midpointX := (hull.Verts[edge.A].X + hull.Verts[edge.B].X) * 0.5
		index := 1
		if midpointX < -0.48 {
			index = 0
		} else if midpointX > 0.48 {
			index = 2
		}
		fragments[index].Edges = append(fragments[index].Edges, edge)
	}
	for _, face := range hull.Faces {
		centroidX := 0.0
		for _, vertex := range face.Vertices {
			centroidX += hull.Verts[vertex].X
		}
		centroidX /= float64(len(face.Vertices))
		index := 1
		if centroidX < -0.48 {
			index = 0
		} else if centroidX > 0.48 {
			index = 2
		}
		fragments[index].Faces = append(fragments[index].Faces, face)
	}
	for index := range fragments {
		fragments[index] = Prepare(fragments[index])
	}
	return fragments
}

func appendBox(mesh *Model, centerX, centerY, centerZ, width, height, depth float64) int {
	base := len(mesh.Verts)
	halfWidth, halfHeight, halfDepth := width/2, height/2, depth/2
	mesh.Verts = append(mesh.Verts,
		math3d.Vec3{X: centerX - halfWidth, Y: centerY - halfHeight, Z: centerZ - halfDepth},
		math3d.Vec3{X: centerX + halfWidth, Y: centerY - halfHeight, Z: centerZ - halfDepth},
		math3d.Vec3{X: centerX + halfWidth, Y: centerY + halfHeight, Z: centerZ - halfDepth},
		math3d.Vec3{X: centerX - halfWidth, Y: centerY + halfHeight, Z: centerZ - halfDepth},
		math3d.Vec3{X: centerX - halfWidth, Y: centerY - halfHeight, Z: centerZ + halfDepth},
		math3d.Vec3{X: centerX + halfWidth, Y: centerY - halfHeight, Z: centerZ + halfDepth},
		math3d.Vec3{X: centerX + halfWidth, Y: centerY + halfHeight, Z: centerZ + halfDepth},
		math3d.Vec3{X: centerX - halfWidth, Y: centerY + halfHeight, Z: centerZ + halfDepth},
	)
	boxEdges := []Edge{
		{A: 0, B: 1}, {A: 1, B: 2}, {A: 2, B: 3}, {A: 3, B: 0},
		{A: 4, B: 5}, {A: 5, B: 6}, {A: 6, B: 7}, {A: 7, B: 4},
		{A: 0, B: 4}, {A: 1, B: 5}, {A: 2, B: 6}, {A: 3, B: 7},
	}
	for _, edge := range boxEdges {
		mesh.Edges = append(mesh.Edges, Edge{A: base + edge.A, B: base + edge.B})
	}
	boxFaces := [...][4]int{
		{0, 3, 2, 1}, {4, 5, 6, 7}, {0, 1, 5, 4},
		{1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7},
	}
	for _, face := range boxFaces {
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{
			base + face[0], base + face[1], base + face[2], base + face[3],
		}})
	}
	return base
}

func appendPanel(mesh *Model, x float64, pylonFace []int) {
	base := len(mesh.Verts)
	// A point-topped regular hexagon in the YZ plane.
	const panelRadius = 1.27
	panelHalfWidth := math.Sqrt(3) * panelRadius / 2
	front := []math3d.Vec3{
		math3d.Vec3{X: x, Y: -panelRadius},
		math3d.Vec3{X: x, Y: -panelRadius / 2, Z: -panelHalfWidth},
		math3d.Vec3{X: x, Y: panelRadius / 2, Z: -panelHalfWidth},
		math3d.Vec3{X: x, Y: panelRadius},
		math3d.Vec3{X: x, Y: panelRadius / 2, Z: panelHalfWidth},
		math3d.Vec3{X: x, Y: -panelRadius / 2, Z: panelHalfWidth},
		math3d.Vec3{X: x},
	}
	mesh.Verts = append(mesh.Verts, front...)
	thickness := 0.06
	// Keep the authored silhouette at the outer panel plane while adding a
	// shallow body inward toward the cockpit/pylon.
	backX := x - math.Copysign(thickness, x)
	backBase := len(mesh.Verts)
	for _, vertex := range front { vertex.X = backX; mesh.Verts = append(mesh.Verts, vertex) }
	const corners = 6
	hub := base + corners
	backHub := backBase + corners
	for corner := range corners {
		next := (corner + 1) % corners
		mesh.Edges = append(mesh.Edges,
			Edge{A: base + corner, B: base + next, Kind: EdgeStructural},
			Edge{A: backBase + corner, B: backBase + next, Kind: EdgeStructural},
			Edge{A: base + corner, B: backBase + corner, Kind: EdgeStructural},
			Edge{A: base + corner, B: hub, Kind: EdgeDecorative},
			Edge{A: backBase + corner, B: backHub, Kind: EdgeDecorative},
		)
	}
	frontFace := make([]int, corners)
	backFace := make([]int, corners)
	for corner := range corners { frontFace[corner], backFace[corner] = base+corner, backBase+corner }
	if x < 0 { reverseIndices(frontFace) } else { reverseIndices(backFace) }
	mesh.Faces = append(mesh.Faces, Face{Vertices: frontFace}, Face{Vertices: backFace})
	for corner := range corners {
		next := (corner + 1) % corners
		side := []int{base + corner, base + next, backBase + next, backBase + corner}
		if x < 0 { reverseIndices(side) }
		mesh.Faces = append(mesh.Faces, Face{Vertices: side})
	}
	for _, vertex := range pylonFace {
		mesh.Edges = append(mesh.Edges, Edge{A: vertex, B: hub, Kind: EdgeDecorative})
	}
}

func reverseIndices(indices []int) {
	for left, right := 0, len(indices)-1; left < right; left, right = left+1, right-1 {
		indices[left], indices[right] = indices[right], indices[left]
	}
}
