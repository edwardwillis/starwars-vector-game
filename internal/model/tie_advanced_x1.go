package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

const (
	tieAdvancedFuselageRearZ      = -1.65
	tieAdvancedFuselageMaxWidth   = 2.96
	tieAdvancedFuselageMaxHeight  = 0.18
	tieAdvancedFuselageRearHeight = 0.06
	tieAdvancedRearStrutLength    = 1.20
	tieAdvancedRearStrutCenterZ   = -1.08
	tieAdvancedPanelCenterX       = 1.60
	// The array is a single folded shell per side. Its lateral depth keeps the
	// filled panels opaque without creating overlapping bodies at the folds.
	tieAdvancedPanelThickness = 0.14
	// The initial side-view panel section is a long, shallow rectangle. The
	// taller outer array geometry belongs to a later construction stage.
	tieAdvancedPanelHalfHeight = 0.33
	tieAdvancedPanelForwardZ   = 1.04
	tieAdvancedPanelRearZ      = -2.60
	// With a 0.86 rise, this gives the adjoining panel a 53.5-degree included
	// fold relative to the rectangular end panel.
	tieAdvancedOuterPanelInset = 0.635
	tieAdvancedOuterPanelRise  = 0.86
	// The outer panel's front and rear edges both rake inwards by 0.40 in side
	// view, reducing the marked corner angle without changing the lateral fold.
	tieAdvancedOuterPanelTaper  = 0.40
	tieAdvancedOuterPanelRearZ  = tieAdvancedPanelRearZ + tieAdvancedOuterPanelTaper
	tieAdvancedOuterPanelFrontZ = tieAdvancedPanelForwardZ - tieAdvancedOuterPanelTaper
	// Sparse array markings are inset from each outward-facing panel perimeter.
	// They communicate the reference's major panel divisions without adding a
	// second coplanar surface or dense vector noise.
	tieAdvancedPanelMarkChordInset = 0.055
	tieAdvancedPanelMarkSpanInset  = 0.10
	tieAdvancedPanelMarkSplit      = 0.68
	tieAdvancedPanelMarkOffsetX    = 0.008
	// The x1's forward viewport has a broader outer ring than the shared TIE
	// fighter canopy, while retaining the same central pane and radial frame.
	tieAdvancedWindowOuterRadius = 0.46
	// The initial armament pass is intentionally only the twin barrel silhouette.
	// Both barrels are mounted below the shared cockpit and point along +Z.
	tieAdvancedCannonOffsetX = tieUnderCockpitCannonOffsetX
	tieAdvancedCannonY       = tieUnderCockpitCannonY
	tieAdvancedCannonRadius  = tieUnderCockpitCannonRadius
	tieAdvancedCannonLength  = tieUnderCockpitCannonLength
	tieAdvancedCannonRearZ   = tieUnderCockpitCannonRearZ
	tieAdvancedCannonCenterZ = tieUnderCockpitCannonCenterZ
)

// TIEAdvancedX1Geometry is the staged construction source for Vader's fighter.
// Its cockpit remains the exact shared TIE command pod throughout later stages.
type TIEAdvancedX1Geometry struct {
	Full                Model
	Cockpit             Model
	Reactor             Model
	Fuselage            Model
	Pylons              Model
	PortSolarArray      Model
	StarboardSolarArray Model
	SolarArrays         Model
	Cannons             Model
	RearStrut           Model
	Window              Model
	Fragments           [3]Model
}

