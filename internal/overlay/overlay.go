// Package overlay provides lightweight, presentation-only character portraits.
//
// It owns decoded portrait images and their short screen-space animations, but
// has no dependency on mission or simulation state. Callers advance it from
// their presentation clock and may draw it after their normal HUD.
package overlay

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

// ID identifies a configured character portrait.
type ID string

const (
	// Jabba is the initial portrait supplied with the game.
	Jabba     ID = "jabba"
	R2D2      ID = "r2-d2"
	Chewbacca ID = "chewbacca"
	HanSolo   ID = "han-solo"
)

// State is the visible overlay lifecycle.
type State uint8

const (
	Hidden State = iota
	Entering
	Holding
	Exiting
)

// Effect selects the presentation treatment for a portrait.
type Effect uint8

const (
	EffectSlide Effect = iota
	EffectHologram
)

// Edge identifies where a portrait enters and exits the viewport.
type Edge uint8

const (
	FromBottom Edge = iota
	FromLeft
	FromTop
	FromRight
	// RandomEdge delegates the choice to the manager's presentation-only PRNG.
	RandomEdge
)

// Anchor positions the portrait's resting rectangle in the viewport.
type Anchor uint8

const (
	LowerRight Anchor = iota
	LowerLeft
	UpperRight
	UpperLeft
)

// Animation controls a portrait lifecycle. Durations are seconds.
type Animation struct {
	// Edge can be RandomEdge. The resolved edge remains fixed for the whole
	// lifecycle, including exit.
	Edge Edge
	// VisibleDepth is the on-screen fraction of the rotated dimension
	// perpendicular to Edge. Zero preserves the original fully-visible layout.
	VisibleDepth float64
	// EdgeOffset positions the portrait along its entrance edge as a fraction
	// of the available range. Values outside [0, 1] are clamped.
	EdgeOffset    float64
	StartDelay    float64
	EnterDuration float64
	HoldDuration  float64
	ExitDuration  float64
	Fade          bool
}

// PlaybackPolicy specifies what happens to an already-started voice when its
// portrait is cancelled or replaced.
type PlaybackPolicy uint8

const (
	// FinishNaturally leaves the voice alone. This is the default and avoids
	// abruptly cutting off a character line during a navigation transition.
	FinishNaturally PlaybackPolicy = iota
	// StopOnCancel stops a voice owned by an overlay when it is cancelled or
	// replaced, if the audio sink supports independently stoppable voices.
	StopOnCancel
)

// Audio configures an optional sound associated with a portrait appearance.
// Asset is an application-defined asset key; an empty key disables audio.
type Audio struct {
	Asset  string
	Delay  float64
	Volume float64
	Policy PlaybackPolicy
}

// AudioEvent is a one-shot request emitted from Update when an active
// portrait reaches its configured audio time.
type AudioEvent struct {
	Asset  string
	Volume float64
	Policy PlaybackPolicy
}

const (
	defaultHologramStrips = 12
	maxHologramStrips     = 16
)

// Hologram configures the deliberately small first-pass transmission effect.
type Hologram struct {
	Tint            color.RGBA
	EnteringFlicker float64
	HoldingFlicker  float64
	ExitingFlicker  float64
	StripCount      int
	MaxDisplacement float64
	InterferenceHz  float64
}

// Presentation controls a portrait's resting position and appearance. Insets
// are logical screen pixels; WidthFraction is relative to the current target.
type Presentation struct {
	Effect        Effect
	Anchor        Anchor
	InsetX        float64
	InsetY        float64
	WidthFraction float64
	Scale         float64
	Opacity       float32
	Tint          color.RGBA
	Animation     Animation
	Audio         Audio
	Hologram      Hologram
	// Caption is reserved for a future text treatment; it has no backing panel
	// and intentionally does not affect portrait layout.
	Caption string
}

// DefaultSlidePresentation is shared by characters that peek in from a
// randomly selected edge. Characters may override only the fields they need.
var DefaultSlidePresentation = Presentation{
	Effect:        EffectSlide,
	Anchor:        LowerRight,
	InsetX:        22,
	InsetY:        16,
	WidthFraction: .30,
	Scale:         1,
	Opacity:       1,
	Tint:          color.RGBA{R: 255, G: 255, B: 255, A: 255},
	Animation: Animation{
		Edge:          RandomEdge,
		VisibleDepth:  .86,
		EdgeOffset:    .72,
		StartDelay:    .4,
		EnterDuration: .35,
		HoldDuration:  2.5,
		ExitDuration:  .35,
		Fade:          true,
	},
}

