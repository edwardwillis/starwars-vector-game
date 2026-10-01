package model

import (
	"math"
	"reflect"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestTIEAdvancedX1FuselageCheckpointReusesSharedTIEGeometry(t *testing.T) {
	geometry := TIEAdvancedX1GeometryData()
	sharedCockpit := TIEFighterCockpit()
	sharedWindow := TIEFighterWindow()
	for name, mesh := range map[string]Model{
		"full":                  geometry.Full,
		"cockpit":               geometry.Cockpit,
		"reactor":               geometry.Reactor,
		"fuselage":              geometry.Fuselage,
		"pylons":                geometry.Pylons,
		"port solar array":      geometry.PortSolarArray,
		"starboard solar array": geometry.StarboardSolarArray,
		"solar arrays":          geometry.SolarArrays,
		"under-cockpit cannons": geometry.Cannons,
		"rear strut":            geometry.RearStrut,
		"window":                geometry.Window,
	} {
		if err := mesh.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if !reflect.DeepEqual(geometry.Cockpit, sharedCockpit) {
		t.Fatal("x1 fuselage checkpoint does not exactly reuse the shared TIE command pod")
	}
	assertPlainAdvancedReactorCollar(t, geometry.Reactor)
	if reflect.DeepEqual(geometry.Pylons, TIEFighterGeometryData().Pylons) {
		t.Fatal("x1 wing roots incorrectly reuse the standard TIE's round pylons")
	}
	assertAdvancedWindowOuterRing(t, geometry.Window, sharedWindow)
	if len(geometry.Full.Verts) <= len(geometry.Cockpit.Verts) {
		t.Fatal("x1 full mesh does not include the primary fuselage")
	}
	assertAdvancedCannons(t, geometry.Cannons)
	for index, fragment := range geometry.Fragments {
		if err := fragment.Validate(); err != nil {
			t.Fatalf("x1 fragment %d: %v", index, err)
		}
		if len(fragment.PolygonModels()) == 0 {
			t.Fatalf("x1 fragment %d has no final polygon shards", index)
		}
	}
	assertAdvancedRootCrossSection(t, tieAdvancedWingRoot())
	arrayMin, arrayMax := tieAdvancedBounds(geometry.SolarArrays)
	if math.Abs(arrayMin.X+arrayMax.X) > 1e-9 || math.Abs(arrayMin.Y+arrayMax.Y) > 1e-9 {
		t.Fatalf("solar arrays are not symmetric about X=0/Y=0: min=%v max=%v", arrayMin, arrayMax)
	}
	rightArray := tieAdvancedSolarArray()
	centralVerts := append([]math3d.Vec3(nil), rightArray.Verts[:4]...)
	centralVerts = append(centralVerts, rightArray.Verts[8:12]...)
	centralMin, centralMax := tieAdvancedBounds(Model{Verts: centralVerts})
	if got := centralMax.Y - centralMin.Y; math.Abs(got-2*tieAdvancedPanelHalfHeight) > 1e-9 {
		t.Fatalf("central solar panel height=%v, want %v", got, 2*tieAdvancedPanelHalfHeight)
	}
	if got := centralMax.Z - centralMin.Z; math.Abs(got-tieAdvancedPanelForwardZ+tieAdvancedPanelRearZ) > 1e-9 {
		t.Fatalf("central solar panel chord=%v, want %v", got, tieAdvancedPanelForwardZ-tieAdvancedPanelRearZ)
	}
	if got := centralMax.Z - tieCockpitRadius; math.Abs(got-0.36) > 1e-9 {
		t.Fatalf("solar panels project %v ahead of cockpit, want 0.36", got)
	}
	if ratio := (centralMax.Z - centralMin.Z) / (centralMax.Y - centralMin.Y); ratio < 5.3 || ratio > 5.7 {
		t.Fatalf("solar panel side-view aspect=%v, want a long shallow 5.3:1 to 5.7:1 rectangle", ratio)
	}
	if arrayMax.Y <= centralMax.Y || arrayMin.Y >= centralMin.Y {
		t.Fatalf("folded panels do not extend above and below the central rectangle: central=%+v/%+v arrays=%+v/%+v", centralMin, centralMax, arrayMin, arrayMax)
	}
	if includedAngle := 90 - math.Atan2(tieAdvancedOuterPanelInset, tieAdvancedOuterPanelRise)*180/math.Pi; math.Abs(includedAngle-53.53) > 0.1 {
		t.Fatalf("outer panel included fold=%v, want approximately 53.53 degrees", includedAngle)
	}
	rearTaper := tieAdvancedOuterPanelRearZ - tieAdvancedPanelRearZ
	frontTaper := tieAdvancedPanelForwardZ - tieAdvancedOuterPanelFrontZ
	if math.Abs(rearTaper-frontTaper) > 1e-9 {
		t.Fatalf("outer panel front/rear taper=%v/%v, want mirrored equal offsets", frontTaper, rearTaper)
	}
	if math.Abs(rearTaper-tieAdvancedOuterPanelTaper) > 1e-9 {
		t.Fatalf("outer panel side-view taper=%v, want %v", rearTaper, tieAdvancedOuterPanelTaper)
	}
	if tieAdvancedOuterPanelRise != 0.86 {
		t.Fatalf("folded panel side-view rise=%v, want 0.86", tieAdvancedOuterPanelRise)
	}
	if rearRun := tieAdvancedPanelMarkSplit * (tieAdvancedPanelForwardZ - tieAdvancedPanelRearZ); math.Abs(rearRun-2.4752) > 1e-9 {
		t.Fatalf("panel rear run=%v, want 2.4752", rearRun)
	}
	if len(geometry.SolarArrays.Faces) != 28 {
		t.Fatalf("solar-array faces=%d, want two folded manifold shells", len(geometry.SolarArrays.Faces))
	}
	for _, pair := range [][2]int{{0, 3}, {1, 4}, {2, 5}} {
		first, second := rightArray.Faces[pair[0]].Normal, rightArray.Faces[pair[1]].Normal
		if dot := first.Dot(second); dot > -0.999 {
			t.Fatalf("fold skin normals %d/%d have dot=%v, want opposite-facing opaque skins", pair[0], pair[1], dot)
		}
	}
	markings := 0
	for _, edge := range rightArray.Edges {
		if edge.Kind == EdgeDecorative {
			markings++
		}
	}
	if markings != 15 {
		t.Fatalf("starboard array has %d decorative panel-marking edges, want 15", markings)
	}
	for index, face := range geometry.SolarArrays.Faces {
		if face.DoubleSided {
			t.Fatalf("solar-array body face %d unexpectedly became a transparent double-sided plane", index)
		}
	}
	for index, edge := range geometry.SolarArrays.Edges {
		if edge.Kind == EdgeDecorative {
			continue
		}
		if len(edge.AdjacentFaces) != 2 {
			t.Fatalf("solar-array edge %d has %d incident faces, want a closed folded shell", index, len(edge.AdjacentFaces))
		}
	}
}

func assertAdvancedCannons(t *testing.T, cannons Model) {
	t.Helper()
	minimum, maximum := tieAdvancedBounds(cannons)
	if math.Abs(minimum.X+maximum.X) > 1e-9 {
		t.Fatalf("under-cockpit cannons are not symmetric: min=%v max=%v", minimum, maximum)
	}
	if maximum.Y >= 0 {
		t.Fatalf("under-cockpit cannon top=%v, want entirely below the cockpit centreline", maximum.Y)
	}
	if muzzle := maximum.Z; math.Abs(muzzle-(tieAdvancedCannonCenterZ+tieAdvancedCannonLength/2)) > 1e-9 {
		t.Fatalf("cannon muzzle Z=%v, want +Z forward muzzle at %v", muzzle, tieAdvancedCannonCenterZ+tieAdvancedCannonLength/2)
	}
}

func assertAdvancedWindowOuterRing(t *testing.T, advanced, shared Model) {
	t.Helper()
	if len(advanced.Verts) != len(shared.Verts) {
		t.Fatalf("x1 window vertices=%d, want shared viewport topology of %d vertices", len(advanced.Verts), len(shared.Verts))
	}
	if reflect.DeepEqual(advanced, shared) {
		t.Fatal("x1 viewport did not widen its outer windscreen ring")
	}
	for index := 0; index < tieCockpitWindowSides; index++ {
		if radius := math.Hypot(advanced.Verts[index].X, advanced.Verts[index].Y); math.Abs(radius-tieAdvancedWindowOuterRadius) > 1e-9 {
			t.Fatalf("x1 outer viewport radius at %d=%v, want %v", index, radius, tieAdvancedWindowOuterRadius)
		}
	}
	for index := tieCockpitWindowSides; index < len(advanced.Verts); index++ {
		if advanced.Verts[index] != shared.Verts[index] {
			t.Fatalf("x1 inner viewport vertex %d changed while widening only the outer circle", index)
		}
	}
}

func assertPlainAdvancedReactorCollar(t *testing.T, collar Model) {
	t.Helper()
	if len(collar.Verts) != tieRearReactorSides || len(collar.Faces) != 0 {
		t.Fatalf("x1 reactor collar vertices/faces=%d/%d, want one line-only %d-sided ring", len(collar.Verts), len(collar.Faces), tieRearReactorSides)
	}
	if len(collar.Edges) != tieRearReactorSides {
		t.Fatalf("x1 reactor collar edges=%d, want %d", len(collar.Edges), tieRearReactorSides)
	}
	for _, edge := range collar.Edges {
		if edge.Kind == EdgeInternal || edge.A < 0 || edge.B >= tieRearReactorSides {
			t.Fatalf("x1 reactor collar has invalid perimeter edge %+v", edge)
		}
	}
}

func assertAdvancedRootCrossSection(t *testing.T, root Model) {
	t.Helper()
	const verticesPerStation = 6
	if len(root.Verts) != 4*verticesPerStation {
		t.Fatalf("x1 root vertices=%d, want four hexagonal stations", len(root.Verts))
	}
	for station := 0; station < 4; station++ {
		base := station * verticesPerStation
		front, upperFront, upperRear, rear := root.Verts[base], root.Verts[base+1], root.Verts[base+2], root.Verts[base+3]
		topSpan := upperFront.Sub(upperRear).Length()
		frontFacet := front.Sub(upperFront).Length()
		rearFacet := upperRear.Sub(rear).Length()
		if frontFacet <= topSpan || rearFacet <= topSpan {
			t.Fatalf("station %d does not keep its fore/aft facets larger than the top face", station)
		}
		if station > 0 {
			previousFront := root.Verts[base-verticesPerStation]
			previousUpperFront := root.Verts[base-verticesPerStation+1]
			if math.Abs(upperFront.Y) >= math.Abs(previousUpperFront.Y) || front.Z >= previousFront.Z {
				t.Fatalf("station %d does not taper toward the panel root", station)
			}
		}
	}
}

func TestTIEAdvancedX1FuselageCheckpointDimensions(t *testing.T) {
	geometry := TIEAdvancedX1GeometryData()
	fuselageMin, fuselageMax := tieAdvancedBounds(geometry.Fuselage)
	fullMin, fullMax := tieAdvancedBounds(geometry.Full)
	if got := 2 * tieCockpitRadius; math.Abs(got-1.36) > 1e-9 {
		t.Fatalf("cockpit diameter=%v, want 1.36", got)
	}
	if got := fullMax.Z - fullMin.Z; math.Abs(got-3.64) > 1e-9 {
		t.Fatalf("cockpit plus panel length=%v, want 3.64", got)
	}
	if got := fuselageMax.X - fuselageMin.X; math.Abs(got-tieAdvancedFuselageMaxWidth) > 1e-9 {
		t.Fatalf("fuselage width=%v, want %v", got, tieAdvancedFuselageMaxWidth)
	}
	if math.Abs(fuselageMin.X+fuselageMax.X) > 1e-9 {
		t.Fatalf("fuselage is not symmetric about X=0: min=%v max=%v", fuselageMin.X, fuselageMax.X)
	}
	if got := fuselageMax.Y - fuselageMin.Y; math.Abs(got-tieAdvancedFuselageMaxHeight) > 1e-9 {
		t.Fatalf("fuselage height=%v, want %v", got, tieAdvancedFuselageMaxHeight)
	}
	if math.Abs(fuselageMin.Z-tieAdvancedFuselageRearZ) > 1e-9 {
		t.Fatalf("rear extremity=%v, want %v", fuselageMin.Z, tieAdvancedFuselageRearZ)
	}
	arrayMin, _ := tieAdvancedBounds(geometry.SolarArrays)
	if got := fuselageMin.Z - arrayMin.Z; math.Abs(got-0.95) > 1e-9 {
		t.Fatalf("solar panels extend %v behind fuselage, want 0.95", got)
	}
	if got := tieAdvancedThicknessAtZ(geometry.Fuselage, tieAdvancedFuselageRearZ); math.Abs(got-tieAdvancedFuselageRearHeight) > 1e-9 {
		t.Fatalf("rear fuselage thickness=%v, want %v", got, tieAdvancedFuselageRearHeight)
	}
	rearMin, rearMax := tieAdvancedBounds(geometry.RearStrut)
	if got := rearMax.Z - rearMin.Z; math.Abs(got-tieAdvancedRearStrutLength) > 1e-9 {
		t.Fatalf("rear strut length=%v, want %v", got, tieAdvancedRearStrutLength)
	}
	if math.Abs(rearMin.X+rearMax.X) > 1e-9 {
		t.Fatalf("rear strut is not symmetric about X=0: min=%v max=%v", rearMin.X, rearMax.X)
	}
	for _, edge := range geometry.RearStrut.Edges {
		if edge.A == 12 || edge.B == 12 || (edge.A >= 6 && edge.A < 12 && edge.B >= 6 && edge.B < 12) {
			if edge.Kind != EdgeInternal {
				t.Fatalf("embedded forward strut-cap edge %+v remains visible", edge)
			}
		}
	}
}

func tieAdvancedThicknessAtZ(mesh Model, z float64) float64 {
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, vertex := range mesh.Verts {
		if math.Abs(vertex.Z-z) > 1e-9 {
			continue
		}
		minY = math.Min(minY, vertex.Y)
		maxY = math.Max(maxY, vertex.Y)
	}
	return maxY - minY
}

func tieAdvancedBounds(mesh Model) (math3d.Vec3, math3d.Vec3) {
	min, max := mesh.Verts[0], mesh.Verts[0]
	for _, vertex := range mesh.Verts[1:] {
		min.X, min.Y, min.Z = math.Min(min.X, vertex.X), math.Min(min.Y, vertex.Y), math.Min(min.Z, vertex.Z)
		max.X, max.Y, max.Z = math.Max(max.X, vertex.X), math.Max(max.Y, vertex.Y), math.Max(max.Z, vertex.Z)
	}
	return min, max
}
