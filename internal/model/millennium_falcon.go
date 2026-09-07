package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// MillenniumFalconGeometry groups the immutable, independently drawable
// components of the Falcon. The parts are deliberately separate solids: the
// mandible gap stays open, the cockpit can be hidden by the hull, and the
// turrets/engine grille can participate in normal face-aware visibility.
type MillenniumFalconGeometry struct {
	Full          Model
	Hull          Model
	LeftMandible  Model
	RightMandible Model
	Corridor      Model
	Cockpit       Model
	Window        Model
	Turrets       Model
	Engine        Model
	Details       Model
	Fragments     [3]Model
}

// MillenniumFalconStations are semantic local positions shared by geometry
// and catalog anchors. Keeping them here prevents cockpit-camera anchors from
// drifting when the corridor or pod is reshaped.
type MillenniumFalconStations struct {
	CorridorOrigin  math3d.Vec3
	CorridorBend    math3d.Vec3
	CorridorEnd     math3d.Vec3
	CockpitCenter   math3d.Vec3
	WindscreenMount math3d.Vec3
	SensorDish      math3d.Vec3
}

// MillenniumFalconStationsData returns the authoritative local frame used by
// the cockpit corridor, pod, windscreen, and dorsal sensor dish.
func MillenniumFalconStationsData() MillenniumFalconStations {
	base := falconCockpitBase()
	elbow := base.Add(falconCockpitDirection().Scale(falconCockpitDiagonalLength()))
	end := elbow.Add(math3d.Vec3{Z: falconCockpitForwardLength()})
	podStation := falconCockpitDiagonalLength() + falconCockpitForwardLength() + 0.05
	return MillenniumFalconStations{
		CorridorOrigin:  base,
		CorridorBend:    elbow,
		CorridorEnd:     end,
		CockpitCenter:   falconCockpitStationPosition(podStation + 0.39),
		WindscreenMount: falconCockpitStationPosition(podStation + 0.70),
		SensorDish:      falconSensorDishPosition(),
	}
}

// MillenniumFalconGeometryData creates a sparse, surface-aware YT-1300F
// silhouette. +Z is forward, +X is starboard, and +Y is dorsal. The geometry
// emphasizes the recognisable outline over dense greeble detail: a flattened
// saucer hull, split forward mandibles, offset cockpit, dorsal/ventral quad
// turrets, and the broad aft engine grille.
func MillenniumFalconGeometryData() MillenniumFalconGeometry {
	hull := millenniumFalconHull()
	leftMandible := millenniumFalconMandible(-1)
	rightMandible := millenniumFalconMandible(1)
	corridor := millenniumFalconCockpitCorridor()
	cockpit := millenniumFalconCockpit()
	window := millenniumFalconWindow()
	turrets := millenniumFalconTurrets()
	engine := millenniumFalconEngine()
	details := millenniumFalconDetails()
	full := Merge(hull, leftMandible, rightMandible, corridor, cockpit, window, turrets, engine, details)
	fragments := [3]Model{
		Merge(leftMandible, millenniumFalconTurret(-1)),
		Merge(hull, corridor, cockpit, window, engine, details),
		Merge(rightMandible, millenniumFalconTurret(1)),
	}
	return MillenniumFalconGeometry{
		Full: full, Hull: hull, LeftMandible: leftMandible, RightMandible: rightMandible, Corridor: corridor,
		Cockpit: cockpit, Window: window, Turrets: turrets, Engine: engine,
		Details: details, Fragments: fragments,
	}
}

// MillenniumFalcon returns the complete prepared model, including the
// contrasting windscreen geometry. Catalog users normally consume the
// component family above so each solid receives its own depth owner.
func MillenniumFalcon() Model {
	return MillenniumFalconGeometryData().Full
}

func millenniumFalconHull() Model {
	// The reference views show a convex saucer: the central hull is deep,
	// while the outer armor tapers to a thin rim. A uniform-depth cylinder
	// loses that profile and reads like a flat disk from the front.
	return Transform(profiledSaucer(), math3d.Scaling(1.05, 1, 0.95))
}

