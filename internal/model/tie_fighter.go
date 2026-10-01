package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

const (
	tiePanelX             = 1.61
	tiePylonCockpitX      = 0.45
	tiePylonNeckX         = 0.63
	tiePylonOuterNeckX    = 1.25
	tiePylonPanelX        = tiePanelX - 0.03
	tiePylonCockpitRadius = 0.44
	tiePylonNeckRadius    = 0.22
	tiePylonPanelRadius   = 0.34
	tieCockpitRadius      = 0.68
	tieCockpitHatchRadius = 0.24
	tieCockpitHatchOffset = 0.008
	tieCockpitHatchSides  = 10
	tieCockpitWindowOuter = 0.40
	tieCockpitWindowInner = 0.16
	tieCockpitWindowSides = 16
	tieCockpitWindowPanes = 8
	// The panel outline and its central fitting scale together, preserving the
	// established side-view proportions as the solar array grows.
	tiePanelRadius    = 2.01168
	tiePanelHubRadius = 0.6336
	// The hub spans through the panel body and just into the inward face. This
	// keeps the central opening opaque from either side without a coplanar
	// contact face, while its raised outer ring remains distinct in profile.
	tiePanelHubOverlap = 0.070
	tiePanelHubOutset  = 0.050
	// In a side elevation, the panel's upper and lower edges run parallel to
	// the fighter's +Z/-Z centreline rather than terminating at vertical tips.
	tiePanelHexRotation       = math.Pi / 6
	tiePanelLongitudinalScale = 0.80
	// Both the standard fighter and Advanced x1 use this compact, paired
	// under-cockpit armament. It stays deliberately sparse at vector scale.
	tieUnderCockpitCannonOffsetX = 0.27
	tieUnderCockpitCannonY       = -0.48
	tieUnderCockpitCannonRadius  = 0.055
	tieUnderCockpitCannonLength  = 0.38
	tieUnderCockpitCannonRearZ   = 0.37
	tieUnderCockpitCannonCenterZ = tieUnderCockpitCannonRearZ + tieUnderCockpitCannonLength/2
)

// TIEFighterGeometry contains the immutable variants used by the catalog:
// complete hull, independently drawable parts, window, and debris fragments.
type TIEFighterGeometry struct {
	Full      Model
	Core      Model
	Cockpit   Model
	Pylons    Model
	Reactor   Model
	LeftFoil  Model
	RightFoil Model
	Cannons   Model
	Window    Model
	Fragments [3]Model
}

// TIEFighterGeometryData centralizes fighter geometry construction so live
// objects and destruction representations use the same authored source.
func TIEFighterGeometryData() TIEFighterGeometry {
	cockpit := TIEFighterCockpit()
	pylons := tieFighterPylons()
	reactor := tieFighterReactor()
	cannons := tieUnderCockpitCannons()
	core := Merge(cockpit, pylons, reactor)
	full := assembleTIEFighter(cockpit, pylons, reactor, cannons)
	return TIEFighterGeometry{
		Full:      full,
		Core:      core,
		Cockpit:   cockpit,
		Pylons:    pylons,
		Reactor:   reactor,
		LeftFoil:  TIEFighterFoil(-1),
		RightFoil: TIEFighterFoil(1),
		Cannons:   cannons,
		Window:    TIEFighterWindow(),
		Fragments: splitTIEFighterFragments(full),
	}
}

// TIEFighter returns a deliberately simple, original wireframe fighter:
// a faceted cockpit, two tapered pylons, and two tall framed panels. It is inspired
// by the broad geometry of classic twin-panel space fighters without copying a
// production asset.
func TIEFighter() Model {
	return assembleTIEFighter(TIEFighterCockpit(), tieFighterPylons(), tieFighterReactor(), tieUnderCockpitCannons())
}

func assembleTIEFighter(cockpit, pylons, reactor, cannons Model) Model {
	mesh := Merge(cockpit, pylons, reactor, cannons, TIEFighterFoil(-1), TIEFighterFoil(1))
	mesh.Topology = nil
	// Every independently authored solid has already been oriented outward.
	// Preserve that winding across the joined destruction mesh instead of
	// reorienting the overlapping cockpit/pylon/panel assembly as one volume.
	return Prepare(mesh)
}

