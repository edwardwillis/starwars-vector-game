package overlay

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
)

func TestLifecycleTransitionsUseElapsedTime(t *testing.T) {
	manager := managerWithTestPortrait(t)
	presentation := testPresentation()
	if !manager.Show("test", presentation) || manager.State() != Entering {
		t.Fatalf("initial state = %v, want entering", manager.State())
	}
	manager.Update(.25)
	if manager.State() != Entering {
		t.Fatalf("state after partial entry = %v, want entering", manager.State())
	}
	manager.Update(.25)
	if manager.State() != Holding {
		t.Fatalf("state after entry = %v, want holding", manager.State())
	}
	manager.Update(1)
	if manager.State() != Exiting {
		t.Fatalf("state after hold = %v, want exiting", manager.State())
	}
	manager.Update(.5)
	if manager.State() != Hidden || manager.Active() {
		t.Fatalf("state after exit = %v active=%t, want hidden", manager.State(), manager.Active())
	}
}

func TestEntryAndExitPositionsFollowConfiguredEdge(t *testing.T) {
	manager := managerWithTestPortrait(t)
	presentation := testPresentation()
	presentation.Animation.Edge = FromBottom
	manager.Show("test", presentation)
	frame, ok := manager.Frame(960, 540)
	if !ok || frame.Rect.Y != 540 {
		t.Fatalf("entry frame=%+v visible=%t, want bottom-clipped at y=540", frame.Rect, ok)
	}
	manager.Update(.5)
	frame, _ = manager.Frame(960, 540)
	rest, _, _ := edgeRestingRect(960, 540, 40, 20, presentation, FromBottom)
	if math.Abs(frame.Rect.Y-rest.Y) > 1e-9 {
		t.Fatalf("holding y=%v, want rest y=%v", frame.Rect.Y, rest.Y)
	}
	manager.Update(1)
	frame, _ = manager.Frame(960, 540)
	if math.Abs(frame.Rect.Y-rest.Y) > 1e-9 {
		t.Fatalf("exit start y=%v, want rest y=%v", frame.Rect.Y, rest.Y)
	}
	manager.Update(.25)
	frame, _ = manager.Frame(960, 540)
	if frame.Rect.Y <= rest.Y || frame.Rect.Y >= 540 {
		t.Fatalf("exit midpoint y=%v, want between %v and 540", frame.Rect.Y, rest.Y)
	}
}

func TestEveryEdgeUsesItsConfiguredRotationAndPath(t *testing.T) {
	presentation := testPresentation()
	presentation.Animation.VisibleDepth = .86
	presentation.Animation.EdgeOffset = .5
	for _, test := range []struct {
		edge     Edge
		rotation float64
	}{
		{FromBottom, 0},
		{FromLeft, math.Pi / 2},
		{FromTop, math.Pi},
		{FromRight, -math.Pi / 2},
	} {
		t.Run(edgeName(test.edge), func(t *testing.T) {
			manager := managerWithTestPortrait(t)
			if !manager.ShowFromEdge("test", presentation, test.edge) {
				t.Fatal("could not show test portrait")
			}
			frame, visible := manager.Frame(960, 540)
			if !visible || frame.Edge != test.edge || math.Abs(frame.Rotation-test.rotation) > 1e-9 {
				t.Fatalf("initial frame=%+v visible=%t", frame, visible)
			}
			switch test.edge {
			case FromBottom:
				if frame.Rect.Y != 540 {
					t.Fatalf("bottom starts at y=%v, want 540", frame.Rect.Y)
				}
			case FromLeft:
				if frame.Rect.X != -frame.Rect.Width {
					t.Fatalf("left starts at x=%v, want %v", frame.Rect.X, -frame.Rect.Width)
				}
			case FromTop:
				if frame.Rect.Y != -frame.Rect.Height {
					t.Fatalf("top starts at y=%v, want %v", frame.Rect.Y, -frame.Rect.Height)
				}
			case FromRight:
				if frame.Rect.X != 960 {
					t.Fatalf("right starts at x=%v, want 960", frame.Rect.X)
				}
			}

			manager.Update(presentation.Animation.EnterDuration)
			frame, _ = manager.Frame(960, 540)
			rest, rawWidth, rawHeight := edgeRestingRect(960, 540, 40, 20, presentation, test.edge)
			if math.Abs(frame.Rect.X-rest.X) > 1e-9 || math.Abs(frame.Rect.Y-rest.Y) > 1e-9 {
				t.Fatalf("rest frame=%+v, want %+v", frame.Rect, rest)
			}
			if math.Abs(frame.UnrotatedWidth/frame.UnrotatedHeight-2) > 1e-9 || frame.UnrotatedWidth != rawWidth || frame.UnrotatedHeight != rawHeight {
				t.Fatalf("rotated frame distorted source: %+v raw=%vx%v", frame, rawWidth, rawHeight)
			}

			manager.Update(presentation.Animation.HoldDuration)
			manager.Update(presentation.Animation.ExitDuration / 2)
			frame, _ = manager.Frame(960, 540)
			switch test.edge {
			case FromBottom:
				if frame.Rect.Y <= rest.Y || frame.Rect.Y >= 540 {
					t.Fatalf("bottom exit y=%v, want between %v and 540", frame.Rect.Y, rest.Y)
				}
			case FromLeft:
				if frame.Rect.X <= -frame.Rect.Width || frame.Rect.X >= rest.X {
					t.Fatalf("left exit x=%v, want between %v and %v", frame.Rect.X, -frame.Rect.Width, rest.X)
				}
			case FromTop:
				if frame.Rect.Y <= -frame.Rect.Height || frame.Rect.Y >= rest.Y {
					t.Fatalf("top exit y=%v, want between %v and %v", frame.Rect.Y, -frame.Rect.Height, rest.Y)
				}
			case FromRight:
				if frame.Rect.X <= rest.X || frame.Rect.X >= 960 {
					t.Fatalf("right exit x=%v, want between %v and 960", frame.Rect.X, rest.X)
				}
			}
		})
	}
}

