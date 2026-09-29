package appearance

import (
	"image/color"
	"math"
	"sort"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
)

const DeathStarArcadeName = "builtin/death-star-arcade-billboard"

var deathStarServiceLightColor = color.RGBA{R: 246, G: 132, B: 42, A: 255}

// deathStarServiceLightColorFor makes the authored service lights feel less
// mechanically uniform without introducing per-frame flicker. The small,
// deterministic variation is keyed by the light's placement in the artwork.
func deathStarServiceLightColorFor(index int) color.RGBA {
	seed := deathStarServiceLightSeed(index)
	intensity := 0.76 + 0.24*float64(seed&0xff)/255
	return color.RGBA{
		R: uint8(math.Round(float64(deathStarServiceLightColor.R) * intensity)),
		G: uint8(math.Round(float64(deathStarServiceLightColor.G) * intensity)),
		B: uint8(math.Round(float64(deathStarServiceLightColor.B) * intensity)),
		A: deathStarServiceLightColor.A,
	}
}

func deathStarServiceLightClusterSize(index int) int {
	return 2 + int(deathStarServiceLightSeed(index)&0x3)
}

func deathStarServiceLightSeed(index int) uint32 {
	seed := uint32(index + 1)
	seed ^= seed << 13
	seed ^= seed >> 17
	seed ^= seed << 5
	return seed
}

func isDeathStarServiceLightColor(value color.RGBA) bool {
	return value.A == deathStarServiceLightColor.A &&
		value.R >= 185 && value.R <= deathStarServiceLightColor.R &&
		value.G >= 100 && value.G <= deathStarServiceLightColor.G &&
		value.B >= 32 && value.B <= deathStarServiceLightColor.B &&
		value.R > value.G && value.G > value.B
}

