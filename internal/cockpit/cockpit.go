// Package cockpit contains data-driven fighter cockpit presentations.
package cockpit

import "image/color"

type Cannon struct {
	X, Y            float32
	Housing, Barrel color.RGBA
}

// FrameSegment is a windshield pillar expressed in normalized screen space.
// It is presentation-only: the gameplay renderer never projects, clips, or
// depth-tests it against the 3D world.
type FrameSegment struct {
	FromX, FromY float32
	ToX, ToY     float32
}

type FramePoint struct{ X, Y float32 }

type FramePolygon struct{ Points []FramePoint }

// Palette establishes the visual hierarchy for a cockpit presentation.  The
// structural colour stays deliberately recessive; the computer may use Active
// and Critical sparingly when information requires the pilot's attention.
type Palette struct {
	Structure  color.RGBA
	Instrument color.RGBA
	Active     color.RGBA
	Critical   color.RGBA
}

// XWingComputerMode selects the physical display's information layout. It is
// presentation state, not a new gameplay mode.
type XWingComputerMode uint8

const (
	XWingComputerNavigation XWingComputerMode = iota
	XWingComputerTrench
)

// XWingContact is a normalized local contact position for the targeting
// computer. Coordinates are constrained to roughly [-1, 1], preserving a
// small, schematic display rather than turning it into a world map.
type XWingContact struct {
	X, Y    float32
	Hostile bool
}

// XWingInstrumentState is the small read-only snapshot needed to render the
// cockpit. Game code owns simulation truth and creates this once per frame;
// cockpit presentation never reaches into flight, combat, or mission state.
type XWingInstrumentState struct {
	SpeedPercent      int
	Shields           int
	MaxShields        int
	Torpedoes         int
	Countdown         string
	CountdownFraction float64
	Mode              XWingComputerMode
	Contacts          []XWingContact
	TargetKnown       bool
	TargetX           float32
	TargetY           float32
	TargetRange       float64
	TrenchRange       float64
	VisualDetail      int
}

// Windshield describes lightweight cockpit framing drawn over the world but
// beneath the aiming/HUD symbology.
type Windshield struct {
	Segments       []FrameSegment
	Color          color.RGBA
	LineWidth      float32
	OpaquePolygons []FramePolygon
	OpaqueColor    color.RGBA
	// OpaqueOutlineSegments deliberately excludes joins between adjoining opaque
	// pieces, avoiding a false seam across what should read as one canopy body.
	OpaqueOutlineSegments []FrameSegment
	OpaqueOutlineColor    color.RGBA
	OpaqueOutlineWidth    float32
	InstrumentSegments    []FrameSegment
	InstrumentColor       color.RGBA
	InstrumentLineWidth   float32
}

type Layout struct {
	Definition string
	Cannons    []Cannon
	Windshield Windshield
	Palette    Palette
}

type Registry struct{ layouts map[string]Layout }

func NewRegistry() *Registry { return &Registry{layouts: make(map[string]Layout)} }

func (r *Registry) Register(layout Layout) {
	if r != nil && layout.Definition != "" {
		r.layouts[layout.Definition] = layout
	}
}

func (r *Registry) ForDefinition(definition string) (Layout, bool) {
	if r == nil {
		return Layout{}, false
	}
	layout, ok := r.layouts[definition]
	return layout, ok
}

func DefaultRegistry() *Registry {
	r := NewRegistry()
	tie := Fallback()
	tie.Definition = "builtin/tie-fighter"
	tie.Windshield = tieWindshield()
	r.Register(tie)
	xwing := Fallback()
	xwing.Definition = "builtin/x-wing"
	xwing.Cannons = xWingCannons()
	xwing.Windshield = xWingWindshield()
	xwing.Palette = xWingPalette()
	r.Register(xwing)
	interceptor := Fallback()
	interceptor.Definition = "builtin/tie-interceptor"
	r.Register(interceptor)
	return r
}

func Fallback() Layout {
	red := color.RGBA{R: 255, G: 36, B: 28, A: 255}
	blue := color.RGBA{R: 32, G: 80, B: 255, A: 255}
	return Layout{Cannons: []Cannon{{72, 152, red, blue}, {888, 152, red, blue}, {82, 432, red, blue}, {878, 432, red, blue}}}
}

// xWingCannons keeps the four cockpit barrel assemblies in the open margins
// above and below the side-frame notches. The generic positions overlap the
// X-wing's broader opaque coaming, so they are deliberately X-wing-specific.
func xWingCannons() []Cannon {
	cannons := Fallback().Cannons
	cannons[0].X, cannons[0].Y = 36, 78
	cannons[1].X, cannons[1].Y = 924, 78
	cannons[2].X, cannons[2].Y = 36, 392
	cannons[3].X, cannons[3].Y = 924, 392
	return cannons
}

