package appearance

import (
	"image/color"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
)

const DeathStarArcadeName = "builtin/death-star-arcade-billboard"

// DeathStarArcade builds one normalized drawing. The apparent increase in
// surface detail is stable seeded reveal, not a different model or animation.
func DeathStarArcade() Definition {
	green := color.RGBA{R: 64, G: 255, B: 96, A: 255}
	red := color.RGBA{R: 255, G: 48, B: 48, A: 255}
	white := color.RGBA{R: 220, G: 220, B: 220, A: 255}
	base := []Line{}
	const outlineSegments = 32
	for index := 0; index < outlineSegments; index++ {
		first := 2 * math.Pi * float64(index) / outlineSegments
		second := 2 * math.Pi * float64(index+1) / outlineSegments
		base = append(base, Line{A: Point{X: math.Cos(first), Y: math.Sin(first)}, B: Point{X: math.Cos(second), Y: math.Sin(second)}, Color: green, Width: 1.5})
	}
	base = append(base,
		Line{A: Point{X: -1, Y: 0}, B: Point{X: 1, Y: 0}, Color: white, Width: 1.5},
	)
	// Dish in the upper-right quadrant, matching the original arcade cue.
	const dishX, dishY = 0.55, 0.48
	axisLength := math.Hypot(dishX, dishY)
	axisX, axisY := dishX/axisLength, dishY/axisLength
	dishRotation := math.Atan2(dishY, dishX) + math.Pi/2
	ellipsePoint := func(centerX, centerY, radiusX, radiusY, angle float64) Point {
		localX, localY := radiusX*math.Cos(angle), radiusY*math.Sin(angle)
		return Point{
			X: centerX + localX*math.Cos(dishRotation) - localY*math.Sin(dishRotation),
			Y: centerY + localX*math.Sin(dishRotation) + localY*math.Cos(dishRotation),
		}
	}
	// Nested rings move toward the Death Star centre to imply a recessed bowl.
	midCenter := Point{X: dishX - axisX*0.025, Y: dishY - axisY*0.025}
	deepCenter := Point{X: dishX - axisX*0.055, Y: dishY - axisY*0.055}
	for index := 0; index < 16; index++ {
		first := 2 * math.Pi * float64(index) / 16
		second := 2 * math.Pi * float64(index+1) / 16
		outerFirst := ellipsePoint(dishX, dishY, 0.22, 0.15, first)
		outerSecond := ellipsePoint(dishX, dishY, 0.22, 0.15, second)
		base = append(base,
			Line{A: outerFirst, B: outerSecond, Color: red, Width: 1.5},
			Line{A: deepCenter, B: outerFirst, Color: red, Width: 1.2},
		)
	}
	for index := 0; index < 8; index++ {
		first := 2 * math.Pi * float64(index) / 8
		second := 2 * math.Pi * float64(index+1) / 8
		base = append(base, Line{A: ellipsePoint(deepCenter.X, deepCenter.Y, 0.055, 0.035, first), B: ellipsePoint(deepCenter.X, deepCenter.Y, 0.055, 0.035, second), Color: red, Width: 1.2})
	}
	for index := 0; index < 12; index++ {
		first := 2 * math.Pi * float64(index) / 12
		second := 2 * math.Pi * float64(index+1) / 12
		base = append(base, Line{A: ellipsePoint(midCenter.X, midCenter.Y, 0.135, 0.09, first), B: ellipsePoint(midCenter.X, midCenter.Y, 0.135, 0.09, second), Color: red, Width: 1.1})
	}

	details := make([]Detail, 0, 72)
	state := uint64(0x8f31a7c5d29e104b)
	for index := 0; index < 72; index++ {
		state = state*6364136223846793005 + 1442695040888963407
		x := 0.72 * (float64(state>>11)/float64(uint64(1)<<53) - 0.5)
		state = state*6364136223846793005 + 1442695040888963407
		y := 0.72 * (float64(state>>11)/float64(uint64(1)<<53) - 0.5)
		length := 0.025 + 0.025*float64((state>>8)%100)/100
		line := Line{A: Point{X: x, Y: y}, B: Point{X: x + length, Y: y + 0.004}, Color: green, Width: 0.8}
		if index%5 == 0 {
			line.Color = color.RGBA{R: 210, G: 190, B: 48, A: 255}
		}
		details = append(details, Detail{Threshold: 0.12 + 0.82*float64(index)/71, Line: line})
	}
	return Definition{Name: DeathStarArcadeName, ObjectDefinition: catalog.DeathStarName, Kind: "vector-billboard", Billboard: Billboard{Name: DeathStarArcadeName, Base: base, Details: details, Occludes: true}}
}

func DefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = registry.Register(DeathStarArcade())
	_ = registry.Register(Definition{Name: "builtin/death-star-orbital-wireframe", ObjectDefinition: catalog.DeathStarName, Kind: "model-3d"})
	return registry
}