// profiledSaucer creates a closed, rotationally symmetric hull around the Y
// axis. Each radial station has its own half-depth, producing a deeper centre
// and a thin perimeter without resorting to a filled billboard or open sheet.
func profiledSaucer() Model {
	type station struct {
		radius float64
		halfY  float64
	}
	stations := []station{
		{radius: 0.0, halfY: 0.76},
		{radius: 1.65, halfY: 0.70},
		{radius: 3.15, halfY: 0.53},
		{radius: 4.20, halfY: 0.18},
	}
	const segments = 18
	mesh := Model{}

	// The centre station is represented by one top and one bottom pole; the
	// remaining stations are circular rings in the XZ plane.
	topPole := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Y: stations[0].halfY})
	bottomPole := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Y: -stations[0].halfY})
	for _, station := range stations[1:] {
		for _, y := range []float64{station.halfY, -station.halfY} {
			for segment := 0; segment < segments; segment++ {
				angle := 2 * math.Pi * float64(segment) / float64(segments)
				sine, cosine := math.Sincos(angle)
				mesh.Verts = append(mesh.Verts, math3d.Vec3{
					X: station.radius * cosine,
					Y: y,
					Z: station.radius * sine,
				})
			}
		}
	}

	// Connect the top and bottom surfaces between adjacent radial rings.
	for ring := 0; ring < len(stations)-2; ring++ {
		base := 2 + ring*segments*2
		next := 2 + (ring+1)*segments*2
		for segment := 0; segment < segments; segment++ {
			nextSegment := (segment + 1) % segments
			mesh.Edges = append(mesh.Edges,
				Edge{A: base + segment, B: base + nextSegment},
				Edge{A: next + segment, B: next + nextSegment},
				Edge{A: base + segment, B: next + segment},
				Edge{A: base + segments + segment, B: base + segments + nextSegment},
				Edge{A: next + segments + segment, B: next + segments + nextSegment},
				Edge{A: base + segments + segment, B: next + segments + segment},
			)
			mesh.Faces = append(mesh.Faces,
				Face{Vertices: []int{base + segment, base + nextSegment, next + nextSegment, next + segment}},
				Face{Vertices: []int{base + segments + segment, next + segments + segment, next + segments + nextSegment, base + segments + nextSegment}},
			)
		}
	}

	// Join the poles to the first ring.
	firstRing := 2
	for segment := 0; segment < segments; segment++ {
		nextSegment := (segment + 1) % segments
		mesh.Edges = append(mesh.Edges,
			Edge{A: topPole, B: firstRing + segment},
			Edge{A: bottomPole, B: firstRing + segments + segment},
		)
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{topPole, firstRing + segment, firstRing + nextSegment}},
			Face{Vertices: []int{bottomPole, firstRing + segments + nextSegment, firstRing + segments + segment}},
		)
	}

	// Close the thin outer armor rim between its top and bottom rings.
	outer := 2 + (len(stations)-2)*segments*2
	for segment := 0; segment < segments; segment++ {
		nextSegment := (segment + 1) % segments
		mesh.Edges = append(mesh.Edges,
			Edge{A: outer + segment, B: outer + nextSegment},
			Edge{A: outer + segments + segment, B: outer + segments + nextSegment},
			Edge{A: outer + segment, B: outer + segments + segment},
		)
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{outer + segment, outer + nextSegment, outer + segments + nextSegment, outer + segments + segment}})
	}
	return OrientOutward(mesh)
}

func millenniumFalconMandible(side int) Model {
	if side != -1 && side != 1 {
		panic("model: Millennium Falcon mandible side must be -1 or +1")
	}
	// In plan view each mandible is a broad root that tapers to a distinct
	// forward point. Keeping the profile triangular preserves the open channel
	// between the two prongs while giving the Falcon its characteristic forked
	// nose instead of a pair of rectangular bars.
	profile := []math3d.Vec3{
		{X: 0.72, Z: 1.08},
		// The outer root is just outside the saucer ellipse. Its line to the
		// forward apex follows the local hull tangent before tapering inward.
		{X: 4.24, Z: 1.20},
		// The schematic's prongs run well beyond the saucer and nearly meet at
		// the nose; move the apex forward and inward to retain that narrow fork.
		{X: 1.00, Z: 7.20},
	}
	mesh := extrudeXZ(profile, 0.68)
	if side < 0 {
		mesh = Transform(mesh, math3d.Scaling(-1, 1, 1))
		mesh = OrientOutward(mesh)
	}
	return mesh
}

