package model

import (
	"math"
	"testing"
)

func TestLaserBoltRays(t *testing.T) {
	rays := LaserBoltRays()
	if len(rays.Verts) != 17 || len(rays.Edges) != 16 {
		t.Fatalf("rays have %d vertices and %d edges, want 17 and 16", len(rays.Verts), len(rays.Edges))
	}
	if err := rays.Validate(); err != nil {
		t.Fatalf("LaserBoltRays returned an invalid model: %v", err)
	}
	for _, vertex := range rays.Verts[1:] {
		if math.Abs(vertex.Length()-laserRayLength) > 1e-9 {
			t.Fatalf("ray length is %v, want %v", vertex.Length(), laserRayLength)
		}
	}
}

func TestLaserBoltBranches(t *testing.T) {
	branches := LaserBoltBranches()
	if len(branches.Verts) != 112 || len(branches.Edges) != 96 {
		t.Fatalf("branches have %d vertices and %d edges, want 112 and 96", len(branches.Verts), len(branches.Edges))
	}
	if err := branches.Validate(); err != nil {
		t.Fatalf("LaserBoltBranches returned an invalid model: %v", err)
	}
	for _, edge := range branches.Edges {
		length := branches.Verts[edge.A].Sub(branches.Verts[edge.B]).Length()
		if math.Abs(length-laserBranchLength) > 1e-9 {
			t.Fatalf("branch length is %v, want %v", length, laserBranchLength)
		}
	}
}
