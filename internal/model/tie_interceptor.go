package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

const tieInterceptorWingX = 1.68
const tieInterceptorWingY = 0.48

const tieInterceptorWingAngle = math.Pi * 45 / 180
const tieInterceptorPanelTipZ = 3.12
const tieInterceptorCannonLength = 0.52
const tieInterceptorCannonMuzzleZ = 3.18
const tieInterceptorCoreForwardZ = 0.45
const tieInterceptorWindowLocalZ = 0.58

// Authored placement constants shared with combat anchors and camera poses.
const (
	TIEInterceptorWingX         = tieInterceptorWingX
	TIEInterceptorWingY         = tieInterceptorWingY
	TIEInterceptorWingAngle     = tieInterceptorWingAngle
	TIEInterceptorPanelTipZ     = tieInterceptorPanelTipZ
	TIEInterceptorCannonLength  = tieInterceptorCannonLength
	TIEInterceptorCannonMuzzleZ = tieInterceptorCannonMuzzleZ
	TIEInterceptorCoreForwardZ  = tieInterceptorCoreForwardZ
)

// The pylon is deliberately buried a little inside the solar-panel root. A
// flush cap would be coplanar with the panel attachment and could leak its
// circular outline through the wing during depth classification.
const tieInterceptorPylonWingX = tieInterceptorWingX - 0.14

// TIEInterceptorGeometry contains the immutable prepared components used by
// the catalog, showcase, combat anchors, and destruction lifecycle.
type TIEInterceptorGeometry struct {
	Full      Model
	Core      Model
	Cockpit   Model
	Pylons    Model
	Window    Model
	Panels    [4]Model
	EndPanels [2]Model
	Cannons   [4]Model
	Fragments [3]Model
}

// TIEInterceptorGeometryData builds the sparse, surface-aware Imperial
// Interceptor. The four thin panel solids are deliberately separate so their
// gaps remain real gaps for culling, depth, and point occlusion.
func TIEInterceptorGeometryData() TIEInterceptorGeometry {
	cockpit, pylons := tieInterceptorCoreParts()
	core := Merge(cockpit, pylons)
	window := Transform(cylinder(0.29, 0.035, 10), math3d.Translation(0, 0, tieInterceptorWindowLocalZ+tieInterceptorCoreForwardZ))
	var panels [4]Model
	var endPanels [2]Model
	var cannons [4]Model
	placements := []struct {
		hingeX float64
		hingeY float64
		angle  float64
	}{
		// Each panel starts at an outer side hinge and points inward toward
		// the cockpit, matching the V-shaped front view in the schematic.
		{hingeX: tieInterceptorWingX, hingeY: tieInterceptorWingY, angle: math.Pi - tieInterceptorWingAngle},
		{hingeX: tieInterceptorWingX, hingeY: -tieInterceptorWingY, angle: math.Pi + tieInterceptorWingAngle},
		{hingeX: -tieInterceptorWingX, hingeY: tieInterceptorWingY, angle: tieInterceptorWingAngle},
		{hingeX: -tieInterceptorWingX, hingeY: -tieInterceptorWingY, angle: -tieInterceptorWingAngle},
	}
	for index, placement := range placements {
		transform := math3d.Translation(placement.hingeX, placement.hingeY, 0).Mul(math3d.RotationZ(placement.angle))
		panels[index] = Transform(tieInterceptorPanel(), transform)
		// Cannons sit on the pointed forward wing edge. Their rear ends meet
		// the panel point and their barrels continue straight along +Z.
		cannonPosition := transform.TransformPoint(math3d.Vec3{X: 0.10, Z: tieInterceptorCannonMuzzleZ - tieInterceptorCannonLength/2})
		cannons[index] = Transform(cylinder(0.055, tieInterceptorCannonLength, 6), math3d.Translation(cannonPosition.X, cannonPosition.Y, cannonPosition.Z))
	}
	endProfile := tieInterceptorEndPanel()
	endPanels[0] = Transform(endProfile, math3d.Translation(-tieInterceptorWingX, 0, 0))
	endPanels[1] = Transform(endProfile, math3d.Translation(tieInterceptorWingX, 0, 0))
	fullParts := []Model{core, endPanels[0], endPanels[1]}
	for index := range panels {
		fullParts = append(fullParts, panels[index], cannons[index])
	}
	full := Merge(fullParts...)
	fragments := [3]Model{
		Merge(endPanels[1], panels[0], panels[1], cannons[0], cannons[1]),
		core,
		Merge(endPanels[0], panels[2], panels[3], cannons[2], cannons[3]),
	}
	return TIEInterceptorGeometry{Full: full, Core: core, Cockpit: cockpit, Pylons: pylons, Window: window, Panels: panels, EndPanels: endPanels, Cannons: cannons, Fragments: fragments}
}

// TIEInterceptor returns the complete surface-aware model without the
// contrasting cockpit window layer.
func TIEInterceptor() Model {
	return TIEInterceptorGeometryData().Full
}

func tieInterceptorCore() Model {
	// Reuse the authored TIE cockpit, whose longitudinal X axis is rotated to
	// the shared +Z nose direction for the Interceptor.
	cockpit, pylons := tieInterceptorCoreParts()
	return Merge(cockpit, pylons)
}

func tieInterceptorCoreParts() (Model, Model) {
	cockpit := Transform(TIEFighterCockpit(), math3d.Translation(0, 0, tieInterceptorCoreForwardZ).Mul(math3d.RotationY(-math.Pi/2)))
	parts := []Model{}
	// A shallow rear reactor ring and two strong horizontal attachment struts
	// give the central pod the characteristic front-view silhouette. The upper
	// and lower panel pairs attach at the ends of these side struts rather than
	// converging as four independent spokes.
	parts = append(parts, Transform(cylinder(0.36, 0.14, 10), math3d.Translation(0, 0, -0.54+tieInterceptorCoreForwardZ)))
	for _, angle := range []float64{0, math.Pi} {
		parts = append(parts, Transform(tieInterceptorPylon(), math3d.Translation(0, 0, tieInterceptorCoreForwardZ).Mul(math3d.RotationZ(angle))))
	}
	return cockpit, Merge(parts...)
}