func millenniumFalconCockpit() Model {
	// The pod is a short, faceted tube at the corridor's forward end. Its
	// larger rear ring blends into the corridor; the smaller forward ring
	// leaves a flat transverse face for the windscreen to seat against.
	pod := profiledCylinderZ([]falconConeRing{
		{Z: 0, Radius: 0.30},
		{Z: 0.78, Radius: 0.25},
	}, 8, math3d.Vec3{})
	pod = Transform(pod, math3d.Scaling(1.18, 0.92, 1))
	// Keep a small, visible overlap with the corridor endpoint rather than
	// burying the entire straight section inside the pod.
	podStation := falconCockpitDiagonalLength() + falconCockpitForwardLength() + 0.05
	return Transform(pod, millenniumFalconCockpitFrame(podStation))
}

func millenniumFalconCockpitCorridor() Model {
	// In plan view the corridor first leaves the saucer diagonally, then bends
	// into a straight forward-running section. Sweep one octagonal tube through
	// those stations so the elbow is rounded and the pod can transition without
	// an oversized rectangular sleeve or overlapping seam.
	base := falconCockpitBase()
	diagonal := falconCockpitDirection()
	elbow := base.Add(diagonal.Scale(falconCockpitDiagonalLength()))
	forwardEnd := elbow.Add(math3d.Vec3{Z: falconCockpitForwardLength()})
	return sweptFalconTube([]math3d.Vec3{base, elbow, forwardEnd}, 0.27, 8)
}

func millenniumFalconWindow() Model {
	// The windscreen is seated on the pod's flat transverse forward face. It
	// follows the same corridor frame as the cockpit, while its rear ring
	// overlaps the pod face so no gap opens during rotation.
	canopy := profiledCylinderZ([]falconConeRing{
		{Z: 0, Radius: 0.34},
		{Z: 0.74, Radius: 0.22},
	}, 8, math3d.Vec3{})
	podStation := falconCockpitDiagonalLength() + falconCockpitForwardLength() + 0.05
	return Transform(canopy, millenniumFalconCockpitFrame(podStation+0.70))
}

// millenniumFalconCockpitFrame maps a local cockpit station onto the diagonal
// corridor visible in both reference views. Local +Z points from the saucer
// toward the cockpit nose; local +X spans the corridor laterally.
func millenniumFalconCockpitFrame(distance float64) math3d.Mat4 {
	position := falconCockpitStationPosition(distance)
	if distance <= falconCockpitDiagonalLength() {
		return math3d.Translation(position.X, position.Y, position.Z).Mul(math3d.RotationY(falconCockpitAngle()))
	}
	return math3d.Translation(position.X, position.Y, position.Z)
}

func falconCockpitStationPosition(distance float64) math3d.Vec3 {
	base := falconCockpitBase()
	diagonalLength := falconCockpitDiagonalLength()
	if distance <= diagonalLength {
		return base.Add(falconCockpitDirection().Scale(distance))
	}
	elbow := base.Add(falconCockpitDirection().Scale(diagonalLength))
	return elbow.Add(math3d.Vec3{Z: distance - diagonalLength})
}

func falconCockpitAngle() float64 { return 0.68 }

func falconCockpitBase() math3d.Vec3 { return math3d.Vec3{X: 3.15, Y: 0, Z: 2.20} }

func falconCockpitDirection() math3d.Vec3 {
	angle := falconCockpitAngle()
	return math3d.Vec3{X: math.Sin(angle), Z: math.Cos(angle)}
}

func falconCockpitDiagonalLength() float64 { return 0.85 }

func falconCockpitForwardLength() float64 { return 0.1375 }