// DeathStarArcade builds one normalized, camera-facing vector drawing. The
// orbital station deliberately remains a cheap billboard: its structure is
// authored to read as curved surface detail, while its physical spherical
// opacity remains the renderer's separate analytic point occluder.
func DeathStarArcade() Definition {
	dish := color.RGBA{R: 100, G: 184, B: 164, A: 255}

	base := make([]Line, 0, 128)
	outline := deathStarOutline()
	for index := range outline {
		firstPoint := outline[index]
		secondPoint := outline[(index+1)%len(outline)]
		base = append(base, Line{A: firstPoint, B: secondPoint, Color: deathStarSurfaceLineColor(midpoint(firstPoint, secondPoint)), Width: 1.5})
	}

	// The equatorial trench is a narrow, continuous band with nearly parallel
	// edges. Small silhouette notches show where it enters the station instead
	// of pinching the whole band into an artificial screen-space wedge.
	const trenchSegments = 12
	for index := 0; index < trenchSegments; index++ {
		x0 := -0.990 + 1.980*float64(index)/trenchSegments
		x1 := -0.990 + 1.980*float64(index+1)/trenchSegments
		upper0, upper1 := deathStarTrenchPoint(x0, true), deathStarTrenchPoint(x1, true)
		lower0, lower1 := deathStarTrenchPoint(x0, false), deathStarTrenchPoint(x1, false)
		base = append(base,
			Line{A: upper0, B: upper1, Color: deathStarTrenchLineColor(midpoint(upper0, upper1)), Width: 1.25},
			Line{A: lower0, B: lower1, Color: deathStarTrenchLineColor(midpoint(lower0, lower1)), Width: 1.25},
		)
	}
	// Fine companion seams frame the trench as a wider equatorial service band.
	// They remain unsegmented and lighter than the trench itself, so the station
	// retains a clean vector silhouette instead of becoming a tiled strip.
	for index := 0; index < trenchSegments; index++ {
		x0 := -0.990 + 1.980*float64(index)/trenchSegments
		x1 := -0.990 + 1.980*float64(index+1)/trenchSegments
		upper0, upper1 := deathStarTrenchBandPoint(x0, true), deathStarTrenchBandPoint(x1, true)
		lower0, lower1 := deathStarTrenchBandPoint(x0, false), deathStarTrenchBandPoint(x1, false)
		base = append(base,
			Line{A: upper0, B: upper1, Color: deathStarTrenchLineColor(midpoint(upper0, upper1)), Width: 0.75},
			Line{A: lower0, B: lower1, Color: deathStarTrenchLineColor(midpoint(lower0, lower1)), Width: 0.75},
		)
	}

	// The dish sits high on the illuminated hemisphere. Successive rings shift
	// inward toward the station centre, giving the vector outline a recessed
	// bowl rather than a flat target symbol.
	// The superlaser is mounted out on the upper-right hemisphere. Its major
	// ellipse axis is tangential to the sphere, while progressively deeper rings
	// shift inward toward the station centre to form a recessed bowl.
	const dishX, dishY = 0.52, 0.33
	const dishOuterRadiusX, dishOuterRadiusY = 0.260, 0.230
	dishCenter := Point{X: dishX, Y: dishY}
	axisLength := math.Hypot(dishX, dishY)
	axisX, axisY := dishX/axisLength, dishY/axisLength
	dishRotation := math.Atan2(dishY, dishX) + math.Pi/2
	ellipsePoint := func(center Point, radiusX, radiusY, angle float64) Point {
		localX, localY := radiusX*math.Cos(angle), radiusY*math.Sin(angle)
		return Point{
			X: center.X + localX*math.Cos(dishRotation) - localY*math.Sin(dishRotation),
			Y: center.Y + localX*math.Sin(dishRotation) + localY*math.Cos(dishRotation),
		}
	}
	// The reflector is authored as a shallow projected paraboloid. Every
	// visible feature samples the same bowl, so the rim, receiver, and seams
	// agree about which way the dish recedes instead of reading as a stack of
	// unrelated screen-space ellipses.
	const dishDepth = 0.048
	bowlCenter := func(radius float64) Point {
		depth := dishDepth * (1 - radius*radius)
		return Point{X: dishX - axisX*depth, Y: dishY - axisY*depth}
	}
	bowlPoint := func(radius, angle float64) Point {
		return ellipsePoint(bowlCenter(radius), dishOuterRadiusX*radius, dishOuterRadiusY*radius, angle)
	}
	deepCenter := bowlCenter(0)
	// The outer lip is allowed to lie on the backing edge. Every inner ring
	// and spoke is clipped to that lip below: the deeper, offset ellipses must
	// never emerge beyond it or they read as surface detail seen through a
	// transparent dish.
	dishLines := make([]Line, 0, 80)
	innerDishLines := make([]Line, 0, 64)
	appendBowlRing := func(lines *[]Line, radius float64, segments int, width float32) {
		for index := 0; index < segments; index++ {
			first := 2 * math.Pi * float64(index) / float64(segments)
			second := 2 * math.Pi * float64(index+1) / float64(segments)
			*lines = append(*lines, Line{A: bowlPoint(radius, first), B: bowlPoint(radius, second), Color: dish, Width: width})
		}
	}
	// A doubled rim gives the aperture a shallow bevel. The broad interval from
	// the inner lip to the receiver is intentionally left mostly open: it reads
	// as one bowl rather than a wheel of closed polygon panels.
	const receiverRadius = 0.39
	const intermediateReceiverRadius = 0.17
	const feedRadius = 0.07
	appendBowlRing(&dishLines, 1, 18, 1.45)
	appendBowlRing(&innerDishLines, 0.92, 16, 0.68)
	appendBowlRing(&innerDishLines, receiverRadius, 16, 1.00)
	appendBowlRing(&innerDishLines, intermediateReceiverRadius, 12, 0.72)
	// The small feed antenna sits at the centre of the nested receiver rings.
	appendBowlRing(&innerDishLines, feedRadius, 10, 0.95)

	// Fine, curved seams contour the same projected bowl. They stop at the
	// receiver rather than forming closed, uniform wedge panels.
	const reflectorSeams = 12
	const seamSegments = 2
	for seam := 0; seam < reflectorSeams; seam++ {
		angle := 2*math.Pi*float64(seam)/reflectorSeams + 0.12
		for segment := 0; segment < seamSegments; segment++ {
			outerRadius := 0.91 - (0.91-(receiverRadius+0.02))*float64(segment)/seamSegments
			innerRadius := 0.91 - (0.91-(receiverRadius+0.02))*float64(segment+1)/seamSegments
			innerDishLines = append(innerDishLines, Line{
				A:     bowlPoint(outerRadius, angle),
				B:     bowlPoint(innerRadius, angle),
				Color: dish, Width: 0.62,
			})
		}
	}
	for _, line := range innerDishLines {
		dishLines = append(dishLines, clipLineInsideRotatedEllipse(line, dishCenter, dishOuterRadiusX, dishOuterRadiusY, dishRotation)...)
	}
	// Shade the already-authored vector geometry rather than filling the bowl.
	// The concave-light approximation and feed shadow retain the sparse vector
	// character while giving the superlaser reflector a recognisable interior.
	for index := range dishLines {
		dishLines[index].Color = deathStarDishLineColor(midpoint(dishLines[index].A, dishLines[index].B), dishCenter, deepCenter, dishOuterRadiusX, dishOuterRadiusY, dishRotation)
	}

	surfaceModules := deathStarSurfaceModules()
	moduleOccluders := make([][]Point, len(surfaceModules))
	for index, module := range surfaceModules {
		moduleOccluders[index] = deathStarSurfaceModulePolygon(module)
	}

	details := make([]Detail, 0, 112)
	appendDetail := func(threshold float64, line Line) {
		details = append(details, Detail{Threshold: threshold, Line: line})
	}
	// Surface seams must not pass through the superlaser. Clip them before the
	// final local opaque backing both to prevent leakage and to avoid submitting
	// detail that the backing would immediately cover.
	appendSurfaceDetail := func(threshold float64, line Line) {
		visibleLines := clipLineOutsideRotatedEllipse(line, dishCenter, dishOuterRadiusX, dishOuterRadiusY, dishRotation)
		for _, module := range moduleOccluders {
			clipped := make([]Line, 0, len(visibleLines)+1)
			for _, visible := range visibleLines {
				clipped = append(clipped, clipLineOutsideConvexPolygon(visible, module)...)
			}
			visibleLines = clipped
		}
		for _, visible := range visibleLines {
			appendDetail(threshold, visible)
		}
	}
	// Modules own their outlines, so their lines must not be clipped by the
	// same masks that reserve their interiors from the surrounding surface grid.
	appendModuleDetail := func(threshold float64, line Line) {
		for _, visible := range clipLineOutsideRotatedEllipse(line, dishCenter, dishOuterRadiusX, dishOuterRadiusY, dishRotation) {
			appendDetail(threshold, visible)
		}
	}
	appendIngressDetail := func(threshold float64, line Line, opening []Point) {
		for _, visible := range clipLineOutsideRotatedEllipse(line, dishCenter, dishOuterRadiusX, dishOuterRadiusY, dishRotation) {
			for _, interior := range clipLineInsideConvexPolygon(visible, opening) {
				appendDetail(threshold, interior)
			}
		}
	}
	// Two restrained latitude belts demarcate the polar caps. They echo the
	// equatorial trench's segmented construction, but sit in the detail layer
	// so the distant station retains its sparse arcade silhouette.
	const polarBandLatitude = 0.82
	const beltHalfHeight = 0.018
	for bandIndex, latitude := range []float64{-polarBandLatitude, polarBandLatitude} {
		const beltSegments = 8
		for segment := 0; segment < beltSegments; segment++ {
			for _, edge := range []float64{-beltHalfHeight, beltHalfHeight} {
				u0 := -0.97 + 1.94*float64(segment)/beltSegments + 0.008
				u1 := -0.97 + 1.94*float64(segment+1)/beltSegments - 0.008
				first, second := deathStarPolarBeltPoint(latitude, edge, u0), deathStarPolarBeltPoint(latitude, edge, u1)
				appendSurfaceDetail(0.18+0.05*float64(bandIndex)+0.012*float64(segment), Line{A: first, B: second, Color: deathStarTrenchLineColor(midpoint(first, second)), Width: 0.78})
			}
			if segment%2 == 0 {
				u := -0.97 + 1.94*float64(segment)/beltSegments
				lower, upper := deathStarPolarBeltPoint(latitude, -beltHalfHeight, u), deathStarPolarBeltPoint(latitude, beltHalfHeight, u)
				appendSurfaceDetail(0.18+0.05*float64(bandIndex)+0.012*float64(segment), Line{A: lower, B: upper, Color: deathStarTrenchLineColor(midpoint(lower, upper)), Width: 0.68})
			}
		}
	}
	// Projected meridians are visibly curved: each coordinate is scaled by the
	// available half-width at that latitude. Gaps at the trench preserve it as
	// a real surface belt instead of drawing a grid through it. Each curve is
	// sampled into short vector segments rather than rendered as one straight
	// chord across the face of the sphere.
	meridian := func(u, y float64) Point {
		return deathStarSurfacePoint(u, y)
	}
	for column, u := range []float64{-0.84, -0.61, -0.35, -0.09, 0.20, 0.48, 0.72} {
		// Each meridian meets the curved inner edge of a polar belt rather than
		// continuing through the cap, giving the bands a structural role in the
		// surface rather than a floating screen-space decoration.
		meridianRanges := [][2]float64{
			{deathStarPolarBeltPoint(-polarBandLatitude, beltHalfHeight, u).Y, -0.18},
			{0.17, deathStarPolarBeltPoint(polarBandLatitude, -beltHalfHeight, u).Y},
		}
		for rangeIndex, span := range meridianRanges {
			for segment := 0; segment < 4; segment++ {
				y0 := span[0] + (span[1]-span[0])*float64(segment)/4
				y1 := span[0] + (span[1]-span[0])*float64(segment+1)/4
				first, second := meridian(u, y0), meridian(u, y1)
				appendSurfaceDetail(0.12+0.045*float64(column+rangeIndex)+0.018*float64(segment), Line{A: first, B: second, Color: deathStarSurfaceLineColor(midpoint(first, second)), Width: 0.8})
			}
		}
	}
	// Latitude rows really do project to horizontal contours head-on. Make
	// them irregularly broken instead of bending them into a false, repeated
	// chevron pattern; the curved meridians carry the volume cue.
	for row, rowSpec := range []struct {
		y        float64
		segments int
		phase    float64
	}{
		{-0.70, 5, 0.13}, {-0.49, 7, 0.05}, {-0.29, 6, 0.18},
		{0.28, 7, 0.09}, {0.50, 6, 0.20}, {0.70, 5, 0.02},
	} {
		y := rowSpec.y
		width := math.Sqrt(1-y*y) * 0.90
		for segment := 0; segment < rowSpec.segments; segment++ {
			start := float64(segment)/float64(rowSpec.segments) + rowSpec.phase/float64(rowSpec.segments)
			end := float64(segment+1)/float64(rowSpec.segments) - (0.024+0.007*float64((segment+row)%3))/width
			if end <= start {
				continue
			}
			x0 := -width + 2*width*start
			x1 := -width + 2*width*end
			first, second := Point{X: x0, Y: y}, Point{X: x1, Y: y}
			threshold := 0.25 + 0.045*float64(row) + 0.018*float64(segment)
			appendSurfaceDetail(threshold, Line{A: first, B: second, Color: deathStarSurfaceLineColor(midpoint(first, second)), Width: 0.72})
		}
	}
	// A handful of warm service lights punctuate the latitude rows. They are
	// deliberately unshaded: unlike reflected hull detail, these are powered
	// sources and should remain visible across the Death Star's dark side.
	for lightIndex, light := range []struct {
		x, y, length, threshold float64
	}{
		{-0.50, -0.62, 0.004, 0.20}, {0.12, -0.62, 0.012, 0.00},
		{-0.62, -0.41, 0.012, 0.32}, {-0.16, -0.41, 0.005, 0.46}, {0.40, -0.41, 0.016, 0.56},
		{-0.55, -0.18, 0.006, 0.38}, {0.10, -0.18, 0.012, 0.52},
		{-0.38, 0.18, 0.014, 0.00}, {0.62, 0.18, 0.004, 0.68},
		{0.05, 0.40, 0.008, 0.60}, {0.47, 0.40, 0.005, 0.74},
		{-0.22, 0.60, 0.010, 0.64},
	} {
		members := deathStarServiceLightClusterSize(lightIndex)
		for memberIndex := 0; memberIndex < members; memberIndex++ {
			position := float64(memberIndex) - float64(members-1)/2
			memberSeed := deathStarServiceLightSeed(lightIndex*8 + memberIndex)
			scale := 0.52 + 0.43*float64((memberSeed>>8)&0xff)/255
			appendSurfaceDetail(min(1, light.threshold+0.045*float64(memberIndex)), Line{
				A:     Point{X: light.x + position*0.020 - light.length*scale/2, Y: light.y},
				B:     Point{X: light.x + position*0.020 + light.length*scale/2, Y: light.y},
				Color: deathStarServiceLightColorFor(lightIndex*8 + memberIndex),
				Width: 1,
			})
		}
	}
	// A few large, fixed surface modules stop the panel network becoming an
	// abstract grid. They are authored in sphere coordinates rather than as
	// screen rectangles: their side edges curve and their apparent widths vary
	// with latitude just as plates on the real station would.
	for index, module := range surfaceModules {
		left, right := module.longitude-module.width/2, module.longitude+module.width/2
		bottom, top := module.latitude-module.height/2, module.latitude+module.height/2
		bottomLeft, bottomRight := deathStarSurfacePoint(left, bottom), deathStarSurfacePoint(right, bottom)
		topLeft, topRight := deathStarSurfacePoint(left, top), deathStarSurfacePoint(right, top)
		threshold := 0.46 + 0.07*float64(index)
		for _, line := range []Line{
			{A: bottomLeft, B: bottomRight, Color: deathStarSurfaceLineColor(midpoint(bottomLeft, bottomRight)), Width: 0.9},
			{A: topLeft, B: topRight, Color: deathStarSurfaceLineColor(midpoint(topLeft, topRight)), Width: 0.9},
		} {
			appendModuleDetail(threshold, line)
		}
		for _, longitude := range []float64{left, right} {
			middle := deathStarSurfacePoint(longitude, module.latitude)
			lower, upper := deathStarSurfacePoint(longitude, bottom), deathStarSurfacePoint(longitude, top)
			appendModuleDetail(threshold, Line{A: lower, B: middle, Color: deathStarSurfaceLineColor(midpoint(lower, middle)), Width: 0.9})
			appendModuleDetail(threshold, Line{A: middle, B: upper, Color: deathStarSurfaceLineColor(midpoint(middle, upper)), Width: 0.9})
		}

		// At close orbital range, reveal an inset back wall shifted toward the
		// station centre. Its return edges then read as an ingress descending into
		// the spherical body rather than a flat concentric printed rectangle.
		outerCorners, innerCorners := deathStarSurfaceIngressGeometry(module)
		ingressThreshold := 0.78 + 0.025*float64(index)
		for corner := range innerCorners {
			next := (corner + 1) % len(innerCorners)
			backWall := Line{A: innerCorners[corner], B: innerCorners[next], Color: deathStarSurfaceLineColor(midpoint(innerCorners[corner], innerCorners[next])), Width: 0.62}
			returnEdge := Line{A: outerCorners[corner], B: innerCorners[corner], Color: deathStarSurfaceLineColor(midpoint(outerCorners[corner], innerCorners[corner])), Width: 0.55}
			appendIngressDetail(ingressThreshold, backWall, outerCorners)
			appendIngressDetail(ingressThreshold, returnEdge, outerCorners)
		}
	}
	return Definition{Name: DeathStarArcadeName, ObjectDefinition: catalog.DeathStarName, Kind: "vector-billboard", PointOccluder: "sphere", Billboard: Billboard{
		Name: DeathStarArcadeName, Base: base, Details: details, Foreground: dishLines,
		OpaqueEllipses: []BillboardEllipse{{
			Center: dishCenter, RadiusX: dishOuterRadiusX, RadiusY: dishOuterRadiusY, Rotation: dishRotation,
			// The opaque backing is deliberately local. The station's wide star
			// occlusion remains analytic and never requires a full-disc fill.
			Color: color.RGBA{R: 5, G: 13, B: 12, A: 255},
		}},
		// The orbital view opens with a readable, distant station rather than a
		// screen-filling disc. Its apparent radius then catches up smoothly as
		// the fighter closes on the established physical approach boundary.
		NearDepth: 360, FarDepth: 650, FarScale: 0.48,
	}}
}

