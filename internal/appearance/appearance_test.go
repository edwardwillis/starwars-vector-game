package appearance

import (
	"image/color"
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
)

func TestDeathStarArcadeProgressivelyRevealsStableDetail(t *testing.T) {
	billboard := DeathStarArcade().Billboard
	far := billboard.Lines(0)
	near := billboard.Lines(1)
	if len(far) >= len(near) {
		t.Fatalf("far=%d near=%d", len(far), len(near))
	}
	first := billboard.Lines(0.5)
	second := billboard.Lines(0.5)
	if len(first) != len(second) {
		t.Fatal("detail reveal is not deterministic")
	}
	for index := range first {
		if first[index] != second[index] {
			t.Fatal("detail reveal changed between identical inputs")
		}
	}
}

func TestDeathStarArcadeArtworkStaysWithinSphericalSilhouette(t *testing.T) {
	billboard := DeathStarArcade().Billboard
	lines := append(append([]Line(nil), billboard.Base...), billboard.Foreground...)
	lines = append(lines, func() []Line {
		lines := make([]Line, len(billboard.Details))
		for index, detail := range billboard.Details {
			lines[index] = detail.Line
		}
		return lines
	}()...)
	for index, line := range lines {
		for _, point := range []Point{line.A, line.B} {
			if math.Hypot(point.X, point.Y) > 1.000001 {
				t.Fatalf("line %d endpoint %+v lies outside Death Star silhouette", index, point)
			}
		}
	}
}

func TestDeathStarArcadeUsesOpaqueBackingBehindForegroundDish(t *testing.T) {
	billboard := DeathStarArcade().Billboard
	if len(billboard.OpaqueEllipses) != 1 {
		t.Fatalf("dish backings=%d, want 1", len(billboard.OpaqueEllipses))
	}
	if len(billboard.Foreground) == 0 {
		t.Fatal("superlaser dish has no foreground line artwork")
	}
	farDetails := 0
	for _, detail := range billboard.Details {
		if detail.Threshold <= 0 {
			farDetails++
		}
	}
	if got, want := len(billboard.Lines(0)), len(billboard.Base)+len(billboard.Foreground)+farDetails; got != want {
		t.Fatalf("far line count=%d, want base plus dish foreground and far details=%d", got, want)
	}
}

func TestDeathStarArcadeKeepsDishInteriorWithinOuterRim(t *testing.T) {
	billboard := DeathStarArcade().Billboard
	if len(billboard.OpaqueEllipses) != 1 {
		t.Fatalf("dish backings=%d, want 1", len(billboard.OpaqueEllipses))
	}
	const outerRimSegments = 16
	if len(billboard.Foreground) <= outerRimSegments {
		t.Fatalf("dish foreground lines=%d, want outer rim plus inner detail", len(billboard.Foreground))
	}
	rim := billboard.OpaqueEllipses[0]
	cosine, sine := math.Cos(rim.Rotation), math.Sin(rim.Rotation)
	insideRim := func(point Point) bool {
		x, y := point.X-rim.Center.X, point.Y-rim.Center.Y
		u := (x*cosine + y*sine) / rim.RadiusX
		v := (-x*sine + y*cosine) / rim.RadiusY
		return u*u+v*v <= 1+1e-9
	}
	for index, line := range billboard.Foreground[outerRimSegments:] {
		if !insideRim(line.A) || !insideRim(line.B) {
			t.Fatalf("dish inner line %d projects beyond outer rim: %+v", index, line)
		}
	}
}

func TestDeathStarDishLightingDarkensFeedShadow(t *testing.T) {
	rimCenter := Point{X: 0.57, Y: 0.43}
	feedCenter := Point{X: 0.51, Y: 0.38}
	shadowed := deathStarDishLineColor(Point{X: feedCenter.X + 0.12*0.70, Y: feedCenter.Y - 0.12*0.72}, rimCenter, feedCenter, 0.280, 0.180, 0)
	lit := deathStarDishLineColor(Point{X: feedCenter.X - 0.12*0.70, Y: feedCenter.Y + 0.12*0.72}, rimCenter, feedCenter, 0.280, 0.180, 0)
	luminance := func(value color.RGBA) int { return int(value.R) + int(value.G) + int(value.B) }
	if luminance(shadowed) >= luminance(lit) {
		t.Fatalf("feed shadow was not darker: shadowed=%+v lit=%+v", shadowed, lit)
	}
}