func TestRandomEdgeSelectionIsDeterministicAndCoversAllEdges(t *testing.T) {
	first, second := managerWithTestPortrait(t), managerWithTestPortrait(t)
	first.selector, second.selector = 91, 91
	presentation := testPresentation()
	presentation.Animation.Edge = RandomEdge
	seen := make(map[Edge]bool)
	for index := 0; index < 32; index++ {
		if !first.Show("test", presentation) || !second.Show("test", presentation) {
			t.Fatal("could not show random portrait")
		}
		one, _ := first.Edge()
		two, _ := second.Edge()
		if one != two {
			t.Fatalf("seeded selection diverged at %d: %v != %v", index, one, two)
		}
		seen[one] = true
	}
	for _, edge := range []Edge{FromBottom, FromLeft, FromTop, FromRight} {
		if !seen[edge] {
			t.Fatalf("random selection never produced %s", edgeName(edge))
		}
	}
}

func TestRotatedPortraitReflowsForViewportChanges(t *testing.T) {
	manager := managerWithTestPortrait(t)
	presentation := testPresentation()
	presentation.Animation.EdgeOffset = .5
	if !manager.ShowFromEdge("test", presentation, FromLeft) {
		t.Fatal("could not show left portrait")
	}
	manager.Update(presentation.Animation.EnterDuration)
	for _, viewport := range [][2]int{{960, 540}, {640, 480}, {320, 180}} {
		frame, visible := manager.Frame(viewport[0], viewport[1])
		if !visible || frame.Rect.Y < 0 || frame.Rect.Y+frame.Rect.Height > float64(viewport[1]) {
			t.Fatalf("viewport=%v frame=%+v visible=%t clips along edge", viewport, frame.Rect, visible)
		}
		if frame.Rect.X+frame.Rect.Width <= 0 || frame.Rect.X >= float64(viewport[0]) {
			t.Fatalf("viewport=%v frame=%+v has no visible peek depth", viewport, frame.Rect)
		}
	}
}

func TestHologramMaterializesUprightAtFixedPosition(t *testing.T) {
	manager := managerWithTestPortrait(t)
	presentation := testPresentation()
	presentation.Effect = EffectHologram
	presentation.Anchor = LowerRight
	presentation.Animation = Animation{Edge: RandomEdge, EnterDuration: .65, HoldDuration: 2.5, ExitDuration: .45}
	presentation.Hologram = Hologram{Tint: color.RGBA{R: 100, G: 220, B: 255, A: 255}, EnteringFlicker: .26, HoldingFlicker: .04, ExitingFlicker: .30, StripCount: 12, MaxDisplacement: .032, InterferenceHz: 16}
	if !manager.ShowFromEdge("test", presentation, FromRight) {
		t.Fatal("could not show hologram")
	}
	initial, visible := manager.Frame(960, 540)
	if !visible || initial.Effect != EffectHologram || initial.Rotation != 0 || initial.Edge != FromBottom || initial.Opacity != 0 {
		t.Fatalf("initial hologram=%+v visible=%t", initial, visible)
	}
	manager.Update(.325)
	middle, _ := manager.Frame(960, 540)
	if middle.Rect != initial.Rect || middle.Rotation != 0 || middle.Opacity <= 0 || middle.Opacity > presentation.Opacity {
		t.Fatalf("materializing hologram changed position/orientation/opacity: initial=%+v middle=%+v", initial, middle)
	}
	manager.Update(.325)
	holding, _ := manager.Frame(960, 540)
	if holding.Rect != initial.Rect || holding.Opacity <= 0 || holding.Opacity > presentation.Opacity || holding.MaxDisplacement >= middle.MaxDisplacement {
		t.Fatalf("holding hologram=%+v", holding)
	}
	manager.Update(presentation.Animation.HoldDuration)
	manager.Update(presentation.Animation.ExitDuration)
	if manager.State() != Hidden {
		t.Fatalf("hologram lifecycle ended in %v, want hidden", manager.State())
	}
}