type deathStarSurfaceModule struct {
	longitude float64
	latitude  float64
	width     float64
	height    float64
}

func deathStarSurfaceModules() []deathStarSurfaceModule {
	return []deathStarSurfaceModule{
		{-0.51, 0.34, 0.055, 0.030}, {-0.39, -0.56, 0.070, 0.028},
		{0.19, -0.47, 0.070, 0.028}, {0.45, -0.31, 0.055, 0.026},
		{0.23, 0.69, 0.060, 0.022},
	}
}

func deathStarSurfaceModulePolygon(module deathStarSurfaceModule) []Point {
	left, right := module.longitude-module.width/2, module.longitude+module.width/2
	bottom, top := module.latitude-module.height/2, module.latitude+module.height/2
	return []Point{
		deathStarSurfacePoint(left, bottom),
		deathStarSurfacePoint(right, bottom),
		deathStarSurfacePoint(right, top),
		deathStarSurfacePoint(left, top),
	}
}

// deathStarSurfaceIngressGeometry creates a smaller back wall displaced
// toward the visible Death Star centre. This is an intentional perspective
// cue for the close-detail billboard artwork: ingress direction should be
// radial, not screen-parallel.
func deathStarSurfaceIngressGeometry(module deathStarSurfaceModule) ([]Point, []Point) {
	outer := deathStarSurfaceModulePolygon(module)
	innerModule := module
	innerModule.width *= 0.48
	innerModule.height *= 0.48
	inner := deathStarSurfaceModulePolygon(innerModule)
	centre := Point{}
	for _, point := range outer {
		centre.X += point.X
		centre.Y += point.Y
	}
	centre.X /= float64(len(outer))
	centre.Y /= float64(len(outer))
	distance := math.Hypot(centre.X, centre.Y)
	if distance <= 1e-9 {
		return outer, inner
	}
	span := max(math.Hypot(outer[1].X-outer[0].X, outer[1].Y-outer[0].Y), math.Hypot(outer[3].X-outer[0].X, outer[3].Y-outer[0].Y))
	offset := 0.40 * span
	shiftX, shiftY := -centre.X/distance*offset, -centre.Y/distance*offset
	for index := range inner {
		inner[index].X += shiftX
		inner[index].Y += shiftY
	}
	return outer, inner
}