// JabbaSlidePresentation is a development-only override retained for visual
// comparison. Normal Jabba appearances use JabbaPortrait's hologram setting.
var JabbaSlidePresentation = func() Presentation {
	presentation := DefaultSlidePresentation
	presentation.Audio = Audio{Asset: "jabba-laugh", Volume: 1, Policy: FinishNaturally}
	return presentation
}()

var R2D2SlidePresentation = func() Presentation {
	presentation := DefaultSlidePresentation
	presentation.Audio = Audio{Asset: "r2d2-1", Volume: 1, Policy: FinishNaturally}
	return presentation
}()

// JabbaHologramPresentation materializes Jabba upright at a fixed location.
// It shares the normal portrait and sound configuration without modifying them.
var JabbaHologramPresentation = Presentation{
	Effect:        EffectHologram,
	Anchor:        LowerRight,
	InsetX:        22,
	InsetY:        16,
	WidthFraction: .30,
	Scale:         1,
	Opacity:       .9,
	Tint:          color.RGBA{R: 100, G: 220, B: 255, A: 255},
	Animation:     Animation{Edge: FromBottom, StartDelay: .4, EnterDuration: .65, HoldDuration: 2.5, ExitDuration: .45, Fade: true},
	Audio:         Audio{Asset: "jabba-laugh", Volume: 1, Policy: FinishNaturally},
	Hologram: Hologram{
		Tint: color.RGBA{R: 100, G: 220, B: 255, A: 255}, EnteringFlicker: .26, HoldingFlicker: .04,
		ExitingFlicker: .30, StripCount: defaultHologramStrips, MaxDisplacement: .032, InterferenceHz: 16,
	},
}

// CharacterPortrait owns the presentation selected by ordinary callers. An
// image asset is descriptive; cached image registration remains keyed by ID.
type CharacterPortrait struct {
	ID           ID
	ImageAsset   string
	Presentation Presentation
}

var (
	JabbaPortrait     = CharacterPortrait{ID: Jabba, ImageAsset: "jabba-overlay-transparent.png", Presentation: JabbaHologramPresentation}
	R2D2Portrait      = CharacterPortrait{ID: R2D2, ImageAsset: "r2-d2-overlay-transparent.png", Presentation: R2D2SlidePresentation}
	ChewbaccaPortrait = CharacterPortrait{ID: Chewbacca, ImageAsset: "chewbacca-overlay-transparent.png", Presentation: DefaultSlidePresentation}
	HanSoloPortrait   = CharacterPortrait{ID: HanSolo, ImageAsset: "han-solo-overlay-transparent.png", Presentation: DefaultSlidePresentation}
)

// Rect is a floating-point screen-space rectangle.
type Rect struct {
	X, Y          float64
	Width, Height float64
}

// Frame is the draw-only snapshot of an active overlay.
type Frame struct {
	Image           *ebiten.Image
	Rect            Rect
	UnrotatedWidth  float64
	UnrotatedHeight float64
	Rotation        float64
	Edge            Edge
	Opacity         float32
	Tint            color.RGBA
	Effect          Effect
	StripCount      int
	StripOffsets    [maxHologramStrips]float64
	MaxDisplacement float64
}

type portrait struct {
	image *ebiten.Image
}

type activeOverlay struct {
	id                  ID
	presentation        Presentation
	edge                Edge
	state               State
	elapsed             float64
	startDelay          float64
	audioElapsed        float64
	audioFired          bool
	audioEvent          AudioEvent
	hasAudioEvent       bool
	instanceSeed        uint64
	interferenceElapsed float64
	interferenceEpoch   uint64
	stripOffsets        [maxHologramStrips]float64
}

// Manager owns the cached images and, by design, permits only one active
// portrait. Showing a second configured portrait explicitly replaces the first.
type Manager struct {
	portraits  map[ID]portrait
	characters map[ID]CharacterPortrait
	active     activeOverlay
	selector   uint64
	sequence   uint64
}

