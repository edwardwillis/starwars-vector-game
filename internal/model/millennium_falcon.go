package model

import (
	"math"
	"sort"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// MillenniumFalconGeometry groups the immutable Falcon components. Hull is
// the single intact depth-owning mesh for the saucer, mandibles, and cargo
// assembly; CargoRamp remains as a semantic source for destruction fragments
// and geometry inspection without being rendered as a duplicate part.
type MillenniumFalconGeometry struct {
	Full          Model
	HullCore      Model
	Hull          Model
	LeftMandible  Model
	RightMandible Model
	CargoRamp     Model // semantic source retained by the destruction system
	Corridor      Model
	Window        Model
	Turrets       Model
	Hyperdrive    Model
	Details       Model
	Fragments     [3]Model
}

// MillenniumFalconStations are semantic local positions shared by geometry
// and catalog anchors. Keeping them here prevents cockpit-camera anchors from
// drifting when the corridor or windscreen is reshaped.
type MillenniumFalconStations struct {
	CorridorOrigin  math3d.Vec3
	CorridorBend    math3d.Vec3
	CorridorEnd     math3d.Vec3
	CockpitCenter   math3d.Vec3
	WindscreenMount math3d.Vec3
	SensorDish      math3d.Vec3
}

// MillenniumFalconStationsData returns the authoritative local frame used by
// the cockpit corridor, windscreen, and dorsal sensor dish.
func MillenniumFalconStationsData() MillenniumFalconStations {
	base := falconCockpitBase()
	elbow := base.Add(falconCockpitDirection().Scale(falconCockpitDiagonalLength()))
	end := elbow.Add(math3d.Vec3{Z: falconCockpitForwardLength()})
	cockpitStation := falconCockpitDiagonalLength() + falconCockpitForwardLength()
	return MillenniumFalconStations{
		CorridorOrigin:  base,
		CorridorBend:    elbow,
		CorridorEnd:     end,
		CockpitCenter:   falconCockpitStationPosition(cockpitStation + falconCockpitWindscreenLength()/2),
		WindscreenMount: falconCockpitStationPosition(cockpitStation),
		SensorDish:      falconSensorDishPosition(),
	}
}

// MillenniumFalconGeometryData creates a sparse, surface-aware YT-1300F
// silhouette. +Z is forward, +X is starboard, and +Y is dorsal. The geometry
// emphasizes the recognisable outline over dense greeble detail: a flattened
// saucer hull, split forward mandibles, offset windscreen cockpit, dorsal/ventral quad
// turrets, and the dorsal sensor dish.
func MillenniumFalconGeometryData() MillenniumFalconGeometry {
	hullCore := millenniumFalconHull()
	leftMandible := millenniumFalconMandible(-1)
	rightMandible := millenniumFalconMandible(1)
	cargoRamp := millenniumFalconCargoRamp()
	// The intact hull owns the mandibles and cargo assembly as one composite
	// solid. Keeping the ramp/roof in the same topology gives the depth pass a
	// single physical surface to resolve, while retaining CargoRamp below as a
	// semantic source for destruction fragments and inspection tools.
	hull := MergeWelded(hullCore, leftMandible, rightMandible, cargoRamp)
	corridor := millenniumFalconCockpitCorridor()
	window := millenniumFalconWindow()
	turrets := millenniumFalconTurrets()
	hyperdrive := millenniumFalconHyperdrive()
	details := millenniumFalconDetails()
	full := Merge(hull, corridor, window, turrets, hyperdrive, details)
	fragments := [3]Model{
		Merge(leftMandible, millenniumFalconTurret(-1)),
		Merge(hullCore, cargoRamp, corridor, window, hyperdrive, details),
		Merge(rightMandible, millenniumFalconTurret(1)),
	}
	return MillenniumFalconGeometry{
		Full: full, HullCore: hullCore, Hull: hull, LeftMandible: leftMandible, RightMandible: rightMandible, CargoRamp: cargoRamp, Corridor: corridor,
		Window: window, Turrets: turrets, Hyperdrive: hyperdrive,
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
	return Transform(profiledSaucer(), math3d.Scaling(falconHullScaleX, 1, falconHullScaleZ))
}

const (
	falconHullScaleX = 1.05
	falconHullScaleZ = 0.95
	falconHullRadius = 4.20
)

const (
	falconMandibleInnerAngle = 1.377097324908489
	falconMandibleOuterAngle = 0.8906616339056612
)

func falconHullPointAtAngle(angle float64) math3d.Vec3 {
	sine, cosine := math.Sincos(angle)
	return math3d.Scaling(falconHullScaleX, 1, falconHullScaleZ).TransformPoint(math3d.Vec3{
		X: falconHullRadius * cosine,
		Z: falconHullRadius * sine,
	})
}

// falconInnerHullPointAtX returns an exact point on the front half of the
// inner saucer ring after the hull's authored X/Z aspect scaling. Cargo
// attachment vertices are derived from this helper so MergeWelded can turn
// the ramp/roof-to-hull interfaces into real shared topology.
func falconInnerHullPointAtX(x float64) math3d.Vec3 {
	const radius = 1.85
	localX := x / falconHullScaleX
	localZ := math.Sqrt(radius*radius - localX*localX)
	return math3d.Vec3{X: x, Y: falconInnerHullHalfDepth, Z: localZ * falconHullScaleZ}
}

func falconInnerHullAngleAtX(x float64) float64 {
	const radius = 1.85
	localX := x / falconHullScaleX
	localZ := math.Sqrt(radius*radius - localX*localX)
	return math.Atan2(localZ, localX)
}

func falconHullAngles() []float64 {
	angles := make([]float64, 0, 20)
	for segment := 0; segment < 18; segment++ {
		// Replace the two coarse 60/120 degree edges with the mandible
		// attachment boundaries below; all other original hull facets remain.
		if segment == 3 || segment == 6 {
			continue
		}
		angles = append(angles, 2*math.Pi*float64(segment)/18)
	}
	angles = append(angles,
		falconMandibleOuterAngle,
		falconMandibleInnerAngle,
		math.Pi-falconMandibleInnerAngle,
		math.Pi-falconMandibleOuterAngle,
		falconInnerHullAngleAtX(0.52),
		falconInnerHullAngleAtX(0.52*0.82),
		math.Pi-falconInnerHullAngleAtX(0.52*0.82),
		math.Pi-falconInnerHullAngleAtX(0.52),
	)
	sort.Float64s(angles)
	return angles
}

func falconMandibleAttachmentEdge(a, b float64) bool {
	const epsilon = 1e-9
	return (math.Abs(a-falconMandibleOuterAngle) < epsilon && math.Abs(b-falconMandibleInnerAngle) < epsilon) ||
		(math.Abs(a-(math.Pi-falconMandibleInnerAngle)) < epsilon && math.Abs(b-(math.Pi-falconMandibleOuterAngle)) < epsilon)
}

// profiledSaucer creates a rotationally symmetric saucer shell around the Y
// axis. Each radial station has its own half-depth, producing a deeper centre
// and a thin perimeter without resorting to a filled billboard or open sheet.
// The narrow front channel is represented by depth-only backing faces for the
// integrated cargo assembly; their construction boundaries are suppressed by
// the renderer while MergeWelded closes the physical continuation at its four
// exact inner-ring attachment points.
func profiledSaucer() Model {
	type station struct {
		radius float64
		halfY  float64
	}
	stations := []station{
		{radius: 0.0, halfY: 0.76},
		// Widen the thick central saucer section slightly before the outer
		// armor begins to slope, giving the Falcon a fuller body without
		// enlarging the thin perimeter rim.
		{radius: 1.85, halfY: falconInnerHullHalfDepth},
		{radius: 3.15, halfY: falconMiddleHullHalfDepth},
		{radius: falconHullRadius, halfY: falconHullDiskHalfDepth},
	}
	angles := falconHullAngles()
	segments := len(angles)
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
				angle := angles[segment]
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
			upper := []int{base + segment, base + nextSegment, next + nextSegment, next + segment}
			lower := []int{base + segments + segment, next + segments + segment, next + segments + nextSegment, base + segments + nextSegment}
			upperFace := Face{Vertices: upper}
			lowerFace := Face{Vertices: lower}
			if falconCargoChannelFace(mesh.Verts, upper) {
				upperFace.OccluderOnly = true
			}
			if falconCargoChannelFace(mesh.Verts, lower) {
				lowerFace.OccluderOnly = true
			}
			mesh.Faces = append(mesh.Faces, upperFace, lowerFace)
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
		// The edges whose endpoints are the mandible root angles are replaced
		// by the mandible root faces after welding.
		if !falconMandibleAttachmentEdge(angles[segment], angles[nextSegment]) {
			mesh.Faces = append(mesh.Faces, Face{Vertices: []int{outer + segment, outer + nextSegment, outer + segments + nextSegment, outer + segments + segment}})
		}
	}
	return OrientOutward(mesh)
}

// falconCargoChannelFace identifies the saucer's top/bottom faces directly
// behind the cargo assembly. Those faces remain physical depth surfaces, but
// are marked OccluderOnly so the cargo outer surfaces become the visible
// continuation of the hull without drawing a second internal saucer skin.
func falconCargoChannelFace(vertices []math3d.Vec3, indices []int) bool {
	if len(indices) == 0 {
		return false
	}
	center := math3d.Vec3{}
	for _, index := range indices {
		center = center.Add(vertices[index])
	}
	center = center.Scale(1 / float64(len(indices)))
	return math.Abs(center.X) <= 0.62/falconHullScaleX && center.Z >= 1.68/falconHullScaleZ
}

func millenniumFalconMandible(side int) Model {
	if side != -1 && side != 1 {
		panic("model: Millennium Falcon mandible side must be -1 or +1")
	}
	// In plan view each mandible is a broad root that tapers to a distinct
	// forward point. Keeping the profile triangular preserves the open channel
	// between the two prongs while giving the Falcon its characteristic forked
	// nose instead of a pair of rectangular bars.
	innerRoot := falconHullPointAtAngle(falconMandibleInnerAngle)
	outerRoot := falconHullPointAtAngle(falconMandibleOuterAngle)
	profile := []math3d.Vec3{
		// These roots are the exact outer-ring vertices used by the hull. The
		// root side therefore becomes a shared topological boundary, not an
		// overlapping internal plate.
		{X: innerRoot.X, Z: innerRoot.Z},
		{X: outerRoot.X, Z: outerRoot.Z},
		// The schematic shows a short, slightly swept leading edge rather than
		// a sharp point; retain the blunt, level mandible end here.
		{X: 1.23, Z: 6.80},
		{X: 0.98, Z: 6.80},
	}
	// Keep the prongs as thin forward plates, matching the inner saucer's
	// visible disk thickness rather than making the mandibles read as deep
	// rectangular blocks in front view.
	// The mandible is a thin extension plate. Its full extrusion matches the
	// visible middle-disk height rather than spanning both disk half-depths.
	mesh := extrudeXZ(profile, 2*falconHullDiskHalfDepth)
	if side < 0 {
		mesh = Transform(mesh, math3d.Scaling(-1, 1, 1))
		mesh = OrientOutward(mesh)
	}
	return mesh
}

// millenniumFalconCargoRamp models the raised forward cargo spine visible in
// the gap between the mandibles. The lower plate reads as the boarding ramp
// and the upper solid is its roof. The open forward end remains visible rather
// than being capped by a separate door panel. Both solids participate in
// normal culling and hidden-line removal.
func millenniumFalconCargoRamp() Model {
	// The cargo assembly emerges from the front edge of the inner saucer disk.
	// Starting it farther aft embeds the ramp and roof through the hull and
	// creates the long internal lines visible in side views.
	rampJoin := falconInnerHullPointAtX(0.52)
	roofJoin := falconInnerHullPointAtX(0.52 * 0.82)
	rampProfile := []math3d.Vec3{
		{X: -0.52, Z: rampJoin.Z}, {X: 0.52, Z: rampJoin.Z},
		{X: 0.52, Z: 5.24}, {X: -0.52, Z: 5.16},
	}
	roofProfile := []math3d.Vec3{
		{X: -0.52, Z: roofJoin.Z}, {X: 0.52, Z: roofJoin.Z},
		{X: 0.52, Z: 5.24}, {X: -0.52, Z: 5.16},
	}
	// The lower ramp's outer face and upper roof's outer face meet the exact
	// lower/upper inner-ring planes respectively. Their remaining faces retain
	// the authored offset and taper, preserving the visible cargo profile.
	ramp := suppressFaceBoundaryEdges(taperedSlopedExtrudeXZ(rampProfile, -0.63, -0.29, 0.14, 0.82, 0.90), 4)
	roof := suppressFaceBoundaryEdges(taperedSlopedExtrudeXZ(roofProfile, 0.59, 0.40, 0.22, 0.82, 0.90), 5)
	return Merge(ramp, roof)
}

// suppressFaceBoundaryEdges keeps a face in the physical mesh so it can write
// depth, while marking only its buried rear attachment edge as construction
// topology. The remaining inner-face outline stays visible from the cargo
// opening while the hull-interface seam cannot leak through the opposite plate.
func suppressFaceBoundaryEdges(mesh Model, faceIndex int) Model {
	if faceIndex < 0 || faceIndex >= len(mesh.Faces) {
		return mesh
	}
	prepared := Prepare(mesh)
	face := prepared.Faces[faceIndex]
	if len(face.Vertices) > 0 {
		a, b := face.Vertices[0], face.Vertices[1%len(face.Vertices)]
		for edgeIndex, edge := range prepared.Edges {
			if (edge.A == a && edge.B == b) || (edge.A == b && edge.B == a) {
				prepared.Edges[edgeIndex].Kind = EdgeInternal
				break
			}
		}
	}
	prepared.Topology = nil
	return Prepare(prepared)
}

// slopedExtrudeXZ closes an XZ profile with a constant thickness while
// varying its centre Y linearly from the near to the forward Z station.
func slopedExtrudeXZ(profile []math3d.Vec3, nearY, farY, thickness float64) Model {
	return taperedSlopedExtrudeXZ(profile, nearY, farY, thickness, 1, 1)
}

// taperedSlopedExtrudeXZ is the cargo-specific variant whose upper panel can
// be narrower than its lower panel. The resulting side faces lean inward,
// giving the ramp and roof the angled front/side panels shown in the reference
// rather than leaving them as rectangular boxes. topLengthScale also lets the
// upper forward edge sit aft of the lower edge, increasing the front-panel
// rake without changing the shared lower footprint.
func taperedSlopedExtrudeXZ(profile []math3d.Vec3, nearY, farY, thickness, topScale, topLengthScale float64) Model {
	if len(profile) < 3 || thickness <= 0 {
		panic("model: invalid sloped XZ extrusion")
	}
	if topScale <= 0 || topScale > 1 || topLengthScale <= 0 || topLengthScale > 1 {
		panic("model: invalid tapered XZ extrusion scale")
	}
	minZ, maxZ := profile[0].Z, profile[0].Z
	for _, point := range profile[1:] {
		if point.Z < minZ {
			minZ = point.Z
		}
		if point.Z > maxZ {
			maxZ = point.Z
		}
	}
	span := maxZ - minZ
	if span == 0 {
		panic("model: sloped XZ extrusion has no length")
	}
	mesh := Model{}
	for _, side := range []float64{1, -1} {
		for _, point := range profile {
			fraction := (point.Z - minZ) / span
			centerY := nearY + (farY-nearY)*fraction
			scale := 1.0
			z := point.Z
			if side > 0 {
				scale = topScale
				z = minZ + (point.Z-minZ)*topLengthScale
			}
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: point.X * scale, Y: centerY + side*thickness/2, Z: z})
		}
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

func millenniumFalconCockpitCorridor() Model {
	// In plan view the corridor first leaves the saucer diagonally, then bends
	// into a straight forward-running section. Sweep one octagonal tube through
	// those stations so the elbow is rounded and the windscreen can seat without
	// an oversized rectangular sleeve or overlapping seam.
	diagonal := falconCockpitDirection()
	// Start the swept tube inside the saucer so the diagonal corridor fully
	// penetrates the hull; the bend and cockpit stations remain anchored at
	// their established positions.
	hullBase := falconCockpitBase()
	elbow := hullBase.Add(diagonal.Scale(falconCockpitDiagonalLength()))
	forwardEnd := elbow.Add(math3d.Vec3{Z: falconCockpitForwardLength()})
	// Keep the visible elbow fixed, but aim the hull-running segment exactly
	// through the saucer centre rather than inheriting a small angular skew from
	// the cockpit anchor position.
	elbowRadial := math3d.Vec3{X: elbow.X, Z: elbow.Z}.Normalize()
	base := elbow.Sub(elbowRadial.Scale(falconCockpitDiagonalLength() + falconCockpitHullOverlap()))
	// Leave the forward end open: the windscreen supplies the matching rear
	// closure, avoiding coincident cap faces and any apparent taper at the join.
	return sweptFalconTubeWithCaps([]math3d.Vec3{base, elbow, forwardEnd}, falconCockpitCorridorRadius(), 8, true, false)
}

func millenniumFalconWindow() Model {
	// The windscreen is the cockpit assembly: its rear face abuts the open
	// forward corridor endpoint directly, with no intermediate pod mesh.
	canopy := profiledCylinderZ([]falconConeRing{
		{Z: 0, Radius: falconCockpitCorridorRadius()},
		{Z: falconCockpitWindscreenLength(), Radius: 0.27},
	}, 8, math3d.Vec3{})
	cockpitStation := falconCockpitDiagonalLength() + falconCockpitForwardLength()
	return Transform(canopy, millenniumFalconCockpitFrame(cockpitStation))
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

func falconCockpitAngle() float64 {
	base := falconCockpitBase()
	// The diagonal corridor aims radially away from the saucer centre, so its
	// rear extension points directly back toward the hull centre in plan view.
	return math.Atan2(base.Z, base.X)
}

func falconCockpitBase() math3d.Vec3 { return math3d.Vec3{X: 3.15, Y: 0.32, Z: 2.20} }

const falconMiddleHullHalfDepth = 0.53
const falconInnerHullHalfDepth = 0.70
const falconHullDiskHalfDepth = 0.18

func falconCockpitDirection() math3d.Vec3 {
	angle := falconCockpitAngle()
	return math3d.Vec3{X: math.Sin(angle), Z: math.Cos(angle)}
}

// The top-view reference places the corridor elbow at the saucer's outer
// perimeter; the cockpit then runs forward from that hull-facing station.
func falconCockpitDiagonalLength() float64 { return 0.65 }

// The schema shows a short but distinct straight run between the diagonal
// elbow and the cockpit windscreen. Keep it visibly shorter than the diagonal
// corridor while retaining enough length to read in side/profile views.
func falconCockpitForwardLength() float64 { return 0.30 }

func falconCockpitHullOverlap() float64 { return 0.85 }

func falconCockpitCorridorRadius() float64 { return 0.44 }

func falconCockpitWindscreenLength() float64 { return 0.68 }

// millenniumFalconHyperdrive models the three aft drive segments shown as
// blue blocks in the reference views. Each segment is a closed, thin solid,
// rather than a screen-facing line or one continuous band. The rear hull is
// curved, so the segment centres follow that contour while their back faces
// remain readable as separate rectangles from astern.
func millenniumFalconHyperdrive() Model {
	type segment struct {
		centerX float64
		width   float64
	}
	segments := []segment{
		{centerX: -1.35, width: 1.12},
		{centerX: 0, width: 1.28},
		{centerX: 1.35, width: 1.12},
	}
	const (
		height = 0.34
		depth  = 0.16
	)
	parts := make([]Model, 0, len(segments))
	for _, segment := range segments {
		rearZ := falconRearHullZAtX(segment.centerX) - 0.035
		localX := segment.centerX / falconHullScaleX
		rearRadius := math.Sqrt(math.Max(0, falconHullRadius*falconHullRadius-localX*localX))
		// The local X edge follows the tangent of the rear hull arc. The
		// opposite sign compensates for RotationY's handedness, leaving the
		// outward-facing (-Z) box face normal aligned with the hull surface.
		yaw := -math.Atan2(localX, rearRadius)
		profile := []math3d.Vec3{
			{X: -segment.width / 2, Y: -height / 2},
			{X: segment.width / 2, Y: -height / 2},
			{X: segment.width / 2, Y: height / 2},
			{X: -segment.width / 2, Y: height / 2},
		}
		box := extrudeXY(profile, depth)
		parts = append(parts, Transform(box, math3d.Translation(segment.centerX, 0, rearZ).Mul(math3d.RotationY(yaw))))
	}
	return Merge(parts...)
}

func falconRearHullZAtX(x float64) float64 {
	localX := x / falconHullScaleX
	localZ := -math.Sqrt(math.Max(0, falconHullRadius*falconHullRadius-localX*localX))
	return localZ * falconHullScaleZ
}

// sweptFalconTube builds a closed tube along a short polyline in the XZ
// plane. The middle ring uses a mitered tangent, keeping the diagonal-to-
// forward bend continuous while retaining finite polygon faces for culling.
func sweptFalconTube(path []math3d.Vec3, radius float64, segments int) Model {
	return sweptFalconTubeWithCaps(path, radius, segments, true, true)
}

// sweptFalconTubeWithCaps is the corridor-specific variant that can leave a
// deliberate component interface open. This lets an adjoining solid own the
// shared closure instead of creating coplanar caps that shimmer or make one
// component appear to taper inside the other.
func sweptFalconTubeWithCaps(path []math3d.Vec3, radius float64, segments int, capStart, capEnd bool) Model {
	if len(path) < 2 || radius <= 0 || segments < 3 {
		panic("model: invalid Falcon cockpit tube parameters")
	}
	mesh := Model{}
	for index, center := range path {
		var tangent math3d.Vec3
		if index > 0 && index < len(path)-1 {
			// At a single elbow, use the outgoing direction so the forward
			// section remains cylindrical instead of twisting/tapering inside
			// the cockpit assembly.
			tangent = path[index+1].Sub(path[index])
		} else {
			tangent = path[intMin(index+1, len(path)-1)].Sub(path[intMax(index-1, 0)])
		}
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
				// Circumferential rings are construction topology only. Suppress
				// their strokes so the corridor reads as one continuous tube.
				Edge{A: base + segment, B: base + next, Kind: EdgeInternal},
				Edge{A: nextBase + segment, B: nextBase + next, Kind: EdgeInternal},
				Edge{A: base + segment, B: nextBase + segment},
			)
			mesh.Faces = append(mesh.Faces, Face{Vertices: []int{base + segment, base + next, nextBase + next, nextBase + segment}})
		}
	}
	if capStart || capEnd {
		front := make([]int, segments)
		rear := make([]int, segments)
		for segment := 0; segment < segments; segment++ {
			front[segments-1-segment] = (len(path)-1)*segments + segment
			rear[segment] = segment
		}
		if capEnd {
			mesh.Faces = append(mesh.Faces, Face{Vertices: front})
		}
		if capStart {
			mesh.Faces = append(mesh.Faces, Face{Vertices: rear})
		}
	}
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
	dome := falconGunnerDome(sign)
	// Keep the rear of each barrel seated at the housing, while extending the
	// forward run farther beyond the hull so the quad cannons read prominently
	// in the Falcon silhouette.
	const (
		barrelLength = 1.35
		barrelRearZ  = 0.30
	)
	barrels := make([]Model, 0, 4)
	for _, x := range []float64{-0.20, 0.20} {
		for _, offset := range []float64{-0.11, 0.11} {
			barrels = append(barrels, Transform(cylinder(0.065, barrelLength, 6), math3d.Translation(x, y+offset, barrelRearZ+barrelLength/2)))
		}
	}
	// Each turret deliberately carries four independent barrels (two by two),
	// rather than a single stylised ray. The faceted dome gives the gunner a
	// recognisable glass enclosure while remaining sparse vector geometry.
	parts := append([]Model{housing, dome}, barrels...)
	return Merge(parts...)
}