func xWingWindshield() Windshield {
	return Windshield{
		// The X-wing cockpit mask is one concave closed screen-space region. The
		// two side windows are edge notches in this boundary, not separate parts
		// or cutouts; the main windshield is outside the upper contour.
		OpaqueColor:        color.RGBA{R: 15, G: 17, B: 20, A: 255},
		OpaqueOutlineColor: color.RGBA{R: 34, G: 37, B: 42, A: 220},
		OpaqueOutlineWidth: 1.25,
		OpaquePolygons: []FramePolygon{
			{Points: []FramePoint{
				{0, .132}, {.403, .65}, {.597, .65}, {1, .132},
				{1, .30}, {.635, .675}, {.665, .72}, {1, .806},
				{1, 1}, {0, 1}, {0, .806}, {.335, .72},
				{.365, .675}, {0, .30},
			}},
		},
		// Every edge of the one-piece coaming is drawn with the same quiet grey.
		// This makes the cockpit read as a deliberate solid volume while the
		// uncovered main and side apertures remain its only windows.
		OpaqueOutlineSegments: []FrameSegment{
			{FromX: 0, FromY: .132, ToX: .403, ToY: .65},
			{FromX: .403, FromY: .65, ToX: .597, ToY: .65},
			{FromX: .597, FromY: .65, ToX: 1, ToY: .132},
			{FromX: 1, FromY: .132, ToX: 1, ToY: .30},
			{FromX: 1, FromY: .30, ToX: .635, ToY: .675},
			{FromX: .635, FromY: .675, ToX: .665, ToY: .72},
			{FromX: .665, FromY: .72, ToX: 1, ToY: .806},
			{FromX: 1, FromY: .806, ToX: 1, ToY: 1},
			{FromX: 1, FromY: 1, ToX: 0, ToY: 1},
			{FromX: 0, FromY: 1, ToX: 0, ToY: .806},
			{FromX: 0, FromY: .806, ToX: .335, ToY: .72},
			{FromX: .335, FromY: .72, ToX: .365, ToY: .675},
			{FromX: .365, FromY: .675, ToX: 0, ToY: .30},
			{FromX: 0, FromY: .30, ToX: 0, ToY: .132},
		},
		// These short segments define the raised central binnacle and the
		// upper dashboard edge. The opaque contour remains the physical
		// coaming; these are its significant illuminated structural edges.
		Segments: []FrameSegment{
			{FromX: .403, FromY: .65, ToX: .425, ToY: .70},
			{FromX: .597, FromY: .65, ToX: .575, ToY: .70},
			{FromX: .425, FromY: .70, ToX: .575, ToY: .70},
		},
		Color:     color.RGBA{R: 48, G: 52, B: 58, A: 220},
		LineWidth: 1.1,
	}
}

func xWingPalette() Palette {
	return Palette{
		Structure:  color.RGBA{R: 48, G: 52, B: 58, A: 220},
		Instrument: color.RGBA{R: 150, G: 92, B: 44, A: 205},
		Active:     color.RGBA{R: 232, G: 150, B: 64, A: 235},
		Critical:   color.RGBA{R: 255, G: 88, B: 52, A: 240},
	}
}

func tieWindshield() Windshield {
	return Windshield{
		Color: color.RGBA{R: 88, G: 150, B: 176, A: 210}, LineWidth: 3,
		Segments: []FrameSegment{
			// Outer viewport hexagon.
			{FromX: .22, FromY: .18, ToX: .78, ToY: .18},
			{FromX: .78, FromY: .18, ToX: .91, ToY: .50},
			{FromX: .91, FromY: .50, ToX: .78, ToY: .82},
			{FromX: .78, FromY: .82, ToX: .22, ToY: .82},
			{FromX: .22, FromY: .82, ToX: .09, ToY: .50},
			{FromX: .09, FromY: .50, ToX: .22, ToY: .18},
			// Small central hexagon and six restrained radial braces make the
			// TIE silhouette recognisable without blocking the aiming reticle.
			{FromX: .46, FromY: .43, ToX: .54, ToY: .43},
			{FromX: .54, FromY: .43, ToX: .56, ToY: .50},
			{FromX: .56, FromY: .50, ToX: .54, ToY: .57},
			{FromX: .54, FromY: .57, ToX: .46, ToY: .57},
			{FromX: .46, FromY: .57, ToX: .44, ToY: .50},
			{FromX: .44, FromY: .50, ToX: .46, ToY: .43},
			{FromX: .22, FromY: .18, ToX: .46, ToY: .43},
			{FromX: .78, FromY: .18, ToX: .54, ToY: .43},
			{FromX: .91, FromY: .50, ToX: .56, ToY: .50},
			{FromX: .78, FromY: .82, ToX: .54, ToY: .57},
			{FromX: .22, FromY: .82, ToX: .46, ToY: .57},
			{FromX: .09, FromY: .50, ToX: .44, ToY: .50},
		},
	}
}