// tieUnderCockpitCannons adds a mirrored pair of six-sided barrels below the
// command pod. Their rear ends overlap the cockpit, while the tips point along
// +Z, the shared forward axis.
func tieUnderCockpitCannons() Model {
	barrel := cylinder(tieUnderCockpitCannonRadius, tieUnderCockpitCannonLength, 6)
	right := Transform(barrel, math3d.Translation(tieUnderCockpitCannonOffsetX, tieUnderCockpitCannonY, tieUnderCockpitCannonCenterZ))
	left := Transform(barrel, math3d.Translation(-tieUnderCockpitCannonOffsetX, tieUnderCockpitCannonY, tieUnderCockpitCannonCenterZ))
	return Merge(left, right)
}

// TIEFighterCannonMuzzles returns the physical tips of the shared paired
// cannon geometry for catalogue firing anchors.
func TIEFighterCannonMuzzles() [2]math3d.Vec3 {
	muzzleZ := tieUnderCockpitCannonCenterZ + tieUnderCockpitCannonLength/2
	return [2]math3d.Vec3{
		{X: -tieUnderCockpitCannonOffsetX, Y: tieUnderCockpitCannonY, Z: muzzleZ},
		{X: tieUnderCockpitCannonOffsetX, Y: tieUnderCockpitCannonY, Z: muzzleZ},
	}
}

// TIEFighterCore is the cockpit and pylon assembly without the solar-panel
// foils. Keeping the physical assemblies separate lets the renderer assign
// independent depth ownership, so a foil can occlude cockpit lines while each
// part's own structural edges remain visible.
func TIEFighterCore() Model {
	return Merge(TIEFighterCockpit(), tieFighterPylons(), tieFighterReactor())
}

// TIEFighterCockpit returns the shared central command-pod geometry without
// solar-panel pylons. Other TIE-family craft reuse this authored cockpit and
// orient it to their own flight axis instead of duplicating the hull.
func TIEFighterCockpit() Model {
	mesh := Model{}
	appendCockpit(&mesh)
	return OrientOutward(mesh)
}

// tieFighterReactor adds the Solar Ionization Reactor collar to the command
// pod's rear, opposite the forward canopy. Its forward cap is embedded just
// inside the sphere, while the rear circular face projects beyond it as the
// characteristic reactor fitting.
func tieFighterReactor() Model {
	return Transform(
		tieRearReactorAssembly(),
		math3d.Translation(0, 0, tieRearReactorCenterZ),
	)
}

func tieFighterPylons() Model {
	right := tieFighterPylon()
	left := Transform(right, math3d.RotationZ(math.Pi))
	return Merge(left, right)
}

func tieFighterPylon() Model {
	// Like the refined Interceptor brace, this is one continuous finite solid:
	// a broad cockpit collar quickly necks down to a short straight section,
	// then flares back out at the solar-panel root. Both broad ends are buried
	// slightly into their adjoining bodies, preventing a coplanar cap from
	// leaking through either opaque surface during depth classification.
	pylon := profiledCylinderX([]cylinderRing{
		{X: tiePylonCockpitX, Radius: tiePylonCockpitRadius},
		{X: tiePylonNeckX, Radius: tiePylonNeckRadius},
		{X: tiePylonOuterNeckX, Radius: tiePylonNeckRadius},
		{X: tiePylonPanelX, Radius: tiePylonPanelRadius},
	}, 10)
	return hideBuriedProfileInterfaceRings(pylon, 10)
}

// hideBuriedProfileInterfaceRings suppresses only the circular outline at a
// profiled solid's first and final rings. Those boundaries are deliberately
// embedded in adjoining opaque bodies, so rendering them would look like an
// internal line leaking through the cockpit or solar-panel root.
func hideBuriedProfileInterfaceRings(mesh Model, segments int) Model {
	if segments <= 0 || len(mesh.Verts) < 2*segments {
		return mesh
	}
	mesh.Topology = nil
	lastRing := len(mesh.Verts)/segments - 1
	for index := range mesh.Edges {
		edge := &mesh.Edges[index]
		if edge.A/segments != edge.B/segments {
			continue
		}
		ring := edge.A / segments
		if ring == 0 || ring == lastRing {
			edge.Kind = EdgeInternal
		}
	}
	return Prepare(mesh)
}