func deathStarTrenchPoint(x float64, upper bool) Point {
	curvature := math.Sqrt(max(0, 1-(x/0.99)*(x/0.99)))
	// The real equatorial trench remains broadly parallel almost to the limb.
	// Keep only a restrained centre widening as the spherical cue; a strongly
	// tapered band reads as a flat strip pinching across the station.
	halfHeight := 0.013 + 0.003*curvature
	if upper {
		return Point{X: x, Y: halfHeight}
	}
	return Point{X: x, Y: -halfHeight}
}

func deathStarTrenchBandPoint(x float64, upper bool) Point {
	trench := deathStarTrenchPoint(x, upper)
	bandOffset := 0.050
	if upper {
		trench.Y += bandOffset
	} else {
		trench.Y -= bandOffset
	}
	return trench
}

// deathStarDishLineColor is a line-level approximation of concave Phong
// lighting. The bowl receives diffuse/specular light from the upper-left; a
// widening cone from the feed receiver then darkens the far lower-right side,
// approximating the characteristic shadow cast across the real superlaser
// dish without requiring a shaded fill.
func deathStarDishLineColor(point, rimCenter, feedCenter Point, radiusX, radiusY, rotation float64) color.RGBA {
	cosine, sine := math.Cos(rotation), math.Sin(rotation)
	x, y := point.X-rimCenter.X, point.Y-rimCenter.Y
	u := (x*cosine + y*sine) / radiusX
	v := (-x*sine + y*cosine) / radiusY
	radiusSquared := min(1, u*u+v*v)
	// The dish is a shallow bowl, so its apparent normal turns toward the
	// viewer around the centre and away again at the rim.
	normalZ := math.Sqrt(max(0, 1-radiusSquared))
	diffuse := max(0, -0.44*u+0.30*v+0.84*normalZ)
	specular := math.Pow(max(0, -0.18*u+0.12*v+0.98*normalZ), 14)
	intensity := min(1, 0.15+0.68*diffuse+0.25*specular)
	intensity *= 1 - deathStarDishLeadingRimShadow(u, v)

	// The small feed casts a broadening diagonal shadow toward the lower-right
	// of the projected dish. Its edge softens with distance from the receiver,
	// keeping the result in the vocabulary of vector line art rather than a
	// hard painted wedge.
	shadowDirection := Point{X: 0.70, Y: -0.72}
	relative := Point{X: point.X - feedCenter.X, Y: point.Y - feedCenter.Y}
	along := relative.X*shadowDirection.X + relative.Y*shadowDirection.Y
	if along > 0 {
		across := math.Abs(relative.X*shadowDirection.Y - relative.Y*shadowDirection.X)
		halfWidth := 0.020 + 0.62*along
		withinCone := max(0, 1-across/halfWidth)
		shadowRamp := min(1, along/0.055)
		intensity *= 1 - 0.86*withinCone*shadowRamp
	}
	return color.RGBA{
		R: uint8(18 + 105*intensity),
		G: uint8(42 + 180*intensity),
		B: uint8(39 + 166*intensity),
		A: 255,
	}
}