// TIEAdvancedX1GeometryData returns checkpoint two: the unchanged shared pod
// embedded in two thin rear quarter-disks, two dedicated wedge-shaped wing
// roots, two solar-array rectangles, four angled trapezoidal array extensions,
// a plain rear reactor collar, and one centreline rear strut. It intentionally
// contains no panel framing, engine outlets, armament, seams, or decorative
// geometry.
func TIEAdvancedX1GeometryData() TIEAdvancedX1Geometry {
	sharedTIE := TIEFighterGeometryData()
	cockpit := sharedTIE.Cockpit
	reactor := tieAdvancedReactorCollar()
	fuselage := tieAdvancedFuselage()
	pylons := tieAdvancedWingRoots()
	portSolarArray, starboardSolarArray := tieAdvancedSolarArrayPair()
	solarArrays := Merge(portSolarArray, starboardSolarArray)
	cannons := tieAdvancedCannons()
	rearStrut := tieAdvancedRearStrut()
	full := Merge(cockpit, reactor, fuselage, pylons, solarArrays, cannons, rearStrut)
	return TIEAdvancedX1Geometry{
		Full:                full,
		Cockpit:             cockpit,
		Reactor:             reactor,
		Fuselage:            fuselage,
		Pylons:              pylons,
		PortSolarArray:      portSolarArray,
		StarboardSolarArray: starboardSolarArray,
		SolarArrays:         solarArrays,
		Cannons:             cannons,
		RearStrut:           rearStrut,
		Window:              TIEAdvancedX1Window(),
		Fragments:           splitTIEAdvancedX1Fragments(full),
	}
}

// TIEAdvancedX1Window widens only the outer windscreen circle. The shared
// command pod and the inner octagonal pane remain unchanged, so the broader
// x1 viewport integrates cleanly with the reused cockpit geometry.
func TIEAdvancedX1Window() Model {
	window := TIEFighterWindow()
	for segment := range tieCockpitWindowSides {
		angle := 2 * math.Pi * float64(segment) / tieCockpitWindowSides
		sine, cosine := math.Sincos(angle)
		x, y := tieAdvancedWindowOuterRadius*cosine, tieAdvancedWindowOuterRadius*sine
		window.Verts[segment] = math3d.Vec3{
			X: x,
			Y: y,
			Z: math.Sqrt(tieCockpitRadius*tieCockpitRadius-x*x-y*y) + 0.015,
		}
	}
	window.Topology = nil
	return OrientOutward(window)
}

// tieAdvancedCannons adds the two low-detail, six-sided laser barrels under
// the forward cockpit. Their rear ends overlap the pod slightly, avoiding a
// floating attachment seam while their muzzles project cleanly along +Z.
func tieAdvancedCannons() Model {
	return tieUnderCockpitCannons()
}

// TIEAdvancedX1CannonMuzzles returns the port and starboard barrel tips used
// by the catalogue's combat anchors. Both positions derive directly from the
// same barrel placement constants as the visible geometry.
func TIEAdvancedX1CannonMuzzles() [2]math3d.Vec3 {
	return TIEFighterCannonMuzzles()
}

// TIEAdvancedX1Fragments splits the finished x1 into port, central, and
// starboard debris components for the standard two-stage destruction path.
func TIEAdvancedX1Fragments() [3]Model {
	return splitTIEAdvancedX1Fragments(TIEAdvancedX1())
}

func splitTIEAdvancedX1Fragments(hull Model) [3]Model {
	fragments := [3]Model{{Verts: hull.Verts}, {Verts: hull.Verts}, {Verts: hull.Verts}}
	for _, edge := range hull.Edges {
		index := tieAdvancedFragmentIndex((hull.Verts[edge.A].X + hull.Verts[edge.B].X) / 2)
		fragments[index].Edges = append(fragments[index].Edges, edge)
	}
	for _, face := range hull.Faces {
		centroidX := 0.0
		for _, vertex := range face.Vertices {
			centroidX += hull.Verts[vertex].X
		}
		index := tieAdvancedFragmentIndex(centroidX / float64(len(face.Vertices)))
		fragments[index].Faces = append(fragments[index].Faces, face)
	}
	addFractureCaps(hull, &fragments, []float64{-0.48, 0.48})
	for index := range fragments {
		fragments[index] = Prepare(fragments[index])
	}
	return fragments
}

func tieAdvancedFragmentIndex(x float64) int {
	if x < -0.48 {
		return 0
	}
	if x > 0.48 {
		return 2
	}
	return 1
}