const defaultSelectorSeed = 0x6a09e667f3bcc909

// New returns an empty portrait manager.
func New() *Manager {
	return NewWithSeed(defaultSelectorSeed)
}

// NewWithSeed creates a manager with a deterministic presentation-only edge
// sequence. The selector never participates in gameplay simulation state.
func NewWithSeed(seed uint64) *Manager {
	if seed == 0 {
		seed = defaultSelectorSeed
	}
	return &Manager{portraits: make(map[ID]portrait), characters: make(map[ID]CharacterPortrait), selector: seed}
}

// RegisterCharacter adds or replaces a character-owned presentation
// definition. It is valid to register a definition before its artwork arrives.
func (manager *Manager) RegisterCharacter(character CharacterPortrait) error {
	if manager == nil {
		return fmt.Errorf("register character %q: nil manager", character.ID)
	}
	if character.ID == "" {
		return fmt.Errorf("register character: empty id")
	}
	if !validPresentation(character.Presentation) {
		return fmt.Errorf("register character %q: invalid presentation", character.ID)
	}
	manager.characters[character.ID] = character
	return nil
}

// Character returns a configured character definition, including definitions
// whose image asset is not yet available.
func (manager *Manager) Character(id ID) (CharacterPortrait, bool) {
	if manager == nil {
		return CharacterPortrait{}, false
	}
	character, found := manager.characters[id]
	return character, found
}

// RegisterPNG decodes a portrait once and stores its Ebitengine image. Invalid
// or missing data is reported to the caller and never becomes an active asset.
func (manager *Manager) RegisterPNG(id ID, encoded []byte) error {
	if manager == nil {
		return fmt.Errorf("register portrait %q: nil manager", id)
	}
	if id == "" {
		return fmt.Errorf("register portrait: empty id")
	}
	if len(encoded) == 0 {
		return fmt.Errorf("register portrait %q: PNG data is missing", id)
	}
	decoded, err := png.Decode(bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("decode portrait %q: %w", id, err)
	}
	manager.portraits[id] = portrait{image: ebiten.NewImageFromImage(decoded)}
	return nil
}

// Show starts a configured portrait in its entering state. A request for an
// unknown portrait is ignored, leaving any current portrait unchanged.
func (manager *Manager) Show(id ID, presentation Presentation) bool {
	return manager.ShowFromEdge(id, presentation, presentation.Animation.Edge)
}

// ShowConfigured resolves the character-owned presentation before starting it.
// It is the normal gameplay-facing manager API.
func (manager *Manager) ShowConfigured(id ID) bool {
	character, found := manager.Character(id)
	if !found {
		return false
	}
	return manager.ShowFromEdge(id, character.Presentation, character.Presentation.Animation.Edge)
}

// ShowConfiguredFromEdge is a development/scripted override that preserves a
// character's own presentation while forcing only a slide edge.
func (manager *Manager) ShowConfiguredFromEdge(id ID, edge Edge) bool {
	character, found := manager.Character(id)
	if !found {
		return false
	}
	return manager.ShowFromEdge(id, character.Presentation, edge)
}

// ShowFromEdge starts a portrait with a caller-selected edge. Passing
// RandomEdge consumes exactly one value from the manager's cosmetic selector.
func (manager *Manager) ShowFromEdge(id ID, presentation Presentation, edge Edge) bool {
	if manager == nil || !validPresentation(presentation) {
		return false
	}
	asset, found := manager.portraits[id]
	if !found || asset.image == nil {
		return false
	}
	if presentation.Effect == EffectHologram {
		edge = FromBottom
	} else if edge == RandomEdge {
		edge = manager.nextEdge()
	}
	if !isEntranceEdge(edge) {
		return false
	}
	manager.sequence++
	manager.active = activeOverlay{
		id:           id,
		presentation: presentation,
		edge:         edge,
		state:        Entering,
		startDelay:   presentation.Animation.StartDelay,
		instanceSeed: hashPresentation(manager.sequence),
	}
	manager.refreshInterference()
	manager.queueAudioIfDue()
	return true
}