// deathStarDishLeadingRimShadow approximates the broad shadow cast into the
// concave reflector by its leading surface lip. The curved threshold keeps it
// from reading as a flat half-disc: it follows the bowl's ellipse and darkens
// progressively across the bowl's middle-right side.
func deathStarDishLeadingRimShadow(u, v float64) float64 {
	// This local direction maps to the middle-right of the projected bowl after
	// its tangential ellipse rotation has been applied.
	const alongX, alongY = -0.60, -0.80
	along := u*alongX + v*alongY
	across := u*(-alongY) + v*alongX
	// The leading lip's shadow edge bows toward the rim at either side of the
	// bowl, producing a broad curved cast shadow rather than a straight chord.
	boundary := -0.08 + 0.26*across*across
	softness := 0.13
	return 0.58 * min(1, max(0, (along-boundary)/softness))
}

// deathStarOutline is the station limb with shallow symmetric notches at the
// equator. Those notches provide the physical entry/exit points for the
// continuous equatorial trench without implying that the band passes through
// a transparent sphere.
func deathStarOutline() []Point {
	const outlineSegments = 34
	points := make([]Point, 0, outlineSegments+6)
	points = append(points,
		Point{X: 0.973, Y: 0.016},
		Point{X: 0.990, Y: 0.016},
	)
	for index := 1; index < outlineSegments/2; index++ {
		angle := 2 * math.Pi * float64(index) / outlineSegments
		points = append(points, Point{X: math.Cos(angle), Y: math.Sin(angle)})
	}
	points = append(points,
		Point{X: -0.990, Y: 0.016},
		Point{X: -0.973, Y: 0.016},
		Point{X: -0.973, Y: -0.016},
		Point{X: -0.990, Y: -0.016},
	)
	for index := outlineSegments/2 + 1; index < outlineSegments; index++ {
		angle := 2 * math.Pi * float64(index) / outlineSegments
		points = append(points, Point{X: math.Cos(angle), Y: math.Sin(angle)})
	}
	points = append(points,
		Point{X: 0.990, Y: -0.016},
		Point{X: 0.973, Y: -0.016},
	)
	return points
}