// sweptFalconTube builds a closed tube along a short polyline in the XZ
// plane. The middle ring uses a mitered tangent, keeping the diagonal-to-
// forward bend continuous while retaining finite polygon faces for culling.
func sweptFalconTube(path []math3d.Vec3, radius float64, segments int) Model {
	if len(path) < 2 || radius <= 0 || segments < 3 {
		panic("model: invalid Falcon cockpit tube parameters")
	}
	mesh := Model{}
	for index, center := range path {
		tangent := path[intMin(index+1, len(path)-1)].Sub(path[intMax(index-1, 0)])
		if tangent.Length() == 0 {
			tangent = math3d.Vec3{Z: 1}
		}
		tangent = tangent.Normalize()
		lateral := math3d.Vec3{X: tangent.Z, Z: -tangent.X}
		for segment := 0; segment < segments; segment++ {
			angle := 2 * math.Pi * float64(segment) / float64(segments)
			sine, cosine := math.Sincos(angle)
			mesh.Verts = append(mesh.Verts, center.Add(lateral.Scale(radius*cosine)).Add(math3d.Vec3{Y: radius * sine}))
		}
	}
	for ring := 0; ring < len(path)-1; ring++ {
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
	front := make([]int, segments)
	rear := make([]int, segments)
	for segment := 0; segment < segments; segment++ {
		front[segments-1-segment] = (len(path)-1)*segments + segment
		rear[segment] = segment
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: front}, Face{Vertices: rear})
	return OrientOutward(mesh)
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func millenniumFalconTurrets() Model {
	return Merge(millenniumFalconTurret(1), millenniumFalconTurret(-1))
}

func millenniumFalconTurret(sign int) Model {
	if sign != -1 && sign != 1 {
		panic("model: Millennium Falcon turret sign must be -1 or +1")
	}
	y := 0.72 * float64(sign)
	housing := Transform(cylinder(0.66, 0.20, 12), math3d.Translation(0, y, 0.30).Mul(math3d.RotationX(math.Pi/2)))
	// Two short parallel barrels make the quad cannon read clearly without
	// flooding the vector budget. Both turrets point along the ship's nose.
	barrels := make([]Model, 0, 4)
	for _, x := range []float64{-0.20, 0.20} {
		for _, offset := range []float64{-0.11, 0.11} {
			barrels = append(barrels, Transform(cylinder(0.065, 1.05, 6), math3d.Translation(x, y+offset, 0.82)))
		}
	}
	parts := append([]Model{housing}, barrels...)
	return Merge(parts...)
}

func millenniumFalconEngine() Model {
	// The broad aft radiator is a shallow solid with explicit grille strokes.
	mesh := extrudeXY([]math3d.Vec3{
		{X: -2.55, Y: -0.34}, {X: 2.55, Y: -0.34},
		{X: 2.55, Y: 0.34}, {X: -2.55, Y: 0.34},
	}, 0.18)
	mesh = Transform(mesh, math3d.Translation(0, 0, -3.72))
	for index := -4; index <= 4; index++ {
		x := float64(index) * 0.48
		mesh.Verts = append(mesh.Verts,
			math3d.Vec3{X: x, Y: -0.30, Z: -3.83},
			math3d.Vec3{X: x, Y: 0.30, Z: -3.83},
		)
		base := len(mesh.Verts) - 2
		mesh.Edges = append(mesh.Edges, Edge{A: base, B: base + 1, Kind: EdgeDecorative})
	}
	mesh.Topology = nil
	return Prepare(mesh)
}

func millenniumFalconDetails() Model {
	mesh := Model{}
	// A raised dorsal sensor dish and a few structural spokes sell the layered,
	// retrofitted hull while remaining decorative lines rather than fake opaque
	// surfaces.
	dishPosition := falconSensorDishPosition()
	dish := Transform(parabolicSensorDish(), math3d.Translation(dishPosition.X, dishPosition.Y, dishPosition.Z))
	mesh = Merge(dish)
	for _, x := range []float64{-2.4, -1.2, 0, 1.2, 2.4} {
		mesh.Verts = append(mesh.Verts,
			math3d.Vec3{X: x, Y: 0.54, Z: -2.70},
			math3d.Vec3{X: x * 0.85, Y: 0.54, Z: 1.95},
		)
		base := len(mesh.Verts) - 2
		mesh.Edges = append(mesh.Edges, Edge{A: base, B: base + 1, Kind: EdgeDecorative})
	}
	mesh.Topology = nil
	return Prepare(mesh)
}

func falconSensorDishPosition() math3d.Vec3 {
	// Forward is +Z. The top-view schematic places the dish on the forward-left
	// quadrant of the saucer rather than on the aft half.
	return math3d.Vec3{X: -1.55, Y: 0.70, Z: 1.45}
}

// parabolicSensorDish is a sparse, finite solid whose bowl opens along +Z
// (the Falcon's forward direction). Concentric rings and a short emitter keep
// the classic vector silhouette while the closed rim/back provide stable
// topology for culling and hidden-line removal.
func parabolicSensorDish() Model {
	const segments = 12
	mesh := Model{}
	apex := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{})
	for _, station := range []struct{ radius, z float64 }{{0.24, 0.055}, {0.50, 0.20}, {0.50, 0.12}} {
		for segment := 0; segment < segments; segment++ {
			angle := 2 * math.Pi * float64(segment) / float64(segments)
			sine, cosine := math.Sincos(angle)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: station.radius * cosine, Y: station.radius * sine, Z: station.z})
		}
	}
	inner := 1
	outer := 1 + segments
	back := outer + segments
	for segment := 0; segment < segments; segment++ {
		next := (segment + 1) % segments
		mesh.Edges = append(mesh.Edges,
			Edge{A: apex, B: inner + segment},
			Edge{A: inner + segment, B: inner + next},
			Edge{A: outer + segment, B: outer + next},
			Edge{A: back + segment, B: back + next},
			Edge{A: outer + segment, B: back + segment},
		)
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{apex, inner + next, inner + segment}},
			Face{Vertices: []int{inner + segment, inner + next, outer + next, outer + segment}},
			Face{Vertices: []int{outer + segment, outer + next, back + next, back + segment}},
		)
	}
	backFace := make([]int, segments)
	for segment := range backFace {
		backFace[segments-1-segment] = back + segment
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: backFace})
	emitter := Transform(cylinder(0.07, 0.18, 8), math3d.Translation(0, 0, 0.20))
	return Merge(OrientOutward(mesh), emitter)
}

