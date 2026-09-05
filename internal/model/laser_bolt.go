package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

const (
	laserRayCount     = 16
	laserBranchCount  = 6
	laserScale        = 2.0 / 3.0
	laserRayLength    = 0.38 * laserScale
	laserBranchLength = 0.14 * laserScale
)

// LaserBoltRays returns sixteen red radial spokes distributed around a point in
// 3D using a deterministic Fibonacci-sphere layout.
func LaserBoltRays() Model {
	mesh := Model{Verts: []math3d.Vec3{{}}}
	for _, direction := range laserDirections() {
		endpoint := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts, direction.Scale(laserRayLength))
		mesh.Edges = append(mesh.Edges, Edge{A: 0, B: endpoint})
	}
	return Prepare(mesh)
}

// LaserBoltBranches returns six short blue branches around the end of every
// radial spoke. Each branch cluster lies mostly in the tangent plane of its ray.
func LaserBoltBranches() Model {
	mesh := Model{}
	for _, direction := range laserDirections() {
		center := direction.Scale(laserRayLength)
		centerIndex := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts, center)

		reference := math3d.Vec3{Y: 1}
		if math.Abs(direction.Dot(reference)) > 0.9 {
			reference = math3d.Vec3{X: 1}
		}
		tangentA := direction.Cross(reference).Normalize()
		tangentB := direction.Cross(tangentA).Normalize()
		for branch := range laserBranchCount {
			angle := 2 * math.Pi * float64(branch) / laserBranchCount
			sine, cosine := math.Sincos(angle)
			branchDirection := tangentA.Scale(cosine).
				Add(tangentB.Scale(sine)).
				Add(direction.Scale(0.2)).
				Normalize()
			endpoint := len(mesh.Verts)
			mesh.Verts = append(mesh.Verts, center.Add(branchDirection.Scale(laserBranchLength)))
			mesh.Edges = append(mesh.Edges, Edge{A: centerIndex, B: endpoint})
		}
	}
	return Prepare(mesh)
}

func laserDirections() []math3d.Vec3 {
	directions := make([]math3d.Vec3, 0, laserRayCount)
	goldenAngle := math.Pi * (3 - math.Sqrt(5))
	for index := range laserRayCount {
		y := 1 - 2*(float64(index)+0.5)/laserRayCount
		radius := math.Sqrt(1 - y*y)
		angle := goldenAngle * float64(index)
		sine, cosine := math.Sincos(angle)
		directions = append(directions, math3d.Vec3{
			X: cosine * radius,
			Y: y,
			Z: sine * radius,
		})
	}
	return directions
}