// tieAdvancedSolarArrayPair makes the two wings as independent, manifold
// folded shells. Their separate models allow the depth pass to let a near wing
// occlude the opposite wing while preserving each wing's own silhouette edges.
// The three panel planes on one wing share their hinge vertices and have no
// faces buried between them.
func tieAdvancedSolarArrayPair() (port, starboard Model) {
	right := tieAdvancedSolarArray()
	left := tieAdvancedMirrorSolarArray(right)
	return left, right
}

// tieAdvancedMirrorSolarArray mirrors the authored starboard shell while
// explicitly reversing each winding. OrientOutward cannot be used here: the
// folded cross-section is concave, so a centroid-based convex-solid repair
// would turn its inward-facing fold skins back outwards.
func tieAdvancedMirrorSolarArray(source Model) Model {
	result := Transform(source, math3d.Scaling(-1, 1, 1))
	for index := range result.Faces {
		reverseFaceIndices(result.Faces[index].Vertices)
		reverseUVs(result.Faces[index].UVs)
	}
	result.Topology = nil
	return Prepare(result)
}

// tieAdvancedSolarArray returns the starboard folded solar wing.  Each skin
// has a central rectangle and its upper/lower trapezoid, joined along two
// shared crease edges. The only thickness walls are around the outside
// perimeter, so no panel owns an overlapping internal cap.
func tieAdvancedSolarArray() Model {
	const (
		lowerRear = iota
		upperRear
		upperFront
		lowerFront
		upperOuterRear
		upperOuterFront
		lowerOuterRear
		lowerOuterFront
		verticesPerSkin
	)
	outerX := tieAdvancedPanelCenterX - tieAdvancedOuterPanelInset
	outerY := tieAdvancedPanelHalfHeight + tieAdvancedOuterPanelRise
	midplane := []math3d.Vec3{
		{X: tieAdvancedPanelCenterX, Y: -tieAdvancedPanelHalfHeight, Z: tieAdvancedPanelRearZ},
		{X: tieAdvancedPanelCenterX, Y: tieAdvancedPanelHalfHeight, Z: tieAdvancedPanelRearZ},
		{X: tieAdvancedPanelCenterX, Y: tieAdvancedPanelHalfHeight, Z: tieAdvancedPanelForwardZ},
		{X: tieAdvancedPanelCenterX, Y: -tieAdvancedPanelHalfHeight, Z: tieAdvancedPanelForwardZ},
		{X: outerX, Y: outerY, Z: tieAdvancedOuterPanelRearZ},
		{X: outerX, Y: outerY, Z: tieAdvancedOuterPanelFrontZ},
		{X: outerX, Y: -outerY, Z: tieAdvancedOuterPanelRearZ},
		{X: outerX, Y: -outerY, Z: tieAdvancedOuterPanelFrontZ},
	}

	// Translate both skins by one common lateral vector. This preserves exact
	// shared crease vertices between all three panel planes.
	halfThickness := tieAdvancedPanelThickness / 2
	mesh := Model{}
	for _, offsetX := range []float64{halfThickness, -halfThickness} {
		for _, vertex := range midplane {
			vertex.X += offsetX
			mesh.Verts = append(mesh.Verts, vertex)
		}
	}
	inner := verticesPerSkin
	mesh.Faces = []Face{
		// Outer lateral skin: central rectangle, then upper and lower folds.
		{Vertices: []int{lowerRear, upperRear, upperFront, lowerFront}},
		{Vertices: []int{upperRear, upperOuterRear, upperOuterFront, upperFront}},
		{Vertices: []int{lowerRear, lowerFront, lowerOuterFront, lowerOuterRear}},
		// Inner lateral skin, in the opposite winding.
		{Vertices: []int{inner + lowerRear, inner + lowerFront, inner + upperFront, inner + upperRear}},
		{Vertices: []int{inner + upperFront, inner + upperOuterFront, inner + upperOuterRear, inner + upperRear}},
		{Vertices: []int{inner + lowerOuterRear, inner + lowerOuterFront, inner + lowerFront, inner + lowerRear}},
	}

	// The combined folded sheet has one eight-edge outer boundary. Hinge edges
	// are intentionally absent here: the adjacent panel faces already meet at
	// the same vertices, giving a clean structural crease without a buried wall.
	boundary := []int{
		lowerRear, upperRear, upperOuterRear, upperOuterFront,
		upperFront, lowerFront, lowerOuterFront, lowerOuterRear,
	}
	for index, outer := range boundary {
		next := boundary[(index+1)%len(boundary)]
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{outer, inner + outer, inner + next, next}})
	}
	tieAdvancedAddSolarPanelMarkings(&mesh, [4]math3d.Vec3{
		mesh.Verts[lowerRear], mesh.Verts[upperRear], mesh.Verts[upperFront], mesh.Verts[lowerFront],
	})
	tieAdvancedAddSolarPanelMarkings(&mesh, [4]math3d.Vec3{
		mesh.Verts[upperRear], mesh.Verts[upperOuterRear], mesh.Verts[upperOuterFront], mesh.Verts[upperFront],
	})
	tieAdvancedAddSolarPanelMarkings(&mesh, [4]math3d.Vec3{
		mesh.Verts[lowerRear], mesh.Verts[lowerOuterRear], mesh.Verts[lowerOuterFront], mesh.Verts[lowerFront],
	})
	return Prepare(mesh)
}

