package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

const wreckageWheelSegments = 12

// TIEWreckageWheel is a deliberately anachronistic car wheel used only as a
// small piece of TIE fighter wreckage. Its axis is longitudinal (X), so its
// rapid roll remains legible when it tumbles away from a destroyed fighter.
// The visible face has a distinct tyre, rim, six spokes, hub, and wheel nuts
// so it reads as a wheel even at the deliberately spare vector-art detail.
func TIEWreckageWheel() Model {
	tyre := wheelTyre()
	rim := wheelDisc(0.325, 0.125)
	hub := wheelDisc(0.095, 0.175)
	return Merge(tyre, rim, hub, wheelFaceDetail(0.181), wheelFaceDetail(-0.181))
}

func wheelTyre() Model {
	// The first and last stations are the recessed tyre beads; the raised
	// middle stations make the thick, rounded tyre read clearly in side view.
	sections := []struct {
		x, radius float64
	}{
		{x: -0.155, radius: 0.325},
		{x: -0.145, radius: 0.425},
		{x: -0.09, radius: 0.46},
		{x: 0.09, radius: 0.46},
		{x: 0.145, radius: 0.425},
		{x: 0.155, radius: 0.325},
	}
	mesh := Model{}
	for _, section := range sections {
		appendWheelRing(&mesh, section.x, section.radius)
	}
	for section := 0; section+1 < len(sections); section++ {
		for segment := range wreckageWheelSegments {
			next := (segment + 1) % wreckageWheelSegments
			base := section * wreckageWheelSegments
			following := (section + 1) * wreckageWheelSegments
			mesh.Faces = append(mesh.Faces, Face{Vertices: []int{
				base + segment, base + next, following + next, following + segment,
			}})
		}
	}
	return OrientOutward(mesh)
}

func wheelDisc(radius, halfWidth float64) Model {
	mesh := Model{}
	appendWheelRing(&mesh, -halfWidth, radius)
	appendWheelRing(&mesh, halfWidth, radius)
	for segment := range wreckageWheelSegments {
		next := (segment + 1) % wreckageWheelSegments
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{
			segment, next, wreckageWheelSegments + next, wreckageWheelSegments + segment,
		}})
	}
	front, back := make([]int, wreckageWheelSegments), make([]int, wreckageWheelSegments)
	for segment := range wreckageWheelSegments {
		front[segment] = segment
		back[segment] = wreckageWheelSegments + segment
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: front}, Face{Vertices: back})
	return OrientOutward(mesh)
}

func wheelFaceDetail(faceX float64) Model {
	// Place line art just beyond the exposed rim cap, avoiding coplanar depth
	// shimmer while keeping the tyre itself a genuinely thick opaque body.
	mesh := Model{}
	appendWheelCircle(&mesh, faceX, 0.292, wreckageWheelSegments)
	appendWheelCircle(&mesh, faceX+0.002, 0.13, wreckageWheelSegments)
	appendWheelCircle(&mesh, faceX+0.004, 0.062, wreckageWheelSegments)

	for spoke := 0; spoke < 6; spoke++ {
		angle := math.Pi/2 + 2*math.Pi*float64(spoke)/6
		// A slightly tapered trapezoid makes each spoke point outward from the
		// hub, echoing a simple six-spoke car wheel without visual clutter.
		outline := []math3d.Vec3{
			wheelPolar(faceX+0.003, 0.125, angle-0.16),
			wheelPolar(faceX+0.003, 0.125, angle+0.16),
			wheelPolar(faceX+0.003, 0.276, angle+0.10),
			wheelPolar(faceX+0.003, 0.276, angle-0.10),
		}
		appendWheelOutline(&mesh, outline)

	}
	return Prepare(mesh)
}

func appendWheelRing(mesh *Model, x, radius float64) {
	for segment := range wreckageWheelSegments {
		mesh.Verts = append(mesh.Verts, wheelPolar(x, radius, 2*math.Pi*float64(segment)/wreckageWheelSegments))
	}
}

func appendWheelCircle(mesh *Model, x, radius float64, segments int) {
	start := len(mesh.Verts)
	for segment := range segments {
		mesh.Verts = append(mesh.Verts, wheelPolar(x, radius, 2*math.Pi*float64(segment)/float64(segments)))
	}
	for segment := range segments {
		mesh.Edges = append(mesh.Edges, Edge{A: start + segment, B: start + (segment+1)%segments, Kind: EdgeDecorative, Importance: 0.85})
	}
}

func appendWheelOutline(mesh *Model, outline []math3d.Vec3) {
	start := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, outline...)
	for index := range outline {
		mesh.Edges = append(mesh.Edges, Edge{A: start + index, B: start + (index+1)%len(outline), Kind: EdgeDecorative, Importance: 0.9})
	}
}

func wheelPolar(x, radius, angle float64) math3d.Vec3 {
	return math3d.Vec3{X: x, Y: radius * math.Cos(angle), Z: radius * math.Sin(angle)}
}