func tieInterceptorPylon() Model {
	// The brace is one continuous circular solid. It narrows quickly after the
	// broad cockpit interface, has a short straight middle section, then flares
	// back out slightly at the wing attachment instead of ending in a needle.
	return profiledCylinderX([]cylinderRing{
		{X: 0.50, Radius: 0.45},
		{X: 0.76, Radius: 0.22},
		{X: 0.96, Radius: 0.22},
		{X: tieInterceptorPylonWingX, Radius: 0.34},
	}, 10)
}

func tieInterceptorPanel() Model {
	// The side profile is authored with +Z as the nose/forward direction (left
	// in the supplied side schematic). It starts at the outer wing plate and
	// sweeps inward along local X. The Interceptor's panels are not rectangular
	// TIE foils: each is a long, narrow dagger with a deliberately sharp
	// forward point and a short, squared aft section. Keeping that outline in
	// one solid (rather than overlapping flat line drawings) is important for
	// both the hidden-line renderer and the panel gaps visible in the reference.
	profile := []math3d.Vec3{
		{X: 0.00, Z: -0.62},                   // outer/aft corner at the end plate
		{X: 1.48, Z: -0.62},                   // short squared trailing edge
		{X: 1.48, Z: 0.18},                    // rear shoulder toward the command pod
		{X: 0.10, Z: tieInterceptorPanelTipZ}, // long, pointed forward wing tip
		{X: 0.00, Z: 1.04},                    // outer leading shoulder
	}
	mesh := extrudeXZ(profile, 0.085)
	// Surface braces are intentional decorative lines; the surrounding prism
	// faces remain the physical occluding topology.
	mesh.Edges = append(mesh.Edges,
		Edge{A: 0, B: 2, Kind: EdgeDecorative},
		Edge{A: 2, B: 4, Kind: EdgeDecorative},
		Edge{A: 1, B: 3, Kind: EdgeDecorative},
	)
	mesh.Topology = nil
	return Prepare(mesh)
}

// tieInterceptorEndPanel is the flat outer plate visible at each side pylon
// in the front view. It closes the upper/lower panel assembly without making
// the open space between the main solar-array surfaces opaque.
func tieInterceptorEndPanel() Model {
	profile := []math3d.Vec3{
		{Y: -tieInterceptorWingY, Z: -0.62},
		{Y: tieInterceptorWingY, Z: -0.62},
		{Y: tieInterceptorWingY, Z: 1.04},
		{Y: -tieInterceptorWingY, Z: 1.04},
	}
	return extrudeYZ(profile, 0.10)
}

// extrudeXZ makes a finite-thickness solid from a profile in the XZ plane.
// The local X axis is radial from the command pod and Z remains the fighter's
// forward axis; rotating the result around Z supplies the four panels.
func extrudeXZ(profile []math3d.Vec3, thickness float64) Model {
	mesh := Model{}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Add(math3d.Vec3{Y: thickness / 2}))
	}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Sub(math3d.Vec3{Y: thickness / 2}))
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
	front := make([]int, count)
	back := make([]int, count)
	for index := range profile {
		front[index] = index
		back[count-1-index] = count + index
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: front}, Face{Vertices: back})
	return OrientOutward(mesh)
}

func extrudeYZ(profile []math3d.Vec3, thickness float64) Model {
	mesh := Model{}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Add(math3d.Vec3{X: thickness / 2}))
	}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Sub(math3d.Vec3{X: thickness / 2}))
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
	front := make([]int, count)
	back := make([]int, count)
	for index := range profile {
		front[index] = index
		back[count-1-index] = count + index
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: front}, Face{Vertices: back})
	return OrientOutward(mesh)
}

type cylinderRing struct {
	X      float64
	Radius float64
}

// profiledCylinderX creates a closed circular solid whose axis is X and whose
// radius can change across several authored stations. Adjacent equal-radius
// stations form a true cylindrical section; differing stations form a smooth
// frustum section.
func profiledCylinderX(profile []cylinderRing, segments int) Model {
	mesh := Model{}
	for _, ring := range profile {
		for segment := 0; segment < segments; segment++ {
			angle := 2 * math.Pi * float64(segment) / float64(segments)
			sine, cosine := math.Sincos(angle)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: ring.X, Y: ring.Radius * cosine, Z: ring.Radius * sine})
		}
	}
	for ring := 0; ring < len(profile)-1; ring++ {
		base := ring * segments
		nextBase := (ring + 1) * segments
		for segment := 0; segment < segments; segment++ {
			next := (segment + 1) % segments
			mesh.Edges = append(mesh.Edges,
				Edge{A: base + segment, B: base + next},
				Edge{A: nextBase + segment, B: nextBase + next},
				Edge{A: base + segment, B: nextBase + segment},
			)
			mesh.Faces = append(mesh.Faces, Face{Vertices: []int{base + segment, base + next, nextBase + next, nextBase + segment}})
		}
	}
	rear := make([]int, segments)
	for segment := 0; segment < segments; segment++ {
		rear[segments-1-segment] = (len(profile)-1)*segments + segment
	}
	// The first ring is an intentional attachment boundary into the cockpit;
	// do not cap it with a buried polygon that could show through the hull.
	mesh.Faces = append(mesh.Faces, Face{Vertices: rear})
	return OrientOutward(mesh)
}