// deathStarSurfacePoint projects a longitude/latitude-like coordinate from a
// unit sphere. A constant longitude becomes a curved screen path, while panel
// widths naturally shrink toward the top and bottom of the visible sphere.
func deathStarSurfacePoint(longitude, latitude float64) Point {
	return Point{X: longitude * math.Sqrt(max(0, 1-latitude*latitude)), Y: latitude}
}

// deathStarPolarBeltPoint curves each polar-band edge toward its associated
// pole at screen centre. This is a perspective-friendly visual projection:
// the band reads as sitting on the spherical skin rather than as a flat line
// stamped over the orbital billboard.
func deathStarPolarBeltPoint(latitude, edge, longitude float64) Point {
	poleDirection := math.Copysign(1, latitude)
	bow := 0.035 * math.Sqrt(max(0, 1-longitude*longitude))
	y := latitude + edge + poleDirection*bow
	return Point{X: longitude * math.Sqrt(max(0, 1-y*y)), Y: y}
}

// clipLineOutsideRotatedEllipse returns the portions of a line that remain
// outside an ellipse. It avoids submitting surface detail that will sit behind
// a local opaque feature such as the superlaser dish.
func clipLineOutsideRotatedEllipse(line Line, center Point, radiusX, radiusY, rotation float64) []Line {
	return clipLineByRotatedEllipse(line, center, radiusX, radiusY, rotation, false)
}

