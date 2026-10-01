package model

import (
	"math"
	"testing"
)

func TestTIEFighterIsValidAndSymmetrical(t *testing.T) {
	fighter := TIEFighter()
	if len(fighter.Verts) != 285 {
		t.Fatalf("fighter has %d vertices, want 285", len(fighter.Verts))
	}
	if len(fighter.Edges) != 533 {
		t.Fatalf("fighter has %d edges, want 533", len(fighter.Edges))
	}
	if err := fighter.Validate(); err != nil {
		t.Fatalf("TIEFighter returned an invalid model: %v", err)
	}

	minX, maxX := fighter.Verts[0].X, fighter.Verts[0].X
	for _, vertex := range fighter.Verts[1:] {
		minX = min(minX, vertex.X)
		maxX = max(maxX, vertex.X)
	}
	if minX != -maxX {
		t.Fatalf("fighter X bounds are [%v,%v], want symmetry", minX, maxX)
	}
	if math.Abs(maxX-(tiePanelX+tiePanelHubOutset)) > 1e-9 {
		t.Fatalf("fighter outer panel-hub position is %v, want %v", maxX, tiePanelX+tiePanelHubOutset)
	}
}

func TestTIEFighterWindowIsValid(t *testing.T) {
	window := TIEFighterWindow()
	if len(window.Verts) != tieCockpitWindowSides+tieCockpitWindowPanes {
		t.Fatalf("window has %d vertices, want %d", len(window.Verts), tieCockpitWindowSides+tieCockpitWindowPanes)
	}
	if len(window.Edges) != tieCockpitWindowSides+2*tieCockpitWindowPanes {
		t.Fatalf("window has %d edges, want %d", len(window.Edges), tieCockpitWindowSides+2*tieCockpitWindowPanes)
	}
	if err := window.Validate(); err != nil {
		t.Fatalf("TIEFighterWindow returned an invalid model: %v", err)
	}
	for index := range tieCockpitWindowSides {
		vertex := window.Verts[index]
		if math.Abs(math.Hypot(vertex.X, vertex.Y)-tieCockpitWindowOuter) > 1e-9 {
			t.Fatalf("outer canopy vertex %+v is not on radius %v", vertex, tieCockpitWindowOuter)
		}
	}
	for index := range tieCockpitWindowPanes {
		vertex := window.Verts[tieCockpitWindowSides+index]
		if math.Abs(math.Hypot(vertex.X, vertex.Y)-tieCockpitWindowInner) > 1e-9 {
			t.Fatalf("inner canopy vertex %+v is not on radius %v", vertex, tieCockpitWindowInner)
		}
	}
}

func TestTIEFighterCockpitHasCurvedTopHatch(t *testing.T) {
	cockpit := TIEFighterCockpit()
	if len(cockpit.Faces) < 2 {
		t.Fatal("cockpit has no faces")
	}
	for _, hatch := range cockpit.Faces[len(cockpit.Faces)-2:] {
		if hatch.Normal.Y <= 0 {
			t.Fatalf("hatch normal=%+v, want upper-facing", hatch.Normal)
		}
		for _, index := range hatch.Vertices {
			vertex := cockpit.Verts[index]
			if math.Abs(math.Hypot(vertex.X, vertex.Z)-tieCockpitHatchRadius) > 1e-9 {
				t.Fatalf("hatch vertex %+v is not on the hatch radius", vertex)
			}
			if vertex.Y <= 0 {
				t.Fatalf("hatch vertex %+v is not on the upper cockpit hemisphere", vertex)
			}
		}
	}
}

