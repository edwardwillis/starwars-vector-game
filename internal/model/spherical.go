package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// SphericalPlacement describes a local tangent frame on a sphere. Angles are
// radians; local +Z points away from the sphere and local +Y points north.
type SphericalPlacement struct {
	Latitude  float64
	Longitude float64
	Radius    float64
	Offset    float64
	Scale     float64
}

func (placement SphericalPlacement) Matrix() math3d.Mat4 {
	sinLat, cosLat := math.Sincos(placement.Latitude)
	sinLon, cosLon := math.Sincos(placement.Longitude)
	normal := math3d.Vec3{X: cosLat * sinLon, Y: sinLat, Z: cosLat * cosLon}
	east := math3d.Vec3{X: cosLon, Z: -sinLon}
	north := normal.Cross(east).Normalize()
	scale := placement.Scale
	if scale == 0 {
		scale = 1
	}
	position := normal.Scale(placement.Radius + placement.Offset)
	return math3d.Mat4{
		{east.X * scale, north.X * scale, normal.X * scale, position.X},
		{east.Y * scale, north.Y * scale, normal.Y * scale, position.Y},
		{east.Z * scale, north.Z * scale, normal.Z * scale, position.Z},
		{0, 0, 0, 1},
	}
}