// falconGunnerDome creates the low-profile transparent-style canopy over one
// quad-laser turret. The dome is a finite faceted shell, not a screen-facing
// circle, so it participates in normal transforms, culling and hidden-line
// removal like the rest of the Falcon.
func falconGunnerDome(sign int) Model {
	if sign != -1 && sign != 1 {
		panic("model: Millennium Falcon turret sign must be -1 or +1")
	}
	const (
		centerY  = 0.72
		radius   = 0.42
		height   = 0.28
		segments = 8
	)
	mesh := Model{}
	for _, fraction := range []float64{0, 0.62} {
		ringRadius := radius * (1 - 0.22*fraction)
		ringY := float64(sign) * (centerY + height*fraction)
		for segment := 0; segment < segments; segment++ {
			angle := 2 * math.Pi * float64(segment) / float64(segments)
			sine, cosine := math.Sincos(angle)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{
				X: ringRadius * cosine,
				Y: ringY,
				Z: ringRadius*sine + 0.30,
			})
		}
	}
	apex := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Y: float64(sign) * (centerY + height), Z: 0.30})
	for segment := 0; segment < segments; segment++ {
		next := (segment + 1) % segments
		mesh.Edges = append(mesh.Edges,
			Edge{A: segment, B: next},
			Edge{A: segments + segment, B: segments + next},
			Edge{A: segment, B: segments + segment},
			Edge{A: segments + segment, B: apex},
		)
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{segment, next, segments + next, segments + segment}},
			Face{Vertices: []int{segments + segment, segments + next, apex}},
		)
	}
	return OrientOutward(mesh)
}