// tieAdvancedAddSolarPanelMarkings adds the reference's large inset panel
// frame and its longitudinal split to one outward-facing panel plane. The
// vertices sit just outboard of the filled shell, so the lines remain crisp
// without competing with its surface depth.
func tieAdvancedAddSolarPanelMarkings(mesh *Model, corners [4]math3d.Vec3) {
	for index := range corners {
		corners[index].X += tieAdvancedPanelMarkOffsetX
	}
	point := func(chord, span float64) math3d.Vec3 {
		rear := corners[0].Scale(1 - span).Add(corners[1].Scale(span))
		front := corners[3].Scale(1 - span).Add(corners[2].Scale(span))
		return rear.Scale(1 - chord).Add(front.Scale(chord))
	}
	chordMin, chordMax := tieAdvancedPanelMarkChordInset, 1-tieAdvancedPanelMarkChordInset
	spanMin, spanMax := tieAdvancedPanelMarkSpanInset, 1-tieAdvancedPanelMarkSpanInset
	frame := [4]math3d.Vec3{
		point(chordMin, spanMin), point(chordMin, spanMax),
		point(chordMax, spanMax), point(chordMax, spanMin),
	}
	for index, start := range frame {
		tieAdvancedAddPanelMarkEdge(mesh, start, frame[(index+1)%len(frame)])
	}
	tieAdvancedAddPanelMarkEdge(mesh, point(tieAdvancedPanelMarkSplit, spanMin), point(tieAdvancedPanelMarkSplit, spanMax))
}

func tieAdvancedAddPanelMarkEdge(mesh *Model, start, end math3d.Vec3) {
	first := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, start, end)
	mesh.Edges = append(mesh.Edges, Edge{A: first, B: first + 1, Kind: EdgeDecorative, Importance: 0.75})
}

// tieAdvancedReactorCollar is intentionally line-only: the x1 checkpoint needs
// the rear engine's circular outline, not a concealed cylinder or the standard
// TIE's domed, radially framed reactor assembly. The surrounding cockpit and
// rear strut own physical depth at this location.
func tieAdvancedReactorCollar() Model {
	const segments = tieRearReactorSides
	rearZ := tieRearReactorCenterZ - tieRearReactorLength/2
	mesh := Model{}
	for segment := range segments {
		angle := 2 * math.Pi * float64(segment) / segments
		mesh.Verts = append(mesh.Verts, math3d.Vec3{
			X: tieRearReactorRadius * math.Cos(angle),
			Y: tieRearReactorRadius * math.Sin(angle),
			Z: rearZ,
		})
		mesh.Edges = append(mesh.Edges, Edge{A: segment, B: (segment + 1) % segments})
	}
	return Prepare(mesh)
}