func TestHologramStripsCoverPortraitWithBoundedDeterministicOffsets(t *testing.T) {
	first, second := managerWithTestPortrait(t), managerWithTestPortrait(t)
	presentation := testPresentation()
	presentation.Effect = EffectHologram
	presentation.Hologram = Hologram{Tint: color.RGBA{R: 100, G: 220, B: 255, A: 255}, StripCount: 12, MaxDisplacement: .04, InterferenceHz: 16}
	for _, manager := range []*Manager{first, second} {
		if !manager.Show("test", presentation) {
			t.Fatal("could not show hologram")
		}
		manager.Update(.2)
	}
	one, _ := first.Frame(960, 540)
	two, _ := second.Frame(960, 540)
	if one.StripCount != 12 || one.StripOffsets != two.StripOffsets || one.MaxDisplacement != two.MaxDisplacement {
		t.Fatalf("deterministic strips diverged: first=%+v second=%+v", one, two)
	}
	covered := 0
	for index := 0; index < one.StripCount; index++ {
		top := one.Image.Bounds().Dy() * index / one.StripCount
		bottom := one.Image.Bounds().Dy() * (index + 1) / one.StripCount
		if top != covered || bottom <= top || math.Abs(one.StripOffsets[index]) > 1 {
			t.Fatalf("strip %d has invalid coverage/offset top=%d bottom=%d covered=%d offset=%v", index, top, bottom, covered, one.StripOffsets[index])
		}
		covered = bottom
	}
	if covered != one.Image.Bounds().Dy() || one.MaxDisplacement > one.Rect.Width*presentation.Hologram.MaxDisplacement+1e-9 {
		t.Fatalf("strip coverage=%d displacement=%v", covered, one.MaxDisplacement)
	}
}

func TestHologramReflowsInsideViewport(t *testing.T) {
	manager := managerWithTestPortrait(t)
	presentation := testPresentation()
	presentation.Effect = EffectHologram
	presentation.Hologram = Hologram{Tint: color.RGBA{R: 100, G: 220, B: 255, A: 255}}
	if !manager.Show("test", presentation) {
		t.Fatal("could not show hologram")
	}
	for _, viewport := range [][2]int{{960, 540}, {640, 480}, {320, 180}} {
		frame, visible := manager.Frame(viewport[0], viewport[1])
		if !visible || frame.Rect.X < 0 || frame.Rect.Y < 0 || frame.Rect.X+frame.Rect.Width > float64(viewport[0]) || frame.Rect.Y+frame.Rect.Height > float64(viewport[1]) {
			t.Fatalf("viewport=%v hologram=%+v visible=%t", viewport, frame.Rect, visible)
		}
	}
}

func TestRestingRectResizesWithoutDistortingAspectRatio(t *testing.T) {
	presentation := testPresentation()
	for _, viewport := range [][2]int{{960, 540}, {640, 480}, {320, 180}} {
		rect := RestingRect(viewport[0], viewport[1], 400, 200, presentation)
		if rect.Width <= 0 || rect.Height <= 0 || math.Abs(rect.Width/rect.Height-2) > 1e-9 {
			t.Fatalf("viewport=%v rect=%+v does not preserve 2:1 aspect", viewport, rect)
		}
		if rect.X+rect.Width > float64(viewport[0])+1e-9 || rect.Y+rect.Height > float64(viewport[1])+1e-9 {
			t.Fatalf("viewport=%v rect=%+v escapes resting bounds", viewport, rect)
		}
	}
}

func TestUnknownOrMissingAssetDoesNotActivateOverlay(t *testing.T) {
	manager := New()
	if err := manager.RegisterPNG("missing", nil); err == nil {
		t.Fatal("missing PNG was accepted")
	}
	if manager.Show("missing", testPresentation()) || manager.Active() {
		t.Fatal("missing portrait activated")
	}
}