// Edge reports the resolved entrance edge for the active portrait.
func (manager *Manager) Edge() (Edge, bool) {
	if manager == nil || manager.active.state == Hidden {
		return FromBottom, false
	}
	return manager.active.edge, true
}

// Cancel immediately hides the active portrait.
func (manager *Manager) Cancel() {
	if manager != nil {
		manager.active = activeOverlay{}
	}
}

// State reports the active lifecycle state.
func (manager *Manager) State() State {
	if manager == nil {
		return Hidden
	}
	return manager.active.state
}

// Active reports whether a portrait is visible or in its entrance/exit path.
func (manager *Manager) Active() bool {
	return manager.State() != Hidden
}

// ConsumeAudioEvent returns the next one-shot audio request, if any. Drawing
// never consumes or creates events, so render frequency cannot affect audio.
func (manager *Manager) ConsumeAudioEvent() (AudioEvent, bool) {
	if manager == nil || !manager.active.hasAudioEvent {
		return AudioEvent{}, false
	}
	event := manager.active.audioEvent
	manager.active.hasAudioEvent = false
	return event, true
}

// Update advances only presentation state. Draw and Frame remain pure reads.
func (manager *Manager) Update(seconds float64) {
	if manager == nil || manager.active.state == Hidden || seconds <= 0 {
		return
	}
	remaining := seconds
	for remaining > 0 && manager.active.state != Hidden {
		if manager.active.startDelay > 0 {
			step := math.Min(remaining, manager.active.startDelay)
			manager.active.startDelay -= step
			remaining -= step
			manager.queueAudioIfDue()
			continue
		}
		duration := manager.stateDuration()
		if duration <= 0 {
			manager.advanceState()
			continue
		}
		step := math.Min(remaining, duration-manager.active.elapsed)
		manager.active.elapsed += step
		manager.active.audioElapsed += step
		manager.advanceInterference(step)
		remaining -= step
		manager.queueAudioIfDue()
		if manager.active.elapsed >= duration {
			manager.advanceState()
		}
	}
}

// Frame returns the current image placement for the supplied viewport. A false
// result means there is nothing to draw. It intentionally does not mutate the
// manager, keeping render passes side-effect free.
func (manager *Manager) Frame(viewportWidth, viewportHeight int) (Frame, bool) {
	if manager == nil || manager.active.state == Hidden || manager.active.startDelay > 0 || viewportWidth <= 0 || viewportHeight <= 0 {
		return Frame{}, false
	}
	asset, found := manager.portraits[manager.active.id]
	if !found || asset.image == nil {
		return Frame{}, false
	}
	bounds := asset.image.Bounds()
	var position Rect
	var rawWidth, rawHeight float64
	var opacity float32
	if manager.active.presentation.Effect == EffectHologram {
		position = RestingRect(viewportWidth, viewportHeight, bounds.Dx(), bounds.Dy(), manager.active.presentation)
		rawWidth, rawHeight = position.Width, position.Height
		opacity = hologramOpacity(manager.active)
	} else {
		rest, width, height := edgeRestingRect(viewportWidth, viewportHeight, bounds.Dx(), bounds.Dy(), manager.active.presentation, manager.active.edge)
		position, opacity = animatedFrame(rest, viewportWidth, viewportHeight, manager.active)
		rawWidth, rawHeight = width, height
	}
	tint := manager.active.presentation.Tint
	if manager.active.presentation.Effect == EffectHologram {
		tint = manager.active.presentation.Hologram.Tint
	}
	return Frame{
		Image: asset.image, Rect: position, UnrotatedWidth: rawWidth, UnrotatedHeight: rawHeight,
		Rotation: RotationForEdge(manager.active.edge), Edge: manager.active.edge,
		Opacity: opacity, Tint: tint, Effect: manager.active.presentation.Effect,
		StripCount: hologramStripCount(manager.active.presentation.Hologram), StripOffsets: manager.active.stripOffsets,
		MaxDisplacement: position.Width * manager.active.presentation.Hologram.MaxDisplacement * hologramInterference(manager.active),
	}, true
}