func TestTIEFighterGeometryDataIncludesSharedVariants(t *testing.T) {
	geometry := TIEFighterGeometryData()
	for name, mesh := range map[string]Model{
		"full":       geometry.Full,
		"core":       geometry.Core,
		"cockpit":    geometry.Cockpit,
		"pylons":     geometry.Pylons,
		"reactor":    geometry.Reactor,
		"left foil":  geometry.LeftFoil,
		"right foil": geometry.RightFoil,
		"cannons":    geometry.Cannons,
		"window":     geometry.Window,
	} {
		if err := mesh.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if len(geometry.Fragments) != 3 {
		t.Fatalf("fragments=%d, want 3", len(geometry.Fragments))
	}
}

func TestTIEFighterReactorIsRearCircularCollar(t *testing.T) {
	reactor := tieFighterReactor()
	if err := reactor.Validate(); err != nil {
		t.Fatalf("reactor invalid: %v", err)
	}
	if len(reactor.Faces) == 0 || reactor.SkipDepth {
		t.Fatalf("reactor is not a depth-capable solid: faces=%d skip=%t", len(reactor.Faces), reactor.SkipDepth)
	}
	minZ := reactor.Verts[0].Z
	for _, vertex := range reactor.Verts[1:] {
		minZ = min(minZ, vertex.Z)
	}
	rearCockpitSurface := -math.Sqrt(tieCockpitRadius*tieCockpitRadius - tieRearReactorRadius*tieRearReactorRadius)
	if minZ >= rearCockpitSurface {
		t.Fatalf("reactor rear=%v, want it to extend beyond the cockpit surface at its collar radius %v", minZ, rearCockpitSurface)
	}
}

func TestTIEFighterFragmentsReconstructEveryHullEdge(t *testing.T) {
	hull := TIEFighter()
	fragments := TIEFighterFragments()
	edgeCount := 0
	faceCount := 0
	for index, fragment := range fragments {
		if err := fragment.Validate(); err != nil {
			t.Fatalf("fragment %d is invalid: %v", index, err)
		}
		if len(fragment.Edges) == 0 {
			t.Fatalf("fragment %d contains no edges", index)
		}
		edgeCount += len(fragment.Edges)
		faceCount += len(fragment.Faces)
	}
	if edgeCount < len(hull.Edges) {
		t.Fatalf("fragments contain %d edges, fewer than hull's %d", edgeCount, len(hull.Edges))
	}
	wantFaces := len(hull.Faces) + 4 // two split planes, with a cap on each side
	if faceCount != wantFaces {
		t.Fatalf("fragments contain %d faces, want hull faces plus fracture caps (%d)", faceCount, wantFaces)
	}
}

func TestPanelHasTallReferenceSideViewProfile(t *testing.T) {
	mesh := Model{}
	appendPanel(&mesh, 0)
	minY, maxY := mesh.Verts[0].Y, mesh.Verts[0].Y
	minZ, maxZ := mesh.Verts[0].Z, mesh.Verts[0].Z
	for _, vertex := range mesh.Verts[1:6] {
		minY, maxY = min(minY, vertex.Y), max(maxY, vertex.Y)
		minZ, maxZ = min(minZ, vertex.Z), max(maxZ, vertex.Z)
	}
	if width, height := maxZ-minZ, maxY-minY; width >= height || math.Abs(width/height-tiePanelLongitudinalScale/math.Cos(math.Pi/6)) > 1e-9 {
		t.Fatalf("panel side-view width/height=%v/%v, want a tall %v ratio", width, height, tiePanelLongitudinalScale/math.Cos(math.Pi/6))
	}
	// The panel's top and bottom sides, not points, are longitudinal: in a
	// side elevation they are parallel to the fighter's Z centreline.
	if math.Abs(mesh.Verts[0].Y-mesh.Verts[5].Y) > 1e-9 || math.Abs(mesh.Verts[2].Y-mesh.Verts[3].Y) > 1e-9 {
		t.Fatalf("panel top/bottom edges are not parallel with the centreline")
	}
}

func TestTIEFighterPanelHubIsRaisedClosedHexagon(t *testing.T) {
	hub := tieFighterPanelHub(1)
	if err := hub.Validate(); err != nil {
		t.Fatalf("panel hub invalid: %v", err)
	}
	if len(hub.Verts) != 12 || len(hub.Faces) != 8 {
		t.Fatalf("panel hub has %d vertices and %d faces, want a two-ring hexagonal solid", len(hub.Verts), len(hub.Faces))
	}
	if outerX := hub.Verts[6].X; math.Abs(outerX-(tiePanelX+tiePanelHubOutset)) > 1e-9 {
		t.Fatalf("panel-hub outer face X=%v, want %v", outerX, tiePanelX+tiePanelHubOutset)
	}
	if innerX := hub.Verts[0].X; math.Abs(innerX-(tiePanelX-tiePanelHubOverlap)) > 1e-9 {
		t.Fatalf("panel-hub inner face X=%v, want overlap at %v", innerX, tiePanelX-tiePanelHubOverlap)
	}
	for corner := 0; corner < 6; corner++ {
		vertex := hub.Verts[6+corner]
		if normalizedRadius := math.Hypot(vertex.Y, vertex.Z/tiePanelLongitudinalScale); math.Abs(normalizedRadius-tiePanelHubRadius) > 1e-9 {
			t.Fatalf("hub corner %d normalized radius=%v, want %v", corner, normalizedRadius, tiePanelHubRadius)
		}
	}
}

func TestTIEFighterPanelSegmentDividersRadiateFromHub(t *testing.T) {
	panel := Model{}
	appendPanel(&panel, tiePanelX)
	prepared := OrientOutward(panel)
	dividerCount := 0
	for _, edge := range prepared.Edges {
		frontToHub := edge.A < 6 && edge.B >= 6 && edge.B < 12
		backToHub := edge.A >= 12 && edge.A < 18 && edge.B >= 18 && edge.B < 24
		if frontToHub || backToHub {
			if edge.Kind != EdgeDecorative {
				t.Fatalf("panel divider %+v is %v, want visible decorative line art", edge, edge.Kind)
			}
			outer, hub := prepared.Verts[edge.A], prepared.Verts[edge.B]
			if math.Abs(outer.Y*hub.Z-outer.Z*hub.Y) > 1e-9 || outer.Y*hub.Y+outer.Z*hub.Z <= 0 {
				t.Fatalf("panel divider %+v does not follow a centre-origin hub ray", edge)
			}
			dividerCount++
		}
	}
	if dividerCount != 12 {
		t.Fatalf("panel has %d hub-radiating dividers, want 12 across both faces", dividerCount)
	}
}

func TestTIEFighterPylonIsTaperedOpaqueSolid(t *testing.T) {
	pylon := tieFighterPylon()
	if err := pylon.Validate(); err != nil {
		t.Fatalf("pylon invalid: %v", err)
	}
	if pylon.SkipDepth || len(pylon.Faces) == 0 || pylon.Topology == nil {
		t.Fatalf("pylon is not a depth-capable solid: skip=%t faces=%d topology=%v", pylon.SkipDepth, len(pylon.Faces), pylon.Topology != nil)
	}
	const segments = 10
	wantRadii := []float64{tiePylonCockpitRadius, tiePylonNeckRadius, tiePylonNeckRadius, tiePylonPanelRadius}
	for ring, want := range wantRadii {
		vertex := pylon.Verts[ring*segments]
		got := math.Hypot(vertex.Y, vertex.Z)
		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("ring %d radius=%v, want %v", ring, got, want)
		}
	}
	if !(wantRadii[0] > wantRadii[1] && wantRadii[3] > wantRadii[2] && wantRadii[3] < wantRadii[0]) {
		t.Fatalf("pylon profile does not flare at both interfaces: %v", wantRadii)
	}
	// The panel's finite body spans from tiePanelX-0.06 to tiePanelX on the
	// right. The pylon must terminate inside that thickness, not just short of
	// it, so the opaque panel and pylon meet without a visible depth gap.
	if tiePylonPanelX <= tiePanelX-0.06 || tiePylonPanelX >= tiePanelX {
		t.Fatalf("pylon panel join=%v, want buried within panel body ending at %v", tiePylonPanelX, tiePanelX)
	}
	for _, edge := range pylon.Edges {
		if edge.A/segments != edge.B/segments {
			continue
		}
		ring := edge.A / segments
		if ring == 0 || ring == len(wantRadii)-1 {
			if edge.Kind != EdgeInternal {
				t.Fatalf("buried pylon ring %d edge %+v is visible", ring, edge)
			}
		}
	}
}