func TestDefaultCharacterDefinitionsOwnTheirTransitions(t *testing.T) {
	manager, err := NewDefaultManager()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		id     ID
		effect Effect
	}{
		{Jabba, EffectHologram},
		{R2D2, EffectSlide},
		{Chewbacca, EffectSlide},
		{HanSolo, EffectSlide},
	} {
		character, found := manager.Character(test.id)
		if !found || character.Presentation.Effect != test.effect || character.ImageAsset == "" {
			t.Fatalf("character %q=%+v found=%t, want effect=%v with image asset", test.id, character, found, test.effect)
		}
	}
}

func TestConfiguredCharacterUsesDefinitionAndRejectsMissingArtwork(t *testing.T) {
	manager, err := NewDefaultManager()
	if err != nil {
		t.Fatal(err)
	}
	if !manager.ShowConfigured(Jabba) {
		t.Fatal("configured Jabba did not start")
	}
	manager.Update(JabbaHologramPresentation.Animation.StartDelay + .01)
	frame, visible := manager.Frame(960, 540)
	if !visible || frame.Effect != EffectHologram || frame.Rotation != 0 {
		t.Fatalf("configured Jabba frame=%+v visible=%t", frame, visible)
	}
	if !manager.ShowConfigured(R2D2) {
		t.Fatal("configured R2-D2 did not start after its artwork was registered")
	}
	manager.Update(DefaultSlidePresentation.Animation.StartDelay + .01)
	frame, visible = manager.Frame(960, 540)
	if !visible || frame.Effect != EffectSlide {
		t.Fatalf("configured R2-D2 frame=%+v visible=%t", frame, visible)
	}
	if manager.ShowConfigured(Chewbacca) || manager.ShowConfigured("unknown") {
		t.Fatal("configured character without artwork or definition started")
	}
}

func TestRegisterCharacterRejectsInvalidDefinition(t *testing.T) {
	manager := New()
	if err := manager.RegisterCharacter(CharacterPortrait{}); err == nil {
		t.Fatal("empty character definition was accepted")
	}
	if err := manager.RegisterCharacter(CharacterPortrait{ID: "invalid"}); err == nil {
		t.Fatal("invalid presentation was accepted")
	}
}

func TestAudioEventWaitsForEntranceAndAudioDelays(t *testing.T) {
	manager := managerWithTestPortrait(t)
	presentation := testPresentation()
	presentation.Animation.StartDelay = .4
	presentation.Audio = Audio{Asset: "test-line", Delay: .1, Volume: .7, Policy: StopOnCancel}
	if !manager.Show("test", presentation) {
		t.Fatal("could not show test portrait")
	}
	if _, visible := manager.Frame(960, 540); visible {
		t.Fatal("portrait was visible before entrance delay elapsed")
	}
	if _, pending := manager.ConsumeAudioEvent(); pending {
		t.Fatal("audio event fired before entrance delay elapsed")
	}

	manager.Update(.4)
	if _, visible := manager.Frame(960, 540); !visible {
		t.Fatal("portrait did not enter after entrance delay")
	}
	if _, pending := manager.ConsumeAudioEvent(); pending {
		t.Fatal("audio event fired before audio delay elapsed")
	}

	manager.Update(.1)
	event, pending := manager.ConsumeAudioEvent()
	if !pending || event.Asset != "test-line" || event.Volume != .7 || event.Policy != StopOnCancel {
		t.Fatalf("audio event=%+v pending=%t", event, pending)
	}
	if _, pending := manager.ConsumeAudioEvent(); pending {
		t.Fatal("audio event was emitted more than once")
	}
}

func managerWithTestPortrait(t *testing.T) *Manager {
	t.Helper()
	manager := New()
	image := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	image.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image); err != nil {
		t.Fatal(err)
	}
	if err := manager.RegisterPNG("test", encoded.Bytes()); err != nil {
		t.Fatal(err)
	}
	return manager
}

func testPresentation() Presentation {
	return Presentation{
		Anchor:        LowerRight,
		InsetX:        10,
		InsetY:        10,
		WidthFraction: .25,
		Scale:         1,
		Opacity:       1,
		Tint:          color.RGBA{R: 255, G: 255, B: 255, A: 255},
		Animation:     Animation{Edge: FromBottom, EnterDuration: .5, HoldDuration: 1, ExitDuration: .5, Fade: true},
	}
}

func edgeName(edge Edge) string {
	switch edge {
	case FromBottom:
		return "bottom"
	case FromLeft:
		return "left"
	case FromTop:
		return "top"
	case FromRight:
		return "right"
	default:
		return "random"
	}
}