// clipLineInsideRotatedEllipse returns the portions of a line within an
// ellipse. It keeps nested feature detail such as the dish's recessed rings
// and spokes bounded by the visible outer rim.
func clipLineInsideRotatedEllipse(line Line, center Point, radiusX, radiusY, rotation float64) []Line {
	return clipLineByRotatedEllipse(line, center, radiusX, radiusY, rotation, true)
}

// clipLineOutsideConvexPolygon removes the portion of a line inside a
// counter-clockwise convex polygon. It lets authored surface modules reserve
// their own visual area without requiring a depth or fill pass.
func clipLineOutsideConvexPolygon(line Line, polygon []Point) []Line {
	if len(polygon) < 3 {
		return []Line{line}
	}
	direction := Point{X: line.B.X - line.A.X, Y: line.B.Y - line.A.Y}
	enter, exit := 0.0, 1.0
	for index, start := range polygon {
		end := polygon[(index+1)%len(polygon)]
		edge := Point{X: end.X - start.X, Y: end.Y - start.Y}
		relative := Point{X: line.A.X - start.X, Y: line.A.Y - start.Y}
		value := edge.X*relative.Y - edge.Y*relative.X
		delta := edge.X*direction.Y - edge.Y*direction.X
		if math.Abs(delta) <= 1e-12 {
			if value < 0 {
				return []Line{line}
			}
			continue
		}
		crossing := -value / delta
		if delta > 0 {
			enter = max(enter, crossing)
		} else {
			exit = min(exit, crossing)
		}
		if enter > exit {
			return []Line{line}
		}
	}
	if exit <= 0 || enter >= 1 {
		return []Line{line}
	}
	pointAt := func(t float64) Point {
		return Point{X: line.A.X + direction.X*t, Y: line.A.Y + direction.Y*t}
	}
	visible := make([]Line, 0, 2)
	if enter > 0 {
		visible = append(visible, Line{A: line.A, B: pointAt(enter), Color: line.Color, Width: line.Width})
	}
	if exit < 1 {
		visible = append(visible, Line{A: pointAt(exit), B: line.B, Color: line.Color, Width: line.Width})
	}
	return visible
}

// clipLineInsideConvexPolygon returns only the portion of a line inside a
// counter-clockwise convex polygon. It confines recessed ingress detail to the
// visible opening, so an offset back wall cannot leak through the surrounding
// Death Star surface.
func clipLineInsideConvexPolygon(line Line, polygon []Point) []Line {
	if len(polygon) < 3 {
		return []Line{line}
	}
	direction := Point{X: line.B.X - line.A.X, Y: line.B.Y - line.A.Y}
	enter, exit := 0.0, 1.0
	for index, start := range polygon {
		end := polygon[(index+1)%len(polygon)]
		edge := Point{X: end.X - start.X, Y: end.Y - start.Y}
		relative := Point{X: line.A.X - start.X, Y: line.A.Y - start.Y}
		value := edge.X*relative.Y - edge.Y*relative.X
		delta := edge.X*direction.Y - edge.Y*direction.X
		if math.Abs(delta) <= 1e-12 {
			if value < 0 {
				return nil
			}
			continue
		}
		crossing := -value / delta
		if delta > 0 {
			enter = max(enter, crossing)
		} else {
			exit = min(exit, crossing)
		}
		if enter > exit {
			return nil
		}
	}
	if exit <= 0 || enter >= 1 || exit-enter <= 1e-9 {
		return nil
	}
	pointAt := func(t float64) Point {
		return Point{X: line.A.X + direction.X*t, Y: line.A.Y + direction.Y*t}
	}
	return []Line{{A: pointAt(enter), B: pointAt(exit), Color: line.Color, Width: line.Width}}
}