// Draw composites the current frame. Ebitengine clips offscreen positions to
// the destination image, so entrance and exit require no temporary surfaces.
func (manager *Manager) Draw(screen *ebiten.Image) {
	if screen == nil {
		return
	}
	frame, visible := manager.Frame(screen.Bounds().Dx(), screen.Bounds().Dy())
	if !visible {
		return
	}
	if frame.Effect == EffectHologram {
		drawHologram(screen, frame)
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	bounds := frame.Image.Bounds()
	options.GeoM.Scale(frame.UnrotatedWidth/float64(bounds.Dx()), frame.UnrotatedHeight/float64(bounds.Dy()))
	options.GeoM.Rotate(frame.Rotation)
	minX, minY, _, _ := rotatedBounds(frame.UnrotatedWidth, frame.UnrotatedHeight, frame.Rotation)
	options.GeoM.Translate(frame.Rect.X-minX, frame.Rect.Y-minY)
	options.ColorScale.Scale(
		float32(frame.Tint.R)/255,
		float32(frame.Tint.G)/255,
		float32(frame.Tint.B)/255,
		float32(frame.Tint.A)/255*frame.Opacity,
	)
	screen.DrawImage(frame.Image, options)
}

var hologramStripIndices = [...]uint16{0, 1, 2, 1, 3, 2}

func drawHologram(screen *ebiten.Image, frame Frame) {
	bounds := frame.Image.Bounds()
	options := &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear}
	red := float32(frame.Tint.R) / 255
	green := float32(frame.Tint.G) / 255
	blue := float32(frame.Tint.B) / 255
	alpha := float32(frame.Tint.A) / 255 * frame.Opacity
	for index := 0; index < frame.StripCount; index++ {
		top := bounds.Dy() * index / frame.StripCount
		bottom := bounds.Dy() * (index + 1) / frame.StripCount
		stripHeight := frame.Rect.Height * float64(bottom-top) / float64(bounds.Dy())
		stripY := frame.Rect.Y + frame.Rect.Height*float64(top)/float64(bounds.Dy())
		stripX := frame.Rect.X + frame.StripOffsets[index]*frame.MaxDisplacement
		stripX = math.Max(0, math.Min(stripX, float64(screen.Bounds().Dx())-frame.Rect.Width))
		vertices := [4]ebiten.Vertex{
			{DstX: float32(stripX), DstY: float32(stripY), SrcX: 0, SrcY: float32(top), ColorR: red, ColorG: green, ColorB: blue, ColorA: alpha},
			{DstX: float32(stripX + frame.Rect.Width), DstY: float32(stripY), SrcX: float32(bounds.Dx()), SrcY: float32(top), ColorR: red, ColorG: green, ColorB: blue, ColorA: alpha},
			{DstX: float32(stripX), DstY: float32(stripY + stripHeight), SrcX: 0, SrcY: float32(bottom), ColorR: red, ColorG: green, ColorB: blue, ColorA: alpha},
			{DstX: float32(stripX + frame.Rect.Width), DstY: float32(stripY + stripHeight), SrcX: float32(bounds.Dx()), SrcY: float32(bottom), ColorR: red, ColorG: green, ColorB: blue, ColorA: alpha},
		}
		screen.DrawTriangles(vertices[:], hologramStripIndices[:], frame.Image, options)
	}
}

// RestingRect calculates the aspect-preserving, on-screen placement shared by
// every lifecycle state. It is exported for deterministic layout tests.
func RestingRect(viewportWidth, viewportHeight, sourceWidth, sourceHeight int, presentation Presentation) Rect {
	if viewportWidth <= 0 || viewportHeight <= 0 || sourceWidth <= 0 || sourceHeight <= 0 || !validPresentation(presentation) {
		return Rect{}
	}
	width := float64(viewportWidth) * presentation.WidthFraction * presentation.Scale
	width = math.Min(width, float64(viewportWidth))
	height := width * float64(sourceHeight) / float64(sourceWidth)
	if height > float64(viewportHeight) {
		height = float64(viewportHeight)
		width = height * float64(sourceWidth) / float64(sourceHeight)
	}
	x, y := presentation.InsetX, presentation.InsetY
	switch presentation.Anchor {
	case LowerRight:
		x = float64(viewportWidth) - width - presentation.InsetX
		y = float64(viewportHeight) - height - presentation.InsetY
	case LowerLeft:
		y = float64(viewportHeight) - height - presentation.InsetY
	case UpperRight:
		x = float64(viewportWidth) - width - presentation.InsetX
	}
	x = math.Max(0, math.Min(x, float64(viewportWidth)-width))
	y = math.Max(0, math.Min(y, float64(viewportHeight)-height))
	return Rect{X: x, Y: y, Width: width, Height: height}
}

