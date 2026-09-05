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
	south := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Y: -radius})
	ring := func(band int) int { return 1 + (band-1)*columns }
	for band := 1; band < latitudeBands; band++ {
		latitude := -math.Pi/2 + math.Pi*float64(band)/float64(latitudeBands)
		sinLat, cosLat := math.Sincos(latitude)
		for column := 0; column < columns; column++ {
			longitude := 2 * math.Pi * float64(column) / float64(columns)
			sinLon, cosLon := math.Sincos(longitude)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: radius * cosLat * sinLon, Y: radius * sinLat, Z: radius * cosLat * cosLon})
		}
	}
	north := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Y: radius})
	for column := 0; column < columns; column++ {
		next := (column + 1) % columns
		mesh.Edges = append(mesh.Edges, Edge{A: south, B: ring(1) + column})
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{south, ring(1) + next, ring(1) + column}})
	}
	for band := 1; band < latitudeBands-1; band++ {
		for column := 0; column < columns; column++ {
			next := (column + 1) % columns
			current, upper := ring(band)+column, ring(band+1)+column
			mesh.Edges = append(mesh.Edges, Edge{A: current, B: ring(band) + next}, Edge{A: current, B: upper})
			mesh.Faces = append(mesh.Faces, Face{Vertices: []int{current, ring(band) + next, ring(band+1) + next, upper}})
		}
	}
	for column := 0; column < columns; column++ {
		next := (column + 1) % columns
		mesh.Edges = append(mesh.Edges, Edge{A: ring(latitudeBands-1) + column, B: north})
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{north, ring(latitudeBands-1) + column, ring(latitudeBands-1) + next}})
	}
	return Prepare(mesh)
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
	for segment := 0; segment < segments; segment++ {
		mesh.Edges = append(mesh.Edges, Edge{A: 2*segments + segment, B: center, Kind: EdgeDecorative})
	}
	for ring := 0; ring < len(rings)-1; ring++ {
		base, nextBase := ring*segments, (ring+1)*segments
		for segment := 0; segment < segments; segment++ {
			next := (segment + 1) % segments
			mesh.Faces = append(mesh.Faces, Face{Vertices: []int{base + segment, base + next, nextBase + next, nextBase + segment}})
		}
	}
	innerBase := 2 * segments
	for segment := 0; segment < segments; segment++ {
		next := (segment + 1) % segments
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{innerBase + segment, innerBase + next, center}})
	}
	return Prepare(mesh)
}
