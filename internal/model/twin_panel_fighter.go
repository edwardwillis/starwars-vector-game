package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// TwinPanelFighter returns a deliberately simple, original wireframe fighter:
// a faceted cockpit, two box pylons, and two tall framed panels. It is inspired
// by the broad geometry of classic twin-panel space fighters without copying a
// production asset.
func TwinPanelFighter() Model {
	mesh := Model{}

	appendCockpit(&mesh)

	const (
		panelX      = 1.61
		pylonInnerX = 0.65
		pylonWidth  = panelX - pylonInnerX
		pylonCenter = (panelX + pylonInnerX) / 2
	)
	leftPylon := appendBox(&mesh, -pylonCenter, 0, 0, pylonWidth, 0.32, 0.32)
	rightPylon := appendBox(&mesh, pylonCenter, 0, 0, pylonWidth, 0.32, 0.32)
	appendPanel(&mesh, -panelX, []int{leftPylon, leftPylon + 3, leftPylon + 4, leftPylon + 7})
	appendPanel(&mesh, panelX, []int{rightPylon + 1, rightPylon + 2, rightPylon + 5, rightPylon + 6})

	return mesh
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
	}

}

// TwinPanelFighterWindow returns the inset forward window as a separate model
// so the game can draw it in a contrasting vector color.
func TwinPanelFighterWindow() Model {
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
		window.Edges = append(window.Edges,
			Edge{A: segment, B: (segment + 1) % segments},
			Edge{A: segment, B: center},
		)
	}
	return window
}

// TwinPanelFighterFragments partitions the fighter's edges into left, center,
// and right debris meshes. Together the three fragments reconstruct the hull.
func TwinPanelFighterFragments() [3]Model {
	hull := TwinPanelFighter()
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
	return base
}

func appendPanel(mesh *Model, x float64, pylonFace []int) {
	base := len(mesh.Verts)
	// A point-topped regular hexagon in the YZ plane.
	const panelRadius = 1.27
	panelHalfWidth := math.Sqrt(3) * panelRadius / 2
	mesh.Verts = append(mesh.Verts,
		math3d.Vec3{X: x, Y: -panelRadius},
		math3d.Vec3{X: x, Y: -panelRadius / 2, Z: -panelHalfWidth},
		math3d.Vec3{X: x, Y: panelRadius / 2, Z: -panelHalfWidth},
		math3d.Vec3{X: x, Y: panelRadius},
		math3d.Vec3{X: x, Y: panelRadius / 2, Z: panelHalfWidth},
		math3d.Vec3{X: x, Y: -panelRadius / 2, Z: panelHalfWidth},
		math3d.Vec3{X: x},
	)
	const corners = 6
	hub := base + corners
	for corner := range corners {
		mesh.Edges = append(mesh.Edges,
			Edge{A: base + corner, B: base + (corner+1)%corners},
			Edge{A: base + corner, B: hub},
		)
	}
	for _, vertex := range pylonFace {
		mesh.Edges = append(mesh.Edges, Edge{A: vertex, B: hub})
	}
}