// TIEFighterFoils contains the two finite-thickness solar panels. The tapered
// pylon ends are embedded in their inner panel planes, so separately rendered
// parts still form one connected visible fighter.
func TIEFighterFoils() Model {
	return Merge(TIEFighterFoil(-1), TIEFighterFoil(1))
}

// TIEFighterFoil returns one independently occluding solar-panel assembly.
// side must be -1 (left) or +1 (right).
func TIEFighterFoil(side int) Model {
	if side != -1 && side != 1 {
		panic("model: TIE foil side must be -1 or +1")
	}
	mesh := Model{}
	appendPanel(&mesh, tiePanelX*float64(side))
	return Merge(OrientOutward(mesh), tieFighterPanelHub(side))
}

// tieFighterPanelHub makes the conspicuous hexagonal centre fitting on the
// outward face of one solar panel. It is a shallow closed solid rather than
// line art, so the hub remains opaque and its perimeter survives all viewing
// angles without a coplanar shimmer against the panel face.
func tieFighterPanelHub(side int) Model {
	if side != -1 && side != 1 {
		panic("model: TIE panel-hub side must be -1 or +1")
	}
	panelX := tiePanelX * float64(side)
	innerX := panelX - math.Copysign(tiePanelHubOverlap, panelX)
	outerX := panelX + math.Copysign(tiePanelHubOutset, panelX)
	mesh := Model{}
	for _, x := range []float64{innerX, outerX} {
		for corner := 0; corner < 6; corner++ {
			// Match the panel perimeter: every hub corner lies on the same
			// centre-origin ray as its corresponding outer panel corner.
			mesh.Verts = append(mesh.Verts, tiePanelHexVertex(x, tiePanelHubRadius, corner))
		}
	}
	for corner := 0; corner < 6; corner++ {
		next := (corner + 1) % 6
		mesh.Edges = append(mesh.Edges,
			Edge{A: corner, B: next, Kind: EdgeStructural},
			Edge{A: 6 + corner, B: 6 + next, Kind: EdgeStructural},
			Edge{A: corner, B: 6 + corner, Kind: EdgeStructural},
		)
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{corner, next, 6 + next, 6 + corner}})
	}
	inner := []int{5, 4, 3, 2, 1, 0}
	outer := []int{6, 7, 8, 9, 10, 11}
	mesh.Faces = append(mesh.Faces, Face{Vertices: inner}, Face{Vertices: outer})
	return OrientOutward(mesh)
}

func appendCockpit(mesh *Model) {
	const segments = 12
	// Five latitude rings and shallow end caps give the command pod a clearly
	// spherical silhouette at close range while retaining the sparse vector
	// treatment of the rest of the fighter.
	rings := []struct {
		x      float64
		radius float64
	}{
		{x: -0.52, radius: 0.44},
		{x: -0.30, radius: 0.61},
		{x: 0, radius: tieCockpitRadius},
		{x: 0.30, radius: 0.61},
		{x: 0.52, radius: 0.44},
	}

	for _, ring := range rings {
		for segment := range segments {
			angle := 2 * math.Pi * float64(segment) / segments
			sine, cosine := math.Sincos(angle)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{
				X: ring.x,
				Y: ring.radius * cosine,
				Z: ring.radius * sine,
			})
		}
	}
	for ring := range rings {
		base := ring * segments
		for segment := range segments {
			next := (segment + 1) % segments
			mesh.Edges = append(mesh.Edges, Edge{A: base + segment, B: base + next})
			if ring < len(rings)-1 {
				mesh.Edges = append(mesh.Edges, Edge{A: base + segment, B: base + segments + segment})
				mesh.Faces = append(mesh.Faces, Face{Vertices: []int{
					base + segment,
					base + next,
					base + segments + next,
					base + segments + segment,
				}})
			}
		}
	}

	leftCap := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{X: -tieCockpitRadius})
	rightCap := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{X: tieCockpitRadius})
	for segment := range segments {
		mesh.Edges = append(mesh.Edges,
			Edge{A: leftCap, B: segment},
			Edge{A: rightCap, B: (len(rings)-1)*segments + segment},
		)
		next := (segment + 1) % segments
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{leftCap, next, segment}},
			Face{Vertices: []int{rightCap, (len(rings)-1)*segments + segment, (len(rings)-1)*segments + next}},
		)
	}
	appendCockpitHatch(mesh)

}