// RotationForEdge returns the fixed portrait rotation for its entrance edge.
// The source PNG's base is its original bottom edge.
func RotationForEdge(edge Edge) float64 {
	switch edge {
	case FromLeft:
		// Side entries are flipped relative to the initial quarter-turn mapping
		// so the portrait faces into the viewport while peeking from that edge.
		return math.Pi / 2
	case FromTop:
		return math.Pi
	case FromRight:
		return -math.Pi / 2
	default:
		return 0
	}
}

// edgeRestingRect fits the original image so both upright and quarter-turn
// orientations remain inside the viewport along their non-entering axis.
func edgeRestingRect(viewportWidth, viewportHeight, sourceWidth, sourceHeight int, presentation Presentation, edge Edge) (Rect, float64, float64) {
	if viewportWidth <= 0 || viewportHeight <= 0 || sourceWidth <= 0 || sourceHeight <= 0 {
		return Rect{}, 0, 0
	}
	aspect := float64(sourceWidth) / float64(sourceHeight)
	width := float64(viewportWidth) * presentation.WidthFraction * presentation.Scale
	maxWidth := math.Min(
		math.Min(float64(viewportWidth), float64(viewportHeight)),
		math.Min(float64(viewportHeight)*aspect, float64(viewportWidth)*aspect),
	)
	width = math.Min(width, maxWidth)
	height := width / aspect
	rotation := RotationForEdge(edge)
	_, _, rotatedWidth, rotatedHeight := rotatedBounds(width, height, rotation)
	depth := visibleDepth(presentation.Animation.VisibleDepth)
	offset := math.Max(0, math.Min(1, presentation.Animation.EdgeOffset))
	x, y := 0.0, 0.0
	switch edge {
	case FromLeft:
		x = -(1 - depth) * rotatedWidth
		y = (float64(viewportHeight) - rotatedHeight) * offset
	case FromTop:
		x = (float64(viewportWidth) - rotatedWidth) * offset
		y = -(1 - depth) * rotatedHeight
	case FromRight:
		x = float64(viewportWidth) - depth*rotatedWidth
		y = (float64(viewportHeight) - rotatedHeight) * offset
	default: // FromBottom
		x = (float64(viewportWidth) - rotatedWidth) * offset
		y = float64(viewportHeight) - depth*rotatedHeight
	}
	return Rect{X: x, Y: y, Width: rotatedWidth, Height: rotatedHeight}, width, height
}

func rotatedBounds(width, height, rotation float64) (minX, minY, boundsWidth, boundsHeight float64) {
	cosine, sine := math.Cos(rotation), math.Sin(rotation)
	points := [4][2]float64{{}, {width, 0}, {0, height}, {width, height}}
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, point := range points {
		x := cosine*point[0] - sine*point[1]
		y := sine*point[0] + cosine*point[1]
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
	}
	return minX, minY, maxX - minX, maxY - minY
}

func visibleDepth(depth float64) float64 {
	if depth <= 0 {
		return 1
	}
	return math.Max(0, math.Min(1, depth))
}

func (manager *Manager) stateDuration() float64 {
	switch manager.active.state {
	case Entering:
		return manager.active.presentation.Animation.EnterDuration
	case Holding:
		return manager.active.presentation.Animation.HoldDuration
	case Exiting:
		return manager.active.presentation.Animation.ExitDuration
	default:
		return 0
	}
}

func (manager *Manager) advanceState() {
	manager.active.elapsed = 0
	switch manager.active.state {
	case Entering:
		manager.active.state = Holding
	case Holding:
		manager.active.state = Exiting
	case Exiting:
		manager.active = activeOverlay{}
	}
}