func millenniumFalconDetails() Model {
	// The raised dorsal sensor dish is a separate solid. Long decorative spokes
	// across the saucer were intentionally omitted: they read as stray parallel
	// trails from the aft hull in oblique views.
	dishPosition := falconSensorDishPosition()
	dish := Transform(parabolicSensorDish(), math3d.Translation(dishPosition.X, dishPosition.Y, dishPosition.Z))
	return Prepare(dish)
}

func falconSensorDishPosition() math3d.Vec3 {
	// Forward is +Z. The top-view schematic places the dish on the forward-left
	// quadrant of the saucer rather than on the aft half.
	// Lift the dish clear of the saucer: its half-metre bowl radius needs to
	// sit above the local hull surface, otherwise the opaque hull hides the
	// lower half of the front aperture in pitched views.
	return math3d.Vec3{X: -1.55, Y: 1.02, Z: 2.15}
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
	for _, station := range []struct{ radius, z float64 }{{0.20, 0.055}, {0.42, 0.20}, {0.42, 0.12}} {
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
	mesh = OrientOutward(mesh)
	// OrientOutward is correct for the convex rim and rear cap, but a concave
	// bowl's interior normals intentionally point into its cavity. Reverse the
	// two front-facing face bands so the dish opening (+Z) survives back-face
	// culling from every azimuth instead of showing only the near half.
	for faceIndex := 0; faceIndex < segments*3; faceIndex++ {
		if faceIndex%3 != 2 {
			reverseFaceIndices(mesh.Faces[faceIndex].Vertices)
		}
	}
	// Add restrained structure to the rear cap as well. The original cap was a
	// single opaque polygon, which correctly occluded the bowl but left a blank
	// circle when viewed from behind. These stepped rings/spokes are authored
	// as surfaces with -Z normals. They bulge aft of the cap with a shallow depth
	// pass has a deterministic ordering and back-face culling shows them only
	// from the rear without leaking through the front aperture.
	const (
		rearOuterZ  = 0.115 // rim, just aft of the rear cap
		rearInnerZ  = -0.02 // recessed inner ring
		rearCenterZ = -0.08 // aft-most point, matching the front bowl depth
	)
	rearOuter := len(mesh.Verts)
	for segment := 0; segment < segments; segment++ {
		angle := 2 * math.Pi * float64(segment) / float64(segments)
		sine, cosine := math.Sincos(angle)
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: 0.42 * cosine, Y: 0.42 * sine, Z: rearOuterZ})
	}
	rearInner := len(mesh.Verts)
	for segment := 0; segment < segments; segment++ {
		angle := 2 * math.Pi * float64(segment) / float64(segments)
		sine, cosine := math.Sincos(angle)
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: 0.20 * cosine, Y: 0.20 * sine, Z: rearInnerZ})
	}
	rearCenter := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Z: rearCenterZ})
	for segment := 0; segment < segments; segment++ {
		next := (segment + 1) % segments
		mesh.Edges = append(mesh.Edges,
			Edge{A: rearInner + segment, B: rearInner + next},
			Edge{A: rearOuter + segment, B: rearInner + segment},
			Edge{A: rearCenter, B: rearInner + segment},
		)
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{rearOuter + next, rearOuter + segment, rearInner + segment, rearInner + next}},
			Face{Vertices: []int{rearCenter, rearInner + next, rearInner + segment}},
		)
	}
	mesh.Topology = nil
	emitter := Transform(cylinder(0.07, 0.18, 8), math3d.Translation(0, 0, 0.20))
	return Merge(mesh, emitter)
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
// can be placed on the offset cockpit corridor without introducing a screen-facing
// overlay.
func profiledCylinderZ(profile []falconConeRing, segments int, center math3d.Vec3) Model {
	return profiledCylinderZWithCaps(profile, segments, center, true, true)
}

