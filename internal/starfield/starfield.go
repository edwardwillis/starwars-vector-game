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
	Direction  math3d.Vec3
	Brightness uint8
	Size       float32
}

const (
	ModeWorld    = "world"
	ModeSkyfield = "skyfield"
)

type Point struct {
	X          float64
	Y          float64
	Depth      float64
	Brightness uint8
	Size       float32
}

type Field struct {
	Stars       []Star
	Radius      float64
	Center      math3d.Vec3
	Mode        string
	SkyDistance float64
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
	field := &Field{Stars: make([]Star, 0, count), Radius: radius, Center: center, Mode: ModeWorld}
	for range count {
		brightness, size := appearance(random.Float64())
		position := math3d.Vec3{
				X: center.X + uniform(random, radius),
				Y: center.Y + uniform(random, radius),
				Z: center.Z + uniform(random, radius),
			}
		direction := position.Sub(center).Normalize()
		field.Stars = append(field.Stars, Star{
			Position: position,
			Direction: direction,
			Brightness: brightness,
			Size:       size,
		})
	}
	return field
}

// Wrap keeps every star inside a toroidal cube around reference without
// allocating or generating new random values.
func (f *Field) Wrap(reference math3d.Vec3) {
	if f.Mode == ModeSkyfield {
		f.Center = reference
		return
	}
	for index := range f.Stars {
		f.Stars[index].Position.X = wrapAxis(f.Stars[index].Position.X, reference.X, f.Radius)
		f.Stars[index].Position.Y = wrapAxis(f.Stars[index].Position.Y, reference.Y, f.Radius)
		f.Stars[index].Position.Z = wrapAxis(f.Stars[index].Position.Z, reference.Z, f.Radius)
	}
}

// SetMode selects the starfield's depth model. Skyfield mode preserves each
// star's direction from the field centre while placing it at a distant shell,
// so large world objects can reliably occlude it without visible parallax.
func (f *Field) SetMode(mode string, distance float64) {
	if mode != ModeSkyfield {
		f.Mode = ModeWorld
		return
	}
	f.Mode = ModeSkyfield
	if distance > 0 {
		f.SkyDistance = distance
	}
}

func (f *Field) Project(pipeline render.Pipeline) []Point {
	return f.ProjectInto(pipeline, nil)
}

// ProjectInto projects stars into caller-owned scratch storage. Reusing the
// returned slice avoids a per-frame allocation for the stable starfield.
func (f *Field) ProjectInto(pipeline render.Pipeline, points []Point, occluders ...render.PointOccluder) []Point {
	if cap(points) < len(f.Stars)/2 {
		points = make([]Point, 0, len(f.Stars)/2)
	} else {
		points = points[:0]
	}
	var cameraPosition math3d.Vec3
	if f.Mode == ModeSkyfield && f.SkyDistance > 0 {
		cameraPosition = pipeline.CameraPosition()
	}
	for _, star := range f.Stars {
		position := star.Position
		if f.Mode == ModeSkyfield && f.SkyDistance > 0 {
			direction := star.Direction
			if direction.Length() <= 1e-9 {
				direction = star.Position.Sub(f.Center).Normalize()
			}
			if direction.Length() <= 1e-9 {
				continue
			}
			position = cameraPosition.Add(direction.Scale(f.SkyDistance))
		}
		projected, visible := pipeline.ProjectPoint(position)
		if !visible {
			continue
		}
		if pipeline.Stats != nil {
			pipeline.Stats.StarsConsidered++
		}
		if len(occluders) > 0 && occluders[0] != nil && occluders[0].OccludesPoint(render.Point{X: projected.X, Y: projected.Y, Depth: projected.Depth}) {
			continue
		}
		if pipeline.Stats != nil {
			pipeline.Stats.StarsSubmitted++
		}
		points = append(points, Point{
			X:          projected.X,
			Y:          projected.Y,
			Depth:      projected.Depth,
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