func clipLineByRotatedEllipse(line Line, center Point, radiusX, radiusY, rotation float64, keepInside bool) []Line {
	if radiusX <= 0 || radiusY <= 0 {
		return []Line{line}
	}
	cosine, sine := math.Cos(rotation), math.Sin(rotation)
	toEllipseSpace := func(point Point) (float64, float64) {
		x, y := point.X-center.X, point.Y-center.Y
		return (x*cosine + y*sine) / radiusX, (-x*sine + y*cosine) / radiusY
	}
	u0, v0 := toEllipseSpace(line.A)
	u1, v1 := toEllipseSpace(line.B)
	du, dv := u1-u0, v1-v0
	a := du*du + dv*dv
	if a == 0 {
		inside := u0*u0+v0*v0 <= 1
		if inside == keepInside {
			return []Line{line}
		}
		return nil
	}
	b := 2 * (u0*du + v0*dv)
	c := u0*u0 + v0*v0 - 1
	discriminant := b*b - 4*a*c
	cuts := []float64{0, 1}
	if discriminant > 0 {
		root := math.Sqrt(discriminant)
		for _, cut := range []float64{(-b - root) / (2 * a), (-b + root) / (2 * a)} {
			if cut > 0 && cut < 1 {
				cuts = append(cuts, cut)
			}
		}
	}
	sort.Float64s(cuts)
	pointAt := func(t float64) Point {
		return Point{X: line.A.X + (line.B.X-line.A.X)*t, Y: line.A.Y + (line.B.Y-line.A.Y)*t}
	}
	visible := make([]Line, 0, 2)
	for index := 0; index+1 < len(cuts); index++ {
		start, end := cuts[index], cuts[index+1]
		if end-start <= 1e-9 {
			continue
		}
		middle := (start + end) * 0.5
		u, v := u0+du*middle, v0+dv*middle
		inside := u*u+v*v <= 1
		if inside == keepInside {
			visible = append(visible, Line{A: pointAt(start), B: pointAt(end), Color: line.Color, Width: line.Width})
		}
	}
	return visible
}

func midpoint(first, second Point) Point {
	return Point{X: (first.X + second.X) * 0.5, Y: (first.Y + second.Y) * 0.5}
}

// deathStarSurfaceLineColor is a deliberately cheap, vector-compatible
// approximation of Phong illumination. It evaluates diffuse and specular
// terms at a line segment's midpoint on the unit sphere; it is not a filled
// per-pixel shader, so the orbital view remains additive and inexpensive.
func deathStarSurfaceLineColor(point Point) color.RGBA {
	radiusSquared := point.X*point.X + point.Y*point.Y
	normalZ := math.Sqrt(max(0, 1-radiusSquared))
	// Light comes predominantly from the right, with enough frontal component
	// to retain a readable lit rim. This prevents the front-facing centre of
	// the equatorial band from outshining the exposed right hemisphere.
	diffuse := max(0, point.X*0.80+point.Y*0.20+normalZ*0.56)
	// The normalized half-vector of that light and the front-facing viewer.
	specular := math.Pow(max(0, point.X*0.453+point.Y*0.113+normalZ*0.884), 18)
	// The ambient floor is deliberately very low: the occluded hemisphere
	// should read almost black, while retaining just enough teal line work to
	// preserve the station's spherical silhouette in the vector display.
	intensity := min(1, 0.035+0.82*diffuse+0.27*specular)
	return color.RGBA{
		R: uint8(4 + 164*intensity),
		G: uint8(10 + 185*intensity),
		B: uint8(10 + 172*intensity),
		A: 255,
	}
}

func deathStarTrenchLineColor(point Point) color.RGBA {
	color := deathStarSurfaceLineColor(point)
	color.R = uint8(float64(color.R) * 0.78)
	color.G = uint8(float64(color.G) * 0.82)
	color.B = uint8(float64(color.B) * 0.80)
	return color
}

func DefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = registry.Register(DeathStarArcade())
	_ = registry.Register(Definition{Name: "builtin/death-star-orbital-wireframe", ObjectDefinition: catalog.DeathStarName, Kind: "model-3d"})
	_ = registry.Register(Definition{Name: catalog.RebelLaserBoltAppearance, ObjectDefinition: catalog.LaserBoltName, Kind: "model-3d"})
	_ = registry.Register(Definition{Name: catalog.ImperialLaserBoltAppearance, ObjectDefinition: catalog.LaserBoltName, Kind: "model-3d"})
	_ = registry.Register(Definition{Name: catalog.TIEInterceptorAppearance, ObjectDefinition: catalog.TIEInterceptorName, Kind: "model-3d"})
	return registry
}
