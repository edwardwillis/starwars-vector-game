// Package starfield provides a deterministic, client-side field of world-space
// stars for visual motion and direction reference.
package starfield

import (
	"math"
	"math/rand"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
)

type Star struct {
	Position   math3d.Vec3
	Brightness uint8
	Size       float32
}

type Point struct {
	X          float64
	Y          float64
	Brightness uint8
	Size       float32
}

type Field struct {
	Stars  []Star
	Radius float64
}

// New distributes stars deterministically inside a cube centered on center.
func New(count int, seed int64, radius float64, center math3d.Vec3) *Field {
	if count < 0 {
		count = 0
	}
	if radius <= 0 {
		panic("starfield: radius must be positive")
	}
	random := rand.New(rand.NewSource(seed))
	field := &Field{Stars: make([]Star, 0, count), Radius: radius}
	for range count {
		brightness, size := appearance(random.Float64())
		field.Stars = append(field.Stars, Star{
			Position: math3d.Vec3{
				X: center.X + uniform(random, radius),
				Y: center.Y + uniform(random, radius),
				Z: center.Z + uniform(random, radius),
			},
			Brightness: brightness,
			Size:       size,
		})
	}
	return field
}

// Wrap keeps every star inside a toroidal cube around reference without
// allocating or generating new random values.
func (f *Field) Wrap(reference math3d.Vec3) {
	for index := range f.Stars {
		f.Stars[index].Position.X = wrapAxis(f.Stars[index].Position.X, reference.X, f.Radius)
		f.Stars[index].Position.Y = wrapAxis(f.Stars[index].Position.Y, reference.Y, f.Radius)
		f.Stars[index].Position.Z = wrapAxis(f.Stars[index].Position.Z, reference.Z, f.Radius)
	}
}

func (f *Field) Project(pipeline render.Pipeline) []Point {
	points := make([]Point, 0, len(f.Stars)/2)
	for _, star := range f.Stars {
		projected, visible := pipeline.ProjectPoint(star.Position)
		if !visible {
			continue
		}
		points = append(points, Point{
			X:          projected.X,
			Y:          projected.Y,
			Brightness: star.Brightness,
			Size:       star.Size,
		})
	}
	return points
}

func appearance(value float64) (uint8, float32) {
	switch {
	case value < 0.70:
		return 140, 0.75
	case value < 0.95:
		return 205, 1
	default:
		return 255, 1.5
	}
}

func uniform(random *rand.Rand, radius float64) float64 {
	return (random.Float64()*2 - 1) * radius
}

func wrapAxis(value, center, radius float64) float64 {
	relative := value - center
	if relative >= -radius && relative <= radius {
		return value
	}
	width := 2 * radius
	relative = math.Mod(relative+radius, width)
	if relative < 0 {
		relative += width
	}
	return center + relative - radius
}