// extrudeXY creates a closed finite-depth solid from a profile in the X/Y
// plane. It complements the shared XZ prism helper used by fighter panels.
func extrudeXY(profile []math3d.Vec3, depth float64) Model {
	mesh := Model{}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Add(math3d.Vec3{Z: depth / 2}))
	}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Sub(math3d.Vec3{Z: depth / 2}))
	}
	count := len(profile)
	for index := range profile {
		next := (index + 1) % count
		mesh.Edges = append(mesh.Edges,
			Edge{A: index, B: next}, Edge{A: count + index, B: count + next}, Edge{A: index, B: count + index},
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

type falconConeRing struct {
	Z      float64
	Radius float64
}

// profiledCylinderZ creates a closed, faceted frustum aligned with the
// Falcon's forward axis. The centre is supplied in X/Y so the same primitive
// can be placed on the offset cockpit without introducing a screen-facing
// overlay.
func profiledCylinderZ(profile []falconConeRing, segments int, center math3d.Vec3) Model {
	mesh := Model{}
	for _, ring := range profile {
		for segment := 0; segment < segments; segment++ {
			angle := 2 * math.Pi * float64(segment) / float64(segments)
			sine, cosine := math.Sincos(angle)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{
				X: center.X + ring.Radius*cosine,
				Y: center.Y + ring.Radius*sine,
				Z: ring.Z,
			})
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
	front := make([]int, segments)
	rear := make([]int, segments)
	for segment := 0; segment < segments; segment++ {
		front[segments-1-segment] = (len(profile)-1)*segments + segment
		rear[segment] = segment
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: front}, Face{Vertices: rear})
	return OrientOutward(mesh)
}
