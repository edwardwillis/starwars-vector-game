package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

type DeathStarGeometry struct {
	Sphere Model
	Dish   Model
}

// DeathStar builds sparse, deterministic geometry in object-local coordinates.
func DeathStar(radius float64) DeathStarGeometry {
	if radius <= 0 {
		panic("model: Death Star radius must be positive")
	}
	return DeathStarGeometry{
		Sphere: sparseSphere(radius, 12, 6),
		Dish:   Transform(recessedDish(radius*0.22, 12), (SphericalPlacement{Latitude: 0.48, Longitude: math.Pi, Radius: radius, Offset: 0.03}).Matrix()),
	}
}

func sparseSphere(radius float64, longitudeSegments, latitudeBands int) Model {
	mesh := Model{}
	columns := longitudeSegments
	for band := 0; band <= latitudeBands; band++ {
		latitude := -math.Pi/2 + math.Pi*float64(band)/float64(latitudeBands)
		sinLat, cosLat := math.Sincos(latitude)
		for column := 0; column < columns; column++ {
			longitude := 2 * math.Pi * float64(column) / float64(columns)
			sinLon, cosLon := math.Sincos(longitude)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: radius * cosLat * sinLon, Y: radius * sinLat, Z: radius * cosLat * cosLon})
		}
	}
	for band := 0; band <= latitudeBands; band++ {
		for column := 0; column < columns; column++ {
			next := (column + 1) % columns
			if band > 0 && band < latitudeBands {
				mesh.Edges = append(mesh.Edges, Edge{A: band*columns + column, B: band*columns + next})
			}
			if band < latitudeBands {
				mesh.Edges = append(mesh.Edges, Edge{A: band*columns + column, B: (band+1)*columns + column})
			}
			if band < latitudeBands {
				mesh.Faces = append(mesh.Faces, Face{Vertices: []int{band*columns + column, band*columns + next, (band+1)*columns + next, (band+1)*columns + column}})
			}
		}
	}
	return mesh
}

func recessedDish(radius float64, segments int) Model {
	mesh := Model{}
	rings := []struct{ radius, depth float64 }{{radius, 0}, {radius * 0.62, -radius * 0.13}, {radius * 0.27, -radius * 0.25}}
	for _, ring := range rings {
		base := len(mesh.Verts)
		for segment := 0; segment < segments; segment++ {
			angle := 2 * math.Pi * float64(segment) / float64(segments)
			sine, cosine := math.Sincos(angle)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: ring.radius * cosine, Y: ring.radius * sine, Z: ring.depth})
			mesh.Edges = append(mesh.Edges, Edge{A: base + segment, B: base + (segment+1)%segments})
		}
	}
	center := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Z: -radius * 0.32})
	for segment := 0; segment < segments; segment += 2 {
		mesh.Edges = append(mesh.Edges, Edge{A: segment, B: center})
	}
	return mesh
}