func (manager *Manager) queueAudioIfDue() {
	if manager.active.audioFired || manager.active.startDelay > 0 || manager.active.presentation.Audio.Asset == "" || manager.active.audioElapsed < manager.active.presentation.Audio.Delay {
		return
	}
	manager.active.audioFired = true
	manager.active.audioEvent = AudioEvent{
		Asset:  manager.active.presentation.Audio.Asset,
		Volume: manager.active.presentation.Audio.Volume,
		Policy: manager.active.presentation.Audio.Policy,
	}
	manager.active.hasAudioEvent = true
}

func (manager *Manager) advanceInterference(seconds float64) {
	if manager.active.presentation.Effect != EffectHologram || seconds <= 0 {
		return
	}
	hertz := hologramHz(manager.active.presentation.Hologram)
	manager.active.interferenceElapsed += seconds
	interval := 1 / hertz
	for manager.active.interferenceElapsed >= interval {
		manager.active.interferenceElapsed -= interval
		manager.active.interferenceEpoch++
		manager.refreshInterference()
	}
}

func (manager *Manager) refreshInterference() {
	if manager.active.presentation.Effect != EffectHologram {
		return
	}
	count := hologramStripCount(manager.active.presentation.Hologram)
	for index := 0; index < count; index++ {
		value := presentationUnit(manager.active.instanceSeed ^ manager.active.interferenceEpoch*0x9e3779b97f4a7c15 ^ uint64(index+1)*0xbf58476d1ce4e5b9)
		// Roughly a third of strips remain aligned in each burst.
		if value < .34 {
			manager.active.stripOffsets[index] = 0
			continue
		}
		manager.active.stripOffsets[index] = (value*2 - 1)
	}
}

// HologramEnvelope returns the phase opacity before the bounded flicker term.
func HologramEnvelope(state State, elapsed float64, animation Animation) float64 {
	switch state {
	case Entering:
		return progressFor(elapsed, animation.EnterDuration)
	case Exiting:
		return 1 - progressFor(elapsed, animation.ExitDuration)
	case Holding:
		return 1
	default:
		return 0
	}
}

func hologramOpacity(active activeOverlay) float32 {
	envelope := HologramEnvelope(active.state, active.elapsed, active.presentation.Animation)
	flicker := active.presentation.Hologram.HoldingFlicker
	switch active.state {
	case Entering:
		flicker = active.presentation.Hologram.EnteringFlicker
	case Exiting:
		flicker = active.presentation.Hologram.ExitingFlicker
	}
	noise := presentationUnit(active.instanceSeed^active.interferenceEpoch*0x94d049bb133111eb)*2 - 1
	return float32(math.Max(0, math.Min(1, envelope*(1+noise*flicker)*float64(active.presentation.Opacity))))
}

func hologramInterference(active activeOverlay) float64 {
	switch active.state {
	case Entering:
		return 1 - progressFor(active.elapsed, active.presentation.Animation.EnterDuration)
	case Exiting:
		return progressFor(active.elapsed, active.presentation.Animation.ExitDuration)
	default:
		return .08
	}
}

func hologramStripCount(config Hologram) int {
	count := config.StripCount
	if count == 0 {
		return defaultHologramStrips
	}
	return min(max(count, 8), maxHologramStrips)
}

func hologramHz(config Hologram) float64 {
	if config.InterferenceHz <= 0 {
		return 16
	}
	return config.InterferenceHz
}

func hashPresentation(value uint64) uint64 {
	value += 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}

func presentationUnit(value uint64) float64 {
	return float64(hashPresentation(value)>>11) * (1.0 / (1 << 53))
}

func animatedFrame(rest Rect, viewportWidth, viewportHeight int, active activeOverlay) (Rect, float32) {
	start := offscreenRect(rest, viewportWidth, viewportHeight, active.edge)
	opacity := active.presentation.Opacity
	duration := 0.0
	progress := 1.0
	switch active.state {
	case Entering:
		duration = active.presentation.Animation.EnterDuration
		progress = easeOutCubic(progressFor(active.elapsed, duration))
		rest.X = lerp(start.X, rest.X, progress)
		rest.Y = lerp(start.Y, rest.Y, progress)
		if active.presentation.Animation.Fade {
			opacity *= float32(progress)
		}
	case Exiting:
		duration = active.presentation.Animation.ExitDuration
		progress = easeInCubic(progressFor(active.elapsed, duration))
		rest.X = lerp(rest.X, start.X, progress)
		rest.Y = lerp(rest.Y, start.Y, progress)
		if active.presentation.Animation.Fade {
			opacity *= float32(1 - progress)
		}
	}
	return rest, opacity
}