// tieAdvancedWingRoots makes the x1's characteristic muscular root fairings.
// Each longitudinal station has a non-regular hexagonal YZ section. A single
// +Z apex creates the sharp leading ridge; two large fore and two large aft
// facets carry most of the volume, leaving deliberately narrow top and bottom
// faces. The sections taper continuously toward the future solar-panel root.
func tieAdvancedWingRoots() Model {
	right := tieAdvancedWingRoot()
	left := OrientOutward(Transform(right, math3d.Scaling(-1, 1, 1)))
	return Merge(left, right)
}

type tieAdvancedRootStation struct {
	X, HalfHeight, LeadingZ, TrailingZ float64
}

func tieAdvancedWingRoot() Model {
	stations := []tieAdvancedRootStation{
		{X: 0.38, HalfHeight: 0.56, LeadingZ: 0.59, TrailingZ: -0.45},
		{X: 0.64, HalfHeight: 0.25, LeadingZ: 0.30, TrailingZ: -0.20},
		{X: 1.16, HalfHeight: 0.17, LeadingZ: 0.22, TrailingZ: -0.14},
		{X: 1.58, HalfHeight: 0.12, LeadingZ: 0.16, TrailingZ: -0.09},
	}
	const verticesPerStation = 6
	mesh := Model{}
	for _, station := range stations {
		frontShoulderZ := station.LeadingZ * 0.36
		rearShoulderZ := station.TrailingZ * 0.36
		mesh.Verts = append(mesh.Verts,
			math3d.Vec3{X: station.X, Z: station.LeadingZ},
			math3d.Vec3{X: station.X, Y: station.HalfHeight, Z: frontShoulderZ},
			math3d.Vec3{X: station.X, Y: station.HalfHeight, Z: rearShoulderZ},
			math3d.Vec3{X: station.X, Z: station.TrailingZ},
			math3d.Vec3{X: station.X, Y: -station.HalfHeight, Z: rearShoulderZ},
			math3d.Vec3{X: station.X, Y: -station.HalfHeight, Z: frontShoulderZ},
		)
	}
	for station := range stations {
		base := station * verticesPerStation
		for vertex := 0; vertex < verticesPerStation; vertex++ {
			next := (vertex + 1) % verticesPerStation
			mesh.Edges = append(mesh.Edges, Edge{A: base + vertex, B: base + next})
			if station == len(stations)-1 {
				continue
			}
			nextBase := base + verticesPerStation
			mesh.Edges = append(mesh.Edges, Edge{A: base + vertex, B: nextBase + vertex})
			mesh.Faces = append(mesh.Faces, Face{Vertices: []int{base + vertex, base + next, nextBase + next, nextBase + vertex}})
		}
	}
	mesh.Faces = append(mesh.Faces,
		Face{Vertices: []int{0, 5, 4, 3, 2, 1}},
		Face{Vertices: []int{len(mesh.Verts) - 6, len(mesh.Verts) - 5, len(mesh.Verts) - 4, len(mesh.Verts) - 3, len(mesh.Verts) - 2, len(mesh.Verts) - 1}},
	)
	return hideEmbeddedAdvancedRootCap(OrientOutward(mesh))
}

// hideEmbeddedAdvancedRootCap suppresses only the triangular seam embedded in
// the cockpit. The outer triangular cap remains available as the future solar
// wing's attachment boundary.
func hideEmbeddedAdvancedRootCap(mesh Model) Model {
	mesh.Topology = nil
	for index := range mesh.Edges {
		edge := &mesh.Edges[index]
		if edge.A < 6 && edge.B < 6 {
			edge.Kind = EdgeInternal
		}
	}
	return Prepare(mesh)
}

// TIEAdvancedX1 returns the current staged x1 mesh without its contrasting
// viewport layer.
func TIEAdvancedX1() Model { return TIEAdvancedX1GeometryData().Full }