// appendCockpitHatch marks the large circular access door directly into the
// command pod's upper spherical surface. The ring is split by a restrained
// seam so it reads as a hatch rather than a separate object perched on top.
func appendCockpitHatch(mesh *Model) {
	base := len(mesh.Verts)
	for segment := range tieCockpitHatchSides {
		angle := 2 * math.Pi * float64(segment) / tieCockpitHatchSides
		sine, cosine := math.Sincos(angle)
		x, z := tieCockpitHatchRadius*cosine, tieCockpitHatchRadius*sine
		y := math.Sqrt(tieCockpitRadius*tieCockpitRadius-x*x-z*z) + tieCockpitHatchOffset
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: x, Y: y, Z: z})
	}
	for segment := range tieCockpitHatchSides {
		next := (segment + 1) % tieCockpitHatchSides
		mesh.Edges = append(mesh.Edges, Edge{A: base + segment, B: base + next, Kind: EdgeStructural})
	}
	// Two half-door faces keep the central seam tied to camera-facing hatch
	// geometry, so it disappears with the hatch on the far side of the pod.
	mesh.Edges = append(mesh.Edges, Edge{A: base, B: base + tieCockpitHatchSides/2, Kind: EdgeStructural})
	first := make([]int, tieCockpitHatchSides/2+1)
	second := make([]int, tieCockpitHatchSides/2+1)
	for index := range first {
		first[index] = base + tieCockpitHatchSides/2 - index
		second[index] = base + (tieCockpitHatchSides-index)%tieCockpitHatchSides
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: first}, Face{Vertices: second})

}

// TIEFighterWindow returns the enlarged circular forward canopy as a separate
// model so the game can draw it in a contrasting vector color. A central
// octagonal pane and eight framed outer panes follow the command pod's front
// curvature, matching the characteristic TIE canopy construction.
func TIEFighterWindow() Model {
	window := Model{}
	for segment := range tieCockpitWindowSides {
		angle := 2 * math.Pi * float64(segment) / tieCockpitWindowSides
		sine, cosine := math.Sincos(angle)
		x, y := tieCockpitWindowOuter*cosine, tieCockpitWindowOuter*sine
		windowZ := math.Sqrt(tieCockpitRadius*tieCockpitRadius-x*x-y*y) + 0.015
		window.Verts = append(window.Verts, math3d.Vec3{
			X: x,
			Y: y,
			Z: windowZ,
		})
	}
	inner := len(window.Verts)
	for segment := range tieCockpitWindowPanes {
		angle := 2 * math.Pi * float64(segment) / tieCockpitWindowPanes
		sine, cosine := math.Sincos(angle)
		x, y := tieCockpitWindowInner*cosine, tieCockpitWindowInner*sine
		windowZ := math.Sqrt(tieCockpitRadius*tieCockpitRadius-x*x-y*y) + 0.018
		window.Verts = append(window.Verts, math3d.Vec3{X: x, Y: y, Z: windowZ})
	}
	for segment := range tieCockpitWindowSides {
		next := (segment + 1) % tieCockpitWindowSides
		window.Edges = append(window.Edges, Edge{A: segment, B: next, Kind: EdgeStructural})
	}
	for pane := range tieCockpitWindowPanes {
		next := (pane + 1) % tieCockpitWindowPanes
		outerFirst := pane * tieCockpitWindowSides / tieCockpitWindowPanes
		outerMiddle := (outerFirst + 1) % tieCockpitWindowSides
		outerLast := (outerFirst + 2) % tieCockpitWindowSides
		window.Edges = append(window.Edges,
			Edge{A: inner + pane, B: inner + next, Kind: EdgeStructural},
			Edge{A: inner + pane, B: outerFirst, Kind: EdgeStructural},
		)
		window.Faces = append(window.Faces, Face{Vertices: []int{inner + pane, outerFirst, outerMiddle, outerLast, inner + next}})
	}
	innerFace := make([]int, tieCockpitWindowPanes)
	for pane := range tieCockpitWindowPanes {
		innerFace[pane] = inner + pane
	}
	window.Faces = append(window.Faces, Face{Vertices: innerFace})
	return OrientOutward(window)
}