func TestDeathStarDishLeadingRimShadowDarkensFarSideOfBowl(t *testing.T) {
	shadowed := deathStarDishLeadingRimShadow(-0.60, -0.40)
	lit := deathStarDishLeadingRimShadow(0.60, 0.40)
	if shadowed <= lit {
		t.Fatalf("leading-rim shadow does not darken far side: shadowed=%v lit=%v", shadowed, lit)
	}
}

func TestDeathStarArcadeKeepsOrbitalLineBudgetBounded(t *testing.T) {
	billboard := DeathStarArcade().Billboard
	if got := len(billboard.Lines(1)); got > 390 {
		t.Fatalf("near orbital Death Star has %d vector lines, want at most 390", got)
	}
}

func TestDeathStarArcadeHasUnshadedServiceLightsOnDarkSide(t *testing.T) {
	billboard := DeathStarArcade().Billboard
	count, darkSide, farVisible := 0, false, 0
	intensities := make(map[color.RGBA]struct{})
	latitudeRows := []float64{-0.70, -0.49, -0.29, 0.28, 0.50, 0.70}
	for _, detail := range billboard.Details {
		if !isDeathStarServiceLightColor(detail.Line.Color) {
			continue
		}
		count++
		intensities[detail.Line.Color] = struct{}{}
		centre := midpoint(detail.Line.A, detail.Line.B)
		darkSide = darkSide || centre.X < 0
		if detail.Threshold <= 0 {
			farVisible++
		}
		for _, latitude := range latitudeRows {
			if math.Abs(centre.Y-latitude) < 0.01 {
				t.Fatalf("service light lies on latitude seam: %+v", detail.Line)
			}
		}
	}
	if count < 30 || !darkSide || farVisible < 2 || len(intensities) < 3 {
		t.Fatalf("service lights=%d dark-side=%t far-visible=%d intensity-variants=%d", count, darkSide, farVisible, len(intensities))
	}
}

func TestDeathStarServiceLightClusterSizesAreStableAndVaried(t *testing.T) {
	sizes := make(map[int]bool)
	for index := 0; index < 12; index++ {
		size := deathStarServiceLightClusterSize(index)
		if size < 2 || size > 5 {
			t.Fatalf("cluster %d has %d lights, want 2..5", index, size)
		}
		sizes[size] = true
	}
	if len(sizes) < 2 {
		t.Fatalf("service-light clusters are not varied: %v", sizes)
	}
}

func TestDeathStarArcadeTrenchIsSymmetricAboutEquator(t *testing.T) {
	for _, x := range []float64{-0.9, -0.4, 0, 0.4, 0.9} {
		upper, lower := deathStarTrenchPoint(x, true), deathStarTrenchPoint(x, false)
		if upper.X != lower.X || math.Abs(upper.Y+lower.Y) > 1e-12 {
			t.Fatalf("trench at x=%v is not symmetric: upper=%+v lower=%+v", x, upper, lower)
		}
	}
}

func TestDeathStarTrenchRemainsBroadlyParallelAtTheLimb(t *testing.T) {
	centre := deathStarTrenchPoint(0, true).Y
	nearLimb := deathStarTrenchPoint(0.95, true).Y
	if nearLimb/centre < 0.8 {
		t.Fatalf("trench tapers too strongly: centre=%v near-limb=%v", centre, nearLimb)
	}
}

func TestDeathStarOutlineHasSymmetricEquatorialTrenchNotches(t *testing.T) {
	outline := deathStarOutline()
	hasFloor := func(x float64) bool {
		for index, point := range outline {
			next := outline[(index+1)%len(outline)]
			if math.Abs(point.X-x) < 1e-12 && math.Abs(next.X-x) < 1e-12 &&
				math.Abs(math.Abs(point.Y)-0.016) < 1e-12 &&
				math.Abs(math.Abs(next.Y)-0.016) < 1e-12 && point.Y*next.Y < 0 {
				return true
			}
		}
		return false
	}
	if !hasFloor(0.973) || !hasFloor(-0.973) {
		t.Fatalf("outline lacks symmetric trench notches: %+v", outline)
	}
}