func offscreenRect(rest Rect, viewportWidth, viewportHeight int, edge Edge) Rect {
	switch edge {
	case FromLeft:
		rest.X = -rest.Width
	case FromTop:
		rest.Y = -rest.Height
	case FromRight:
		rest.X = float64(viewportWidth)
	default:
		rest.Y = float64(viewportHeight)
	}
	return rest
}

func progressFor(elapsed, duration float64) float64 {
	if duration <= 0 {
		return 1
	}
	return math.Max(0, math.Min(1, elapsed/duration))
}

func easeOutCubic(value float64) float64 {
	return 1 - math.Pow(1-value, 3)
}

func easeInCubic(value float64) float64 {
	return value * value * value
}

func lerp(start, end, progress float64) float64 {
	return start + (end-start)*progress
}

func validPresentation(presentation Presentation) bool {
	return presentation.WidthFraction > 0 && presentation.Scale > 0 && presentation.Opacity >= 0 && presentation.Opacity <= 1 &&
		presentation.Effect <= EffectHologram && isValidEdge(presentation.Animation.Edge) && presentation.Animation.StartDelay >= 0 && presentation.Animation.EnterDuration >= 0 && presentation.Animation.HoldDuration >= 0 && presentation.Animation.ExitDuration >= 0 &&
		presentation.Audio.Delay >= 0 && presentation.Audio.Volume >= 0 && presentation.Hologram.EnteringFlicker >= 0 && presentation.Hologram.HoldingFlicker >= 0 && presentation.Hologram.ExitingFlicker >= 0 && presentation.Hologram.MaxDisplacement >= 0
}

func (manager *Manager) nextEdge() Edge {
	manager.selector ^= manager.selector << 13
	manager.selector ^= manager.selector >> 7
	manager.selector ^= manager.selector << 17
	return Edge(manager.selector & 3)
}

func isEntranceEdge(edge Edge) bool {
	return edge >= FromBottom && edge <= FromRight
}

func isValidEdge(edge Edge) bool {
	return isEntranceEdge(edge) || edge == RandomEdge
}

//go:embed assets/jabba-overlay-transparent.png
var jabbaPNG []byte

//go:embed assets/r2d2-vector-overlay-transparent.png
var r2d2PNG []byte

var (
	defaultJabbaOnce  sync.Once
	defaultJabbaImage *ebiten.Image
	defaultJabbaErr   error
	defaultR2D2Once   sync.Once
	defaultR2D2Image  *ebiten.Image
	defaultR2D2Err    error
)

// NewDefaultManager loads the built-in portrait set once. The asset is
// embedded, so this never performs filesystem I/O or repeat PNG decoding.
func NewDefaultManager() (*Manager, error) {
	defaultJabbaOnce.Do(func() {
		decoded, err := png.Decode(bytes.NewReader(jabbaPNG))
		if err != nil {
			defaultJabbaErr = fmt.Errorf("decode built-in portrait %q: %w", Jabba, err)
			return
		}
		defaultJabbaImage = ebiten.NewImageFromImage(decoded)
	})
	if defaultJabbaErr != nil {
		return nil, defaultJabbaErr
	}
	defaultR2D2Once.Do(func() {
		decoded, err := png.Decode(bytes.NewReader(r2d2PNG))
		if err != nil {
			defaultR2D2Err = fmt.Errorf("decode built-in portrait %q: %w", R2D2, err)
			return
		}
		defaultR2D2Image = ebiten.NewImageFromImage(decoded)
	})
	if defaultR2D2Err != nil {
		return nil, defaultR2D2Err
	}
	manager := New()
	manager.portraits[Jabba] = portrait{image: defaultJabbaImage}
	manager.portraits[R2D2] = portrait{image: defaultR2D2Image}
	for _, character := range []CharacterPortrait{JabbaPortrait, R2D2Portrait, ChewbaccaPortrait, HanSoloPortrait} {
		if err := manager.RegisterCharacter(character); err != nil {
			return nil, err
		}
	}
	return manager, nil
}