// TIEFighterFragments partitions the fighter's edges into left, center,
// and right debris meshes. Together the three fragments reconstruct the hull.
func TIEFighterFragments() [3]Model {
	return splitTIEFighterFragments(TIEFighter())
}

func splitTIEFighterFragments(hull Model) [3]Model {
	fragments := [3]Model{
		{Verts: hull.Verts},
		{Verts: hull.Verts},
		{Verts: hull.Verts},
	}
	for _, edge := range hull.Edges {
		midpointX := (hull.Verts[edge.A].X + hull.Verts[edge.B].X) * 0.5
		index := 1
		if midpointX < -0.48 {
			index = 0
		} else if midpointX > 0.48 {
			index = 2
		}
		fragments[index].Edges = append(fragments[index].Edges, edge)
	}
	for _, face := range hull.Faces {
		centroidX := 0.0
		for _, vertex := range face.Vertices {
			centroidX += hull.Verts[vertex].X
		}
		centroidX /= float64(len(face.Vertices))
		index := 1
		if centroidX < -0.48 {
			index = 0
		} else if centroidX > 0.48 {
			index = 2
		}
		fragments[index].Faces = append(fragments[index].Faces, face)
	}
	addFractureCaps(hull, &fragments, []float64{-0.48, 0.48})
	for index := range fragments {
		fragments[index] = Prepare(fragments[index])
	}
	return fragments
}

func appendPanel(mesh *Model, x float64) {
	base := len(mesh.Verts)
	// A flat-topped regular hexagon in the YZ plane, with a concentric
	// hexagonal opening for the central hub. The six resulting trapezoids are
	// the panel segments; their boundaries begin at the hub perimeter rather
	// than converging in an artificial centre-point star.
	outer := make([]math3d.Vec3, 0, 6)
	for corner := 0; corner < 6; corner++ {
		outer = append(outer, tiePanelHexVertex(x, tiePanelRadius, corner))
	}
	inner := make([]math3d.Vec3, 0, 6)
	for corner := 0; corner < 6; corner++ {
		inner = append(inner, tiePanelHexVertex(x, tiePanelHubRadius, corner))
	}
	mesh.Verts = append(mesh.Verts, outer...)
	mesh.Verts = append(mesh.Verts, inner...)
	thickness := 0.06
	// Keep the authored silhouette at the outer panel plane while adding a
	// shallow body inward toward the cockpit/pylon.
	backX := x - math.Copysign(thickness, x)
	backBase := len(mesh.Verts)
	for _, vertex := range outer {
		vertex.X = backX
		mesh.Verts = append(mesh.Verts, vertex)
	}
	for _, vertex := range inner {
		vertex.X = backX
		mesh.Verts = append(mesh.Verts, vertex)
	}
	const corners = 6
	for corner := range corners {
		next := (corner + 1) % corners
		mesh.Edges = append(mesh.Edges,
			Edge{A: base + corner, B: base + next, Kind: EdgeStructural},
			Edge{A: backBase + corner, B: backBase + next, Kind: EdgeStructural},
			Edge{A: base + corner, B: backBase + corner, Kind: EdgeStructural},
			// The segment dividers lie on a planar panel face. Marking them as
			// decorative preserves them through coplanar-edge suppression while
			// their adjacent trapezoid faces still hide them on the far side.
			Edge{A: base + corner, B: base + corners + corner, Kind: EdgeDecorative},
			Edge{A: backBase + corner, B: backBase + corners + corner, Kind: EdgeDecorative},
		)
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{base + corner, base + next, base + corners + next, base + corners + corner}},
			Face{Vertices: []int{backBase + corner, backBase + corners + corner, backBase + corners + next, backBase + next}},
			Face{Vertices: []int{base + corner, base + next, backBase + next, backBase + corner}},
		)
	}
}

func tiePanelHexVertex(x, radius float64, corner int) math3d.Vec3 {
	angle := tiePanelHexRotation + math.Pi*float64(corner)/3
	return math3d.Vec3{X: x, Y: radius * math.Cos(angle), Z: tiePanelLongitudinalScale * radius * math.Sin(angle)}
}

func reverseIndices(indices []int) {
	for left, right := 0, len(indices)-1; left < right; left, right = left+1, right-1 {
		indices[left], indices[right] = indices[right], indices[left]
	}
}