func profiledCylinderZWithCaps(profile []falconConeRing, segments int, center math3d.Vec3, capStart, capEnd bool) Model {
	return profiledCylinderZWithSeams(profile, segments, center, capStart, capEnd, false)
}

func profiledCylinderZWithSeams(profile []falconConeRing, segments int, center math3d.Vec3, capStart, capEnd, suppressStartSeam bool) Model {
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
			ringKind := EdgeStructural
			if ring == 0 && (suppressStartSeam || !capStart) {
				// This ring is the flush interface with the corridor. Keep the
				// topology but suppress a seam stroke at the shared boundary.
				ringKind = EdgeInternal
			}
			mesh.Edges = append(mesh.Edges,
				Edge{A: base + segment, B: base + next, Kind: ringKind},
				Edge{A: nextBase + segment, B: nextBase + next},
				Edge{A: base + segment, B: nextBase + segment},
			)
			mesh.Faces = append(mesh.Faces, Face{Vertices: []int{base + segment, base + next, nextBase + next, nextBase + segment}})
		}
	}
	if capStart || capEnd {
		front := make([]int, segments)
		rear := make([]int, segments)
		for segment := 0; segment < segments; segment++ {
			front[segments-1-segment] = (len(profile)-1)*segments + segment
			rear[segment] = segment
		}
		if capEnd {
			mesh.Faces = append(mesh.Faces, Face{Vertices: front})
		}
		if capStart {
			mesh.Faces = append(mesh.Faces, Face{Vertices: rear})
		}
	}
	return OrientOutward(mesh)
}