// tieAdvancedFuselage is the two thin, mirrored quarter-disks seen in the
// plan reference. Their outer arcs meet the lateral struts; their inner edges
// stop at the separate rear strut, keeping the central channel structurally
// legible instead of filling it with one deep D-shaped body.
func tieAdvancedFuselage() Model {
	rightProfile := []tieAdvancedFuselageStation{
		{X: 0.24, Z: -0.43, Height: tieAdvancedFuselageMaxHeight},
		{X: tieAdvancedFuselageMaxWidth / 2, Z: -0.10, Height: tieAdvancedFuselageMaxHeight},
		{X: 1.42, Z: -0.84, Height: 0.14},
		{X: 1.08, Z: -1.34, Height: 0.10},
		{X: 0.56, Z: -1.59, Height: tieAdvancedFuselageRearHeight},
		{X: 0.24, Z: tieAdvancedFuselageRearZ, Height: tieAdvancedFuselageRearHeight},
	}
	right := taperedAdvancedQuarterDisk(rightProfile)
	left := OrientOutward(Transform(right, math3d.Scaling(-1, 1, 1)))
	return Merge(left, right)
}

type tieAdvancedFuselageStation struct {
	X, Z   float64
	Height float64
}

// taperedAdvancedQuarterDisk closes a planform profile with independently
// authored top and bottom stations. It preserves the reference top silhouette
// while making the rear edge visibly thinner in side and rear views.
func taperedAdvancedQuarterDisk(profile []tieAdvancedFuselageStation) Model {
	mesh := Model{}
	for _, station := range profile {
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: station.X, Y: station.Height / 2, Z: station.Z})
	}
	for _, station := range profile {
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: station.X, Y: -station.Height / 2, Z: station.Z})
	}
	count := len(profile)
	for index := range profile {
		next := (index + 1) % count
		mesh.Edges = append(mesh.Edges,
			Edge{A: index, B: next},
			Edge{A: count + index, B: count + next},
			Edge{A: index, B: count + index},
		)
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{index, next, count + next, count + index}})
	}
	// Height changes make the two broad surfaces faceted rather than planar.
	// Keep their triangulation as topology only: no diagonal vector edges are
	// emitted across the otherwise clean quarter-disk surfaces.
	for index := 1; index < count-1; index++ {
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{0, index, index + 1}},
			Face{Vertices: []int{count, count + index + 1, count + index}},
		)
	}
	return OrientOutward(mesh)
}

// tieAdvancedRearStrut is intentionally just a squashed hexagonal structural
// member. It shares the approximate longitudinal span of a standard TIE wing
// strut but adds no engine nozzle or exhaust detail at this checkpoint.
func tieAdvancedRearStrut() Model {
	strut := Transform(cylinder(0.28, tieAdvancedRearStrutLength, 6), math3d.Scaling(1, 0.62, 1))
	return hideEmbeddedAdvancedStrutCap(Transform(strut, math3d.Translation(0, -0.04, tieAdvancedRearStrutCenterZ)))
}

// hideEmbeddedAdvancedStrutCap removes the cap outline at the strut's forward
// attachment. That cap is fully inside the shared cockpit shell, so allowing
// it into the vector pass causes unstable internal lines without contributing
// to any exterior silhouette.
func hideEmbeddedAdvancedStrutCap(mesh Model) Model {
	const segments = 6
	forwardRingStart := segments
	forwardCapCenter := 2 * segments
	mesh.Topology = nil
	for index := range mesh.Edges {
		edge := &mesh.Edges[index]
		inForwardRing := func(vertex int) bool {
			return vertex >= forwardRingStart && vertex < forwardRingStart+segments
		}
		if (inForwardRing(edge.A) && inForwardRing(edge.B)) || edge.A == forwardCapCenter || edge.B == forwardCapCenter {
			edge.Kind = EdgeInternal
		}
	}
	return Prepare(mesh)
}