func TestDeathStarSurfaceProjectionNarrowsPanelsTowardTheLimb(t *testing.T) {
	widthAt := func(latitude float64) float64 {
		left := deathStarSurfacePoint(-0.45, latitude)
		right := deathStarSurfacePoint(0.45, latitude)
		return right.X - left.X
	}
	if !(widthAt(0.75) < widthAt(0.15)) {
		t.Fatalf("spherical panel projection did not narrow toward limb: high=%v near-equator=%v", widthAt(0.75), widthAt(0.15))
	}
}

func TestDeathStarPolarBeltsBowTowardTheirPoles(t *testing.T) {
	northCenter := deathStarPolarBeltPoint(0.78, 0, 0)
	northSide := deathStarPolarBeltPoint(0.78, 0, 0.9)
	if northCenter.Y <= northSide.Y {
		t.Fatalf("north polar belt did not bow poleward: center=%+v side=%+v", northCenter, northSide)
	}
	southCenter := deathStarPolarBeltPoint(-0.78, 0, 0)
	southSide := deathStarPolarBeltPoint(-0.78, 0, 0.9)
	if southCenter.Y >= southSide.Y {
		t.Fatalf("south polar belt did not bow poleward: center=%+v side=%+v", southCenter, southSide)
	}
}

func TestDeathStarSurfacePhongLeavesOccludedSideNearBlack(t *testing.T) {
	occluded := deathStarSurfaceLineColor(Point{X: -0.85, Y: -0.35})
	lit := deathStarSurfaceLineColor(Point{X: 0.40, Y: 0.10})
	if occluded.R > 16 || occluded.G > 24 || occluded.B > 24 {
		t.Fatalf("occluded surface line is too bright: %+v", occluded)
	}
	if int(lit.R)+int(lit.G)+int(lit.B) <= int(occluded.R)+int(occluded.G)+int(occluded.B) {
		t.Fatalf("lit surface line did not exceed occluded line: lit=%+v occluded=%+v", lit, occluded)
	}
}

func TestDeathStarSurfacePhongFavoursExposedRightHemisphere(t *testing.T) {
	centre := deathStarSurfaceLineColor(Point{})
	right := deathStarSurfaceLineColor(Point{X: 0.70, Y: 0})
	luminance := func(value color.RGBA) int { return int(value.R) + int(value.G) + int(value.B) }
	if luminance(right) <= luminance(centre) {
		t.Fatalf("right hemisphere is not brighter than centre: right=%+v centre=%+v", right, centre)
	}
}

func TestClipLineOutsideRotatedEllipseRemovesDishInterior(t *testing.T) {
	line := Line{A: Point{X: 0.12, Y: 0.42}, B: Point{X: 0.90, Y: 0.42}}
	clipped := clipLineOutsideRotatedEllipse(line, Point{X: 0.52, Y: 0.42}, 0.225, 0.145, 0)
	if len(clipped) != 2 {
		t.Fatalf("clipped portions=%d, want 2: %+v", len(clipped), clipped)
	}
	for _, portion := range clipped {
		middle := midpoint(portion.A, portion.B)
		normalized := (middle.X - 0.52) / 0.225
		if normalized*normalized < 1 {
			t.Fatalf("dish-interior segment survived clipping: %+v", portion)
		}
	}
}

func TestClipLineInsideRotatedEllipseRemovesDishExterior(t *testing.T) {
	line := Line{A: Point{X: 0.12, Y: 0.42}, B: Point{X: 0.90, Y: 0.42}}
	clipped := clipLineInsideRotatedEllipse(line, Point{X: 0.52, Y: 0.42}, 0.225, 0.145, 0)
	if len(clipped) != 1 {
		t.Fatalf("clipped portions=%d, want 1: %+v", len(clipped), clipped)
	}
	middle := midpoint(clipped[0].A, clipped[0].B)
	normalized := (middle.X - 0.52) / 0.225
	if normalized*normalized > 1 {
		t.Fatalf("dish-exterior segment survived clipping: %+v", clipped[0])
	}
}

func TestClipLineOutsideConvexPolygonReservesSurfaceModule(t *testing.T) {
	line := Line{A: Point{X: 0, Y: 0.5}, B: Point{X: 1, Y: 0.5}}
	module := []Point{{X: 0.4, Y: 0.4}, {X: 0.6, Y: 0.4}, {X: 0.6, Y: 0.6}, {X: 0.4, Y: 0.6}}
	clipped := clipLineOutsideConvexPolygon(line, module)
	if len(clipped) != 2 {
		t.Fatalf("clipped portions=%d, want 2: %+v", len(clipped), clipped)
	}
	for _, portion := range clipped {
		middle := midpoint(portion.A, portion.B)
		if middle.X > 0.4 && middle.X < 0.6 {
			t.Fatalf("surface detail survived module clipping: %+v", portion)
		}
	}
}

func TestClipLineInsideConvexPolygonConfinesIngressDetail(t *testing.T) {
	line := Line{A: Point{X: 0, Y: 0.5}, B: Point{X: 1, Y: 0.5}}
	opening := []Point{{X: 0.4, Y: 0.4}, {X: 0.6, Y: 0.4}, {X: 0.6, Y: 0.6}, {X: 0.4, Y: 0.6}}
	clipped := clipLineInsideConvexPolygon(line, opening)
	if len(clipped) != 1 {
		t.Fatalf("clipped portions=%d, want 1: %+v", len(clipped), clipped)
	}
	middle := midpoint(clipped[0].A, clipped[0].B)
	if middle.X < 0.4 || middle.X > 0.6 || middle.Y < 0.4 || middle.Y > 0.6 {
		t.Fatalf("ingress detail escaped opening: %+v", clipped[0])
	}
}

func TestDeathStarIngressBackWallMovesTowardStationCentre(t *testing.T) {
	outer, inner := deathStarSurfaceIngressGeometry(deathStarSurfaceModules()[0])
	centre := func(points []Point) Point {
		result := Point{}
		for _, point := range points {
			result.X += point.X
			result.Y += point.Y
		}
		result.X /= float64(len(points))
		result.Y /= float64(len(points))
		return result
	}
	outerCentre, innerCentre := centre(outer), centre(inner)
	towardCentre := Point{X: -outerCentre.X, Y: -outerCentre.Y}
	backWallOffset := Point{X: innerCentre.X - outerCentre.X, Y: innerCentre.Y - outerCentre.Y}
	if backWallOffset.X*towardCentre.X+backWallOffset.Y*towardCentre.Y <= 0 {
		t.Fatalf("ingress back wall did not move toward station centre: outer=%+v inner=%+v", outerCentre, innerCentre)
	}
}

func TestDeathStarArcadeUsesSmoothOrbitalApproachScale(t *testing.T) {
	billboard := DeathStarArcade().Billboard
	far := billboard.RadiusScale(billboard.FarDepth + 100)
	middle := billboard.RadiusScale((billboard.NearDepth + billboard.FarDepth) / 2)
	near := billboard.RadiusScale(billboard.NearDepth - 1)
	if far != billboard.FarScale || !(far < middle && middle < near) || near != 1 {
		t.Fatalf("approach scale far=%v middle=%v near=%v, config=%+v", far, middle, near, billboard)
	}
	if plain := (Billboard{}).RadiusScale(1000); plain != 1 {
		t.Fatalf("ordinary billboard scale=%v, want 1", plain)
	}
}

func TestAppearanceRegistrySelectsByLogicalObject(t *testing.T) {
	definition, ok := DefaultRegistry().ForObject("builtin/death-star", "")
	if !ok || definition.Name != DeathStarArcadeName {
		t.Fatalf("definition=%+v ok=%t", definition, ok)
	}
}

func TestAppearanceRegistryIncludesFactionLaserStyles(t *testing.T) {
	registry := DefaultRegistry()
	for _, name := range []string{catalog.RebelLaserBoltAppearance, catalog.ImperialLaserBoltAppearance} {
		definition, err := registry.Lookup(name)
		if err != nil || definition.ObjectDefinition != catalog.LaserBoltName || definition.Kind != "model-3d" {
			t.Fatalf("laser appearance %q=%+v err=%v", name, definition, err)
		}
	}
}

func TestAppearanceRegistryIncludesTIEInterceptorModel(t *testing.T) {
	definition, err := DefaultRegistry().Lookup(catalog.TIEInterceptorAppearance)
	if err != nil {
		t.Fatal(err)
	}
	if definition.ObjectDefinition != catalog.TIEInterceptorName || definition.Kind != "model-3d" {
		t.Fatalf("definition=%+v", definition)
	}
}
