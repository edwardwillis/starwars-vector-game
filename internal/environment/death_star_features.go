package environment

import (
	"image/color"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/collision"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

const installationTextureScale = .5

// Installation prototypes are authored once in unit coordinates. Generated
// tiles share their prepared topology and use an instance pose/scale; collider
// components use the same coordinates, so visual variation cannot silently
// detach the hit volume from the model.
type installationPrototype struct {
	parts, wreck      []scene.Part
	boxes             []installationBox
	muzzles           []math3d.Vec3
	turretPart        int
	turretPivot       math3d.Vec3
	barrelPart        int
	barrelPivot       math3d.Vec3
	visualTurretPivot math3d.Vec3
	visualBarrelPivot math3d.Vec3
	visualOnly        bool
	detail            scene.DetailTier
	hitPoints         int
	disableAfter      int
}

type installationBox struct {
	center, half math3d.Vec3
}

type installationRing struct {
	y, halfX, halfZ float64
}

var installationPrototypes = map[string]installationPrototype{
	"tower": {
		parts: installationParts("stepped tower", steppedInstallation(
			installationRing{-0.5, 0.46, 0.46}, installationRing{-0.31, 0.46, 0.46},
			installationRing{-0.31, 0.27, 0.27}, installationRing{0.23, 0.27, 0.27},
			installationRing{0.23, 0.38, 0.38}, installationRing{0.43, 0.38, 0.38},
			installationRing{0.43, 0.15, 0.15}, installationRing{0.5, 0.15, 0.15},
		)),
		wreck: installationWreck(),
		boxes: []installationBox{
			{center: math3d.Vec3{Y: -0.405}, half: math3d.Vec3{X: 0.46, Y: 0.095, Z: 0.46}},
			{center: math3d.Vec3{Y: -0.04}, half: math3d.Vec3{X: 0.27, Y: 0.27, Z: 0.27}},
			{center: math3d.Vec3{Y: 0.365}, half: math3d.Vec3{X: 0.38, Y: 0.135, Z: 0.38}},
		},
		detail: scene.DetailMedium, hitPoints: 2,
	},
	"cannon": {
		// The cannon is deliberately a low, three-stage pyramid rather than a
		// stack of boxes: the real surface turbolasers have a broad armoured
		// plinth, a smaller middle course and a final mounting course beneath
		// the traversing head.  Keep the existing part indices and pivots: they
		// are gameplay contracts for aiming, muzzles and collision.
		parts: append(append(installationParts("turbolaser pyramid foundation", turbolaserFoundation()),
			installationParts("twin laser turret", installationOpenBottom(turbolaserHead()))...), installationBarrelParts("twin laser barrels", model.Merge(
			turbolaserHexBarrel(math3d.Vec3{X: -0.035, Y: 0.4225, Z: 0.118}, 0.0115, 0.10),
			turbolaserHexBarrel(math3d.Vec3{X: 0.035, Y: 0.4225, Z: 0.118}, 0.0115, 0.10),
		))...),
		wreck: installationWreck(),
		boxes: []installationBox{
			{center: math3d.Vec3{Y: -0.095}, half: math3d.Vec3{X: 0.42, Y: 0.405, Z: 0.39}},
			{center: math3d.Vec3{X: -0.11, Y: 0.12, Z: 0.405}, half: math3d.Vec3{X: 0.0325, Y: 0.0325, Z: 0.185}},
			{center: math3d.Vec3{X: 0.11, Y: 0.12, Z: 0.405}, half: math3d.Vec3{X: 0.0325, Y: 0.0325, Z: 0.185}},
		},
		// Keep the established combat emitters and pitch pivot stable while the
		// visible barrels are refined; surface encounter balance must not depend
		// on a presentation-only proportion change.
		muzzles:    []math3d.Vec3{{X: -0.19, Y: 0.20, Z: 0.91}, {X: 0.19, Y: 0.20, Z: 0.91}},
		turretPart: 1, turretPivot: math3d.Vec3{Y: -0.02},
		barrelPart: 2, barrelPivot: math3d.Vec3{Y: 0.20, Z: 0.26},
		visualTurretPivot: math3d.Vec3{Y: 0.35},
		visualBarrelPivot: math3d.Vec3{Y: 0.4225, Z: 0.068},
		detail:            scene.DetailMedium, hitPoints: 3, disableAfter: 2,
	},
	"antenna": {
		parts: installationParts("sensor aerial", model.Merge(
			steppedInstallation(
				installationRing{-0.5, 0.43, 0.43}, installationRing{-0.35, 0.43, 0.43},
				installationRing{-0.35, 0.09, 0.09}, installationRing{0.36, 0.045, 0.045},
				installationRing{0.36, 0.12, 0.12}, installationRing{0.5, 0.12, 0.12},
			),
			installationUnitBox(math3d.Vec3{X: -0.1625, Y: 0.19}, math3d.Vec3{X: 0.235, Y: 0.045, Z: 0.045}, math3d.Vec3{X: 1}),
			installationUnitBox(math3d.Vec3{X: 0.1625, Y: 0.19}, math3d.Vec3{X: 0.235, Y: 0.045, Z: 0.045}, math3d.Vec3{X: -1}),
		)),
		wreck: installationWreck(),
		boxes: []installationBox{
			{center: math3d.Vec3{Y: -0.425}, half: math3d.Vec3{X: 0.43, Y: 0.075, Z: 0.43}},
			{center: math3d.Vec3{Y: 0.075}, half: math3d.Vec3{X: 0.11, Y: 0.425, Z: 0.11}},
			{center: math3d.Vec3{Y: 0.19}, half: math3d.Vec3{X: 0.28, Y: 0.0225, Z: 0.0225}},
		},
		detail: scene.DetailMedium, hitPoints: 1,
	},
	"vent": {
		parts: installationParts("armored vent", model.Merge(
			steppedInstallation(
				installationRing{-0.5, 0.49, 0.43}, installationRing{0.12, 0.49, 0.43},
				installationRing{0.12, 0.36, 0.32}, installationRing{0.33, 0.36, 0.32},
			),
			installationUnitBox(math3d.Vec3{X: -0.19, Y: 0.39}, math3d.Vec3{X: 0.07, Y: 0.12, Z: 0.53}, math3d.Vec3{Y: -1}),
			installationUnitBox(math3d.Vec3{Y: 0.39}, math3d.Vec3{X: 0.07, Y: 0.12, Z: 0.53}, math3d.Vec3{Y: -1}),
			installationUnitBox(math3d.Vec3{X: 0.19, Y: 0.39}, math3d.Vec3{X: 0.07, Y: 0.12, Z: 0.53}, math3d.Vec3{Y: -1}),
		)),
		wreck: installationWreck(),
		boxes: []installationBox{
			{center: math3d.Vec3{Y: -0.085}, half: math3d.Vec3{X: 0.49, Y: 0.415, Z: 0.43}},
			{center: math3d.Vec3{Y: 0.39}, half: math3d.Vec3{X: 0.26, Y: 0.06, Z: 0.265}},
		},
		detail: scene.DetailMedium, hitPoints: 2,
	},
	"equipment-bank": {
		// This is genuine low-rise scenery rather than a target or obstacle. It
		// carries a physical silhouette and face shading but deliberately leaves
		// mission routes and combat population unchanged.
		parts:      passiveInstallationParts("low-profile equipment bank", lowProfileEquipmentBank()),
		visualOnly: true,
		detail:     scene.DetailPrimary,
	},
	"vent-cluster": {
		// Repeating low vent housings are intentionally actual geometry, rather
		// than a high-frequency ground texture. They stay legible at a grazing
		// angle without introducing another animated surface-material pattern.
		parts:      passiveInstallationParts("low vent cluster", lowProfileVentCluster()),
		visualOnly: true,
		detail:     scene.DetailPrimary,
	},
	"power-pod": {
		// A compact power distribution pod gives the passive scenery library a
		// shorter, squarer silhouette than the long equipment banks and ducts.
		parts:      passiveInstallationParts("low power distribution pod", lowProfilePowerPod()),
		visualOnly: true,
		detail:     scene.DetailPrimary,
	},
	"service-conduit": {
		// The two raised runs and central junction read as service conduits at
		// low altitude, but remain a single inexpensive static prototype.
		parts:      passiveInstallationParts("service conduit array", lowProfileServiceConduit()),
		visualOnly: true,
		detail:     scene.DetailPrimary,
	},
	"deck-cluster": {
		// A spread of shallow housings fills the interior of an active tile. It
		// is one shared mesh/candidate rather than a collection of independent
		// objects, so dense-looking surface furniture remains inexpensive.
		parts:      passiveInstallationParts("low-rise deck equipment cluster", lowProfileDeckCluster()),
		visualOnly: true,
		detail:     scene.DetailPrimary,
	},
	"quarter-hangar": {
		// Four curved roof sections form a compact quarter-circle hangar. Its
		// separately shaded recessed inner bay makes the curved opening legible
		// at surface-flight altitude rather than reading as another dark block.
		parts:      quarterCircleHangarParts(),
		visualOnly: true,
		detail:     scene.DetailPrimary,
	},
}

// Horizon furniture deliberately uses simpler silhouettes than the nearby
// passive library. It is baked into a horizon tile as one visual-only part, so
// it contributes real depth and material variation without becoming a set of
// addressable simulation features or emitting a distant vector-line forest.
var horizonSceneryPrototypes = map[string]model.Model{
	"bank": horizonEquipmentBank(),
	"vent": horizonVentBank(),
	"pod":  horizonPowerPod(),
	// The horizon version retains the same four broad roof sections, but its
	// edges are suppressed when baked into a tile so it reads as silhouette and
	// fixed material shading rather than a distant vector-line cluster.
	"hangar": quarterCircleHangar(),
}

func installationParts(name string, mesh model.Model) []scene.Part {
	return []scene.Part{{
		Name: name, Mesh: mesh, Color: deathStarInstallationLine, LineWidth: deathStarInstallationLineWidth,
		Surface:       deathStarInstallationSurface,
		SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll,
	}}
}

// passiveInstallationParts preserves the active-installation material for
// combat geometry while giving visual-only low-rise furniture fixed per-face
// lighting. The shared mesh remains one render candidate and one GPU texture
// batch; only its submitted face tints differ.
func passiveInstallationParts(name string, mesh model.Model) []scene.Part {
	parts := installationParts(name, mesh)
	parts[0].Surface = deathStarPassiveInstallationSurface
	return parts
}

func quarterCircleHangarParts() []scene.Part {
	shell, bay := quarterCircleHangarMeshes()
	return []scene.Part{
		{
			Name: "quarter-circle hangar shell", Mesh: shell,
			Color: deathStarInstallationLine, LineWidth: deathStarInstallationLineWidth,
			Surface:       deathStarPassiveInstallationSurface,
			SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll,
		},
		{
			Name: "quarter-circle hangar recessed bay", Mesh: bay,
			Color: deathStarHangarBayLine, LineWidth: deathStarInstallationLineWidth,
			Surface:       deathStarHangarBaySurface,
			SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll,
		},
	}
}

// installationBarrelParts remain comparatively plain and outlined so active
// turrets are still legible at combat range. Their geometry and articulation
// are unchanged; their flat material now shares the fixture light source.
func installationBarrelParts(name string, mesh model.Model) []scene.Part {
	return []scene.Part{{
		Name: name, Mesh: mesh, Color: deathStarInstallationBarrelLine, LineWidth: .8,
		Surface:       deathStarInstallationBarrelSurface,
		SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll,
	}}
}

func installationWreck() []scene.Part {
	mesh := model.Prepare(model.Model{
		Verts: []math3d.Vec3{
			{X: -0.44, Y: -0.49, Z: -0.44}, {X: 0.44, Y: -0.49, Z: -0.44},
			{X: 0.44, Y: -0.49, Z: 0.44}, {X: -0.44, Y: -0.49, Z: 0.44},
			{X: -0.20, Y: -0.485, Z: -0.18}, {X: 0.23, Y: -0.485, Z: 0.16},
		},
		Edges: []model.Edge{{A: 0, B: 1}, {A: 1, B: 2}, {A: 2, B: 3}, {A: 3, B: 0}, {A: 4, B: 5}},
	})
	return []scene.Part{{Name: "scorched foundation", Mesh: mesh, Color: color.RGBA{R: 255, G: 116, B: 45, A: 255}, LineWidth: 1.25}}
}

func installationOpenBottom(mesh model.Model) model.Model {
	faces := make([]model.Face, 0, len(mesh.Faces)-1)
	for _, face := range mesh.Faces {
		if face.Normal.Y > -0.99 {
			faces = append(faces, face)
		}
	}
	mesh.Faces = faces
	mesh.Topology = nil
	return model.Prepare(mesh)
}

func installationUnitBox(center, size, joiningFace math3d.Vec3) model.Model {
	// The touching cap is inside the composite installation. Leaving it out
	// avoids coincident opaque faces and shimmering at the body/appendage seam.
	unit := model.Cube(1)
	faces := make([]model.Face, 0, len(unit.Faces)-1)
	for _, face := range unit.Faces {
		if face.Normal.Dot(joiningFace) < 0.99 {
			faces = append(faces, face)
		}
	}
	unit.Faces = faces
	for index, edge := range unit.Edges {
		if unit.Verts[edge.A].Dot(joiningFace) > 0.49 && unit.Verts[edge.B].Dot(joiningFace) > 0.49 {
			unit.Edges[index].Kind = model.EdgeInternal
		}
	}
	unit.Topology = nil
	return texturedInstallation(model.Transform(unit, math3d.Translation(center.X, center.Y, center.Z).Mul(math3d.Scaling(size.X, size.Y, size.Z))))
}

// turbolaserHexBarrel is a short six-sided prism running along +Z. Its rear
// cap is omitted where it enters the cube turret, avoiding a hidden
// coincident surface while giving the visible barrel a less block-like form.
func turbolaserHexBarrel(center math3d.Vec3, radius, length float64) model.Model {
	const sides = 6
	mesh := model.Model{}
	rearZ, frontZ := center.Z-length/2, center.Z+length/2
	for index := 0; index < sides; index++ {
		angle := 2 * math.Pi * float64(index) / sides
		x, y := center.X+math.Cos(angle)*radius, center.Y+math.Sin(angle)*radius
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: x, Y: y, Z: rearZ})
	}
	for index := 0; index < sides; index++ {
		angle := 2 * math.Pi * float64(index) / sides
		x, y := center.X+math.Cos(angle)*radius, center.Y+math.Sin(angle)*radius
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: x, Y: y, Z: frontZ})
	}
	for index := 0; index < sides; index++ {
		next := (index + 1) % sides
		mesh.Edges = append(mesh.Edges,
			model.Edge{A: index, B: next},
			model.Edge{A: sides + index, B: sides + next},
			model.Edge{A: index, B: sides + index},
		)
		mesh.Faces = append(mesh.Faces, model.Face{Vertices: []int{index, next, sides + next, sides + index}})
	}
	mesh.Faces = append(mesh.Faces, model.Face{Vertices: []int{sides, sides + 1, sides + 2, sides + 3, sides + 4, sides + 5}})
	return model.Prepare(mesh)
}

// lowProfileEquipmentBank is a small armoured plinth with three equipment
// housings. Its height is intentionally modest relative to its footprint: it
// supplies the dense, layered silhouette of Death Star surface furniture
// without competing with towers or turbolaser emplacements.
func lowProfileEquipmentBank() model.Model {
	return model.Merge(
		steppedInstallation(
			installationRing{y: -0.50, halfX: 0.50, halfZ: 0.40},
			installationRing{y: -0.26, halfX: 0.50, halfZ: 0.40},
			installationRing{y: -0.26, halfX: 0.42, halfZ: 0.32},
			installationRing{y: 0.16, halfX: 0.42, halfZ: 0.32},
			installationRing{y: 0.16, halfX: 0.34, halfZ: 0.25},
			installationRing{y: 0.22, halfX: 0.34, halfZ: 0.25},
		),
		installationUnitBox(math3d.Vec3{X: -0.24, Y: 0.29}, math3d.Vec3{X: 0.13, Y: 0.14, Z: 0.42}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{Y: 0.29}, math3d.Vec3{X: 0.13, Y: 0.14, Z: 0.42}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{X: 0.24, Y: 0.29}, math3d.Vec3{X: 0.13, Y: 0.14, Z: 0.42}, math3d.Vec3{Y: -1}),
	)
}

// lowProfileVentCluster is a broad, shallow plinth with four raised vent
// ribs.  It deliberately has a different silhouette from a tower or cannon:
// its value is the low horizon breakup of passive surface furniture.
func lowProfileVentCluster() model.Model {
	return model.Merge(
		steppedInstallation(
			installationRing{y: -0.50, halfX: 0.50, halfZ: 0.42},
			installationRing{y: -0.28, halfX: 0.50, halfZ: 0.42},
			installationRing{y: -0.28, halfX: 0.44, halfZ: 0.36},
			installationRing{y: 0.08, halfX: 0.44, halfZ: 0.36},
		),
		installationUnitBox(math3d.Vec3{X: -0.27, Y: 0.16}, math3d.Vec3{X: 0.055, Y: 0.16, Z: 0.40}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{X: -0.09, Y: 0.16}, math3d.Vec3{X: 0.055, Y: 0.16, Z: 0.40}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{X: 0.09, Y: 0.16}, math3d.Vec3{X: 0.055, Y: 0.16, Z: 0.40}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{X: 0.27, Y: 0.16}, math3d.Vec3{X: 0.055, Y: 0.16, Z: 0.40}, math3d.Vec3{Y: -1}),
	)
}

// lowProfilePowerPod is a compact raised cabinet with two side units. It is
// intentionally simple: the shared installation material supplies the small
// panel seams, while the 3D form supplies the important readable scale.
func lowProfilePowerPod() model.Model {
	return model.Merge(
		steppedInstallation(
			installationRing{y: -0.50, halfX: 0.45, halfZ: 0.45},
			installationRing{y: -0.30, halfX: 0.45, halfZ: 0.45},
			installationRing{y: -0.30, halfX: 0.34, halfZ: 0.34},
			installationRing{y: 0.02, halfX: 0.34, halfZ: 0.34},
		),
		installationUnitBox(math3d.Vec3{Y: 0.16}, math3d.Vec3{X: 0.28, Y: 0.28, Z: 0.28}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{X: -0.33, Y: 0.11}, math3d.Vec3{X: 0.10, Y: 0.18, Z: 0.18}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{X: 0.33, Y: 0.11}, math3d.Vec3{X: 0.10, Y: 0.18, Z: 0.18}, math3d.Vec3{Y: -1}),
	)
}

// lowProfileServiceConduit has paired raised service runs meeting at a small
// junction housing. Keeping their bases open avoids coplanar faces where the
// runs enter the plinth, which matters most when the deck is viewed close up.
func lowProfileServiceConduit() model.Model {
	return model.Merge(
		steppedInstallation(
			installationRing{y: -0.50, halfX: 0.50, halfZ: 0.36},
			installationRing{y: -0.34, halfX: 0.50, halfZ: 0.36},
			installationRing{y: -0.34, halfX: 0.44, halfZ: 0.30},
			installationRing{y: -0.05, halfX: 0.44, halfZ: 0.30},
		),
		installationUnitBox(math3d.Vec3{X: -0.18, Y: 0.07}, math3d.Vec3{X: 0.055, Y: 0.24, Z: 0.34}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{X: 0.18, Y: 0.07}, math3d.Vec3{X: 0.055, Y: 0.24, Z: 0.34}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{Y: 0.12}, math3d.Vec3{X: 0.15, Y: 0.10, Z: 0.10}, math3d.Vec3{Y: -1}),
	)
}

// lowProfileDeckCluster distributes eight short, varied housings across one
// unit footprint. After the tile instance scales it to the central deck area,
// the spaces between the forms read as service lanes rather than a single
// block. All bases are open and sit exactly on the deck, avoiding coplanar
// underside faces while keeping the entire cluster to 40 visible faces.
func lowProfileDeckCluster() model.Model {
	return model.Merge(
		lowDeckHousing(-0.30, -0.28, 0.20, 0.52, 0.18),
		lowDeckHousing(0.02, -0.30, 0.32, 0.34, 0.14),
		lowDeckHousing(0.30, -0.26, 0.16, 0.70, 0.20),
		lowDeckHousing(-0.24, 0.15, 0.28, 0.40, 0.20),
		lowDeckHousing(0.10, 0.16, 0.18, 0.56, 0.30),
		lowDeckHousing(0.34, 0.24, 0.12, 0.36, 0.16),
		lowDeckHousing(-0.42, -0.02, 0.10, 0.44, 0.16),
		lowDeckHousing(0.42, 0.02, 0.09, 0.48, 0.14),
	)
}

func lowDeckHousing(x, z, width, height, depth float64) model.Model {
	return installationUnitBox(
		math3d.Vec3{X: x, Y: -0.5 + height/2, Z: z},
		math3d.Vec3{X: width, Y: height, Z: depth},
		math3d.Vec3{Y: -1},
	)
}

// quarterCircleHangar merges the foreground shell and recessed-bay artwork for
// the horizon-only tile path, which deliberately uses one shared material.
func quarterCircleHangar() model.Model {
	shell, bay := quarterCircleHangarMeshes()
	return model.Merge(shell, bay)
}

// quarterCircleHangarMeshes forms an ordinary long, low service building bent
// through a quarter turn. It is deliberately a narrow annular strip rather
// than a compact radial bay: the plan should read like a familiar oblong
// surface installation whose straight run has been curved around a corner.
// The shell has a curved outer fascia and two terminal bulkheads. A separately
// shaded inner lining provides a genuinely recessed opening, so the structure
// remains recognizable at a grazing flight angle.
func quarterCircleHangarMeshes() (model.Model, model.Model) {
	const (
		sections    = 4
		// These radii make the centre-line run over five times as long as the
		// building is wide. The earlier, much fatter annulus read as a small
		// radial installation instead of an oblong structure bent in plan.
		innerRadius = 0.70
		outerRadius = 0.94
		baseY       = -0.50
		bayFloorY   = -0.34
		// Keep the cross-section shallow, matching the low roofline of the
		// ordinary equipment banks while retaining a subtle fall to the outer
		// fascia that catches the fixed surface lighting.
		innerRoofY           = 0.20
		outerRoofY           = 0.08
		quarterCircleRadians = math.Pi / 2
	)
	point := func(radius, angle, y float64) math3d.Vec3 {
		return math3d.Vec3{X: math.Cos(angle) * radius, Y: y, Z: math.Sin(angle) * radius}
	}
	shell, bay := model.Model{}, model.Model{}
	for section := 0; section < sections; section++ {
		start := quarterCircleRadians * float64(section) / sections
		end := quarterCircleRadians * float64(section+1) / sections
		base := len(shell.Verts)
		// Top inner/outer corners followed by the two base corners of the
		// outer wall. The inner lining is emitted separately as the dark bay.
		shell.Verts = append(shell.Verts,
			point(innerRadius, start, innerRoofY),
			point(innerRadius, end, innerRoofY),
			point(outerRadius, end, outerRoofY),
			point(outerRadius, start, outerRoofY),
			point(outerRadius, start, baseY),
			point(outerRadius, end, baseY),
		)
		shell.Faces = append(shell.Faces,
			model.Face{Vertices: []int{base, base + 1, base + 2, base + 3}},     // roof section
			model.Face{Vertices: []int{base + 3, base + 2, base + 5, base + 4}}, // outer shell
		)
		shell.Edges = append(shell.Edges,
			model.Edge{A: base, B: base + 1},
			model.Edge{A: base + 1, B: base + 2},
			model.Edge{A: base + 2, B: base + 3},
			model.Edge{A: base + 3, B: base},
			model.Edge{A: base + 3, B: base + 4},
			model.Edge{A: base + 2, B: base + 5},
			model.Edge{A: base + 4, B: base + 5},
		)

		bayBase := len(bay.Verts)
		bay.Verts = append(bay.Verts,
			point(innerRadius, start, innerRoofY),
			point(innerRadius, end, innerRoofY),
			point(innerRadius, end, bayFloorY),
			point(innerRadius, start, bayFloorY),
		)
		bay.Faces = append(bay.Faces, model.Face{Vertices: []int{bayBase, bayBase + 1, bayBase + 2, bayBase + 3}, DoubleSided: true})
		bay.Edges = append(bay.Edges,
			model.Edge{A: bayBase, B: bayBase + 1},
			model.Edge{A: bayBase + 1, B: bayBase + 2},
			model.Edge{A: bayBase + 2, B: bayBase + 3},
			model.Edge{A: bayBase + 3, B: bayBase},
		)

		if section == 0 {
			base := len(shell.Verts)
			shell.Verts = append(shell.Verts,
				point(innerRadius, start, innerRoofY), point(outerRadius, start, outerRoofY),
				point(outerRadius, start, baseY), point(innerRadius, start, bayFloorY),
			)
			shell.Faces = append(shell.Faces, model.Face{Vertices: []int{base, base + 1, base + 2, base + 3}})
		}
		if section == sections-1 {
			base := len(shell.Verts)
			shell.Verts = append(shell.Verts,
				point(innerRadius, end, innerRoofY), point(innerRadius, end, bayFloorY),
				point(outerRadius, end, baseY), point(outerRadius, end, outerRoofY),
			)
			shell.Faces = append(shell.Faces, model.Face{Vertices: []int{base, base + 1, base + 2, base + 3}})
		}
	}
	return texturedInstallation(model.OrientOutward(shell)), model.Prepare(bay)
}

// horizonEquipmentBank, horizonVentBank and horizonPowerPod are all authored
// in a -0.5..0.5 local vertical span so the same placement rule can rest them
// precisely on a deck. They are intentionally much simpler than close-range
// installations: the horizon needs broad stepped silhouettes, not tiny panel
// detail that would shimmer under a grazing camera angle.
func horizonEquipmentBank() model.Model {
	return model.Merge(
		installationUnitBox(math3d.Vec3{Y: -0.19}, math3d.Vec3{X: 1, Y: 0.62, Z: 0.78}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{Y: 0.30}, math3d.Vec3{X: 0.62, Y: 0.36, Z: 0.48}, math3d.Vec3{Y: -1}),
	)
}

func horizonVentBank() model.Model {
	return model.Merge(
		installationUnitBox(math3d.Vec3{Y: -0.20}, math3d.Vec3{X: 1, Y: 0.60, Z: 0.76}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{X: -0.24, Y: 0.29}, math3d.Vec3{X: 0.12, Y: 0.38, Z: 0.64}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{X: 0.24, Y: 0.29}, math3d.Vec3{X: 0.12, Y: 0.38, Z: 0.64}, math3d.Vec3{Y: -1}),
	)
}

func horizonPowerPod() model.Model {
	return model.Merge(
		installationUnitBox(math3d.Vec3{Y: -0.21}, math3d.Vec3{X: 0.78, Y: 0.58, Z: 0.78}, math3d.Vec3{Y: -1}),
		installationUnitBox(math3d.Vec3{Y: 0.24}, math3d.Vec3{X: 0.42, Y: 0.32, Z: 0.42}, math3d.Vec3{Y: -1}),
	)
}

type horizonSceneryPlacement struct {
	kind     string
	x, z     float64
	scale    math3d.Vec3
	fixedYaw bool
	yaw      float64
}

const horizonScenerySeedOffset = 16

// horizonSurfaceFurniturePart bakes a dense field of simplified low-rise
// groups into a horizon tile. The resulting geometry is constructed only when
// that tile is streamed, then reused across frames. Marking every edge
// internal retains the material/silhouette contribution while suppressing tiny
// horizon vector strokes that would otherwise create visual noise and CPU line
// work.
func horizonSurfaceFurniturePart(coordinate TileCoordinate, xCenter, zCenter float64, trench bool) scene.Part {
	placements := []horizonSceneryPlacement{
		{kind: "bank", x: -24, z: -24, scale: math3d.Vec3{X: 14, Y: 2.0, Z: 9}},
		{kind: "vent", x: 24, z: -23, scale: math3d.Vec3{X: 11, Y: 1.8, Z: 9}},
		{kind: "pod", x: -23, z: 3, scale: math3d.Vec3{X: 9, Y: 2.2, Z: 9}},
		{kind: "bank", x: 24, z: 23, scale: math3d.Vec3{X: 13, Y: 1.8, Z: 8}},
		{kind: "vent", x: -2, z: -24, scale: math3d.Vec3{X: 10, Y: 1.7, Z: 8}},
		{kind: "pod", x: 1, z: 3, scale: math3d.Vec3{X: 10, Y: 1.8, Z: 10}},
		// Match the close-range placement: reserve the rear-port corner for the
		// broad, distinctive quarter hangar rather than burying it among the
		// central horizon props.
		{kind: "hangar", x: -25, z: 26, scale: math3d.Vec3{X: 22, Y: 3.6, Z: 22}, fixedYaw: true, yaw: 0},
	}
	if trench {
		// Preserve the central 28-unit-wide trench opening even for tiles that
		// are only represented by horizon artwork. Every placement remains on
		// one side deck, so neither the floor nor the walls acquire furniture.
		placements = []horizonSceneryPlacement{
			{kind: "bank", x: 26, z: -24, scale: math3d.Vec3{X: 14, Y: 2.0, Z: 9}},
			{kind: "vent", x: -26, z: -23, scale: math3d.Vec3{X: 11, Y: 1.8, Z: 9}},
			{kind: "pod", x: 26, z: -3, scale: math3d.Vec3{X: 9, Y: 2.0, Z: 9}},
			{kind: "bank", x: -26, z: 2, scale: math3d.Vec3{X: 13, Y: 1.8, Z: 8}},
			{kind: "vent", x: 26, z: 24, scale: math3d.Vec3{X: 10, Y: 1.7, Z: 8}},
			{kind: "pod", x: -26, z: 25, scale: math3d.Vec3{X: 9, Y: 2.0, Z: 9}},
		}
	}
	meshes := make([]model.Model, 0, len(placements))
	for index, placement := range placements {
		seed := installationSeed(coordinate, horizonScenerySeedOffset+index)
		jitterX := (float64((seed>>12)&3) - 1.5) * 0.75
		jitterZ := (float64((seed>>16)&3) - 1.5) * 0.75
		scale := placement.scale
		scale.X += float64((seed>>4)&1) * 0.8
		scale.Y += float64((seed>>6)&1) * 0.2
		scale.Z += float64((seed>>8)&1) * 0.6
		yaw := installationYaw(seed)
		if placement.fixedYaw {
			yaw = placement.yaw
		}
		transform := math3d.Translation(xCenter+placement.x+jitterX, scale.Y/2, zCenter+placement.z+jitterZ).
			Mul(math3d.QuaternionFromAxisAngle(math3d.Vec3{Y: 1}, yaw).Matrix()).
			Mul(math3d.Scaling(scale.X, scale.Y, scale.Z))
		meshes = append(meshes, model.Transform(horizonSceneryPrototypes[placement.kind], transform))
	}
	mesh := model.Merge(meshes...)
	for index := range mesh.Edges {
		mesh.Edges[index].Kind = model.EdgeInternal
	}
	mesh.Topology = nil
	mesh = model.Prepare(mesh)
	return scene.Part{
		Name: "distant low-rise surface furniture", Mesh: mesh,
		Color: deathStarInstallationLine, LineWidth: deathStarInstallationLineWidth,
		Surface: deathStarPassiveInstallationSurface,
	}
}

// steppedInstallation connects rectangular rings without hidden coincident
// caps. Equal-height rings form real horizontal ledges, not overlapping boxes.
func steppedInstallation(rings ...installationRing) model.Model {
	return texturedInstallation(steppedInstallationMesh(rings...))
}

// steppedInstallationMesh is the untextured authored form used when a
// specialised installation needs additional vector detail before the shared
// material UVs and immutable topology are prepared.
func steppedInstallationMesh(rings ...installationRing) model.Model {
	mesh := model.Model{}
	for _, ring := range rings {
		mesh.Verts = append(mesh.Verts,
			math3d.Vec3{X: -ring.halfX, Y: ring.y, Z: -ring.halfZ},
			math3d.Vec3{X: ring.halfX, Y: ring.y, Z: -ring.halfZ},
			math3d.Vec3{X: ring.halfX, Y: ring.y, Z: ring.halfZ},
			math3d.Vec3{X: -ring.halfX, Y: ring.y, Z: ring.halfZ},
		)
	}
	for ring := range rings {
		base := ring * 4
		for side := 0; side < 4; side++ {
			next := (side + 1) % 4
			mesh.Edges = append(mesh.Edges, model.Edge{A: base + side, B: base + next})
			if ring+1 == len(rings) {
				continue
			}
			upper := base + 4
			mesh.Edges = append(mesh.Edges, model.Edge{A: base + side, B: upper + side})
			mesh.Faces = append(mesh.Faces, model.Face{Vertices: []int{base + side, upper + side, upper + next, base + next}})
		}
	}
	last := (len(rings) - 1) * 4
	mesh.Faces = append(mesh.Faces, model.Face{Vertices: []int{0, 1, 2, 3}}, model.Face{Vertices: []int{last + 3, last + 2, last + 1, last}})
	return mesh
}

// turbolaserFoundation is the static, three-course tapered pedestal used by
// each surface gun.  The tiny ledges separate the courses without coincident
// faces; the vertical face ribs give the broad slopes the characteristic ANH
// surface-panel construction rather than a Lego-like block silhouette.
func turbolaserFoundation() model.Model {
	rings := []installationRing{
		// These rings trace the supplied side profile: broad first frustum,
		// smaller second frustum, then a tall narrow upper mounting block.
		{y: -0.50, halfX: 0.42, halfZ: 0.39}, // section 1: 0.24 high
		{y: -0.26, halfX: 0.225, halfZ: 0.209},
		{y: -0.255, halfX: 0.22, halfZ: 0.204}, // ledge 1 -> 2
		{y: 0.025, halfX: 0.115, halfZ: 0.107}, // section 2: 0.28 high
		{y: 0.030, halfX: 0.11, halfZ: 0.102},  // ledge 2 -> 3
		{y: 0.350, halfX: 0.10, halfZ: 0.092},  // section 3: nearly vertical
	}
	mesh := steppedInstallationMesh(rings...)
	for _, span := range [][2]int{{0, 1}, {2, 3}, {4, 5}} {
		appendFrustumFaceRibs(&mesh, rings, span[0], span[1], 4)
	}
	return texturedInstallation(mesh)
}

// turbolaserHead is the compact gunhouse mounted on the foundation's flat top.
// Its slightly smaller roof gives all four side panels a restrained inward
// slope, while retaining the squat WW2 battleship-turret silhouette.
func turbolaserHead() model.Model {
	return installationOpenBottom(texturedInstallation(steppedInstallationMesh(
		installationRing{y: 0.350, halfX: 0.0725, halfZ: 0.0725},
		installationRing{y: 0.495, halfX: 0.065, halfZ: 0.065},
	)))
}

// appendFrustumFaceRibs adds inset-free vector construction lines to one
// tapered square course.  They are offset slightly from the opaque face to
// avoid depth fighting, and are decorative so they never alter occlusion.
func appendFrustumFaceRibs(mesh *model.Model, rings []installationRing, lower, upper, count int) {
	if lower < 0 || upper >= len(rings) || lower >= upper || count <= 0 {
		return
	}
	for side := 0; side < 4; side++ {
		next := (side + 1) % 4
		lowerA := mesh.Verts[lower*4+side]
		lowerB := mesh.Verts[lower*4+next]
		upperA := mesh.Verts[upper*4+side]
		upperB := mesh.Verts[upper*4+next]

		centre := lowerA.Add(lowerB).Add(upperA).Add(upperB).Scale(0.25)
		normal := lowerB.Sub(lowerA).Cross(upperA.Sub(lowerA)).Normalize()
		if normal.Dot(centre) < 0 {
			normal = normal.Scale(-1)
		}
		offset := normal.Scale(0.0025)
		for rib := 1; rib <= count; rib++ {
			t := float64(rib) / float64(count+1)
			lowerPoint := lowerA.Add(lowerB.Sub(lowerA).Scale(t)).Add(offset)
			upperPoint := upperA.Add(upperB.Sub(upperA).Scale(t)).Add(offset)
			start := len(mesh.Verts)
			mesh.Verts = append(mesh.Verts, lowerPoint, upperPoint)
			mesh.Edges = append(mesh.Edges, model.Edge{A: start, B: start + 1, Kind: model.EdgeDecorative, Importance: 0.55})
		}
	}
}

// texturedInstallation supplies stable planar UVs for the existing authored
// box/ring faces. The model remains in prototype-local coordinates, so every
// streamed instance retains the same coherent panel layout through its pose.
func texturedInstallation(mesh model.Model) model.Model {
	mesh = model.Prepare(mesh)
	for faceIndex := range mesh.Faces {
		face := &mesh.Faces[faceIndex]
		face.UVs = make([]model.UV, len(face.Vertices))
		normal := face.Normal
		for index, vertexIndex := range face.Vertices {
			vertex := mesh.Verts[vertexIndex]
			switch {
			case math.Abs(normal.Y) >= math.Abs(normal.X) && math.Abs(normal.Y) >= math.Abs(normal.Z):
				face.UVs[index] = model.UV{U: vertex.X / installationTextureScale, V: vertex.Z / installationTextureScale}
			case math.Abs(normal.X) >= math.Abs(normal.Z):
				face.UVs[index] = model.UV{U: vertex.Z / installationTextureScale, V: vertex.Y / installationTextureScale}
			default:
				face.UVs[index] = model.UV{U: vertex.X / installationTextureScale, V: vertex.Y / installationTextureScale}
			}
		}
	}
	mesh.Topology = nil
	return model.Prepare(mesh)
}

func newDeathStarInstallation(id, kind string, position, scale math3d.Vec3, yaw float64) (Feature, []collision.OrientedBox) {
	prototype := installationPrototypes[kind]
	orientation := math3d.QuaternionFromAxisAngle(math3d.Vec3{Y: 1}, yaw)
	boxes := make([]collision.OrientedBox, 0, len(prototype.boxes))
	for _, component := range prototype.boxes {
		offset := math3d.Vec3{X: component.center.X * scale.X, Y: component.center.Y * scale.Y, Z: component.center.Z * scale.Z}
		boxes = append(boxes, collision.OrientedBox{
			Center: position.Add(orientation.Rotate(offset)), Orientation: orientation,
			HalfExtents: math3d.Vec3{X: component.half.X * scale.X, Y: component.half.Y * scale.Y, Z: component.half.Z * scale.Z},
			FeatureID:   collision.FeatureID(id),
		})
	}
	return Feature{
		ID: id, Kind: kind, Team: scene.TeamEmpire,
		Pose: kinematics.Pose{Position: position, Orientation: orientation}, Scale: scale,
		Parts: append([]scene.Part(nil), prototype.parts...), WreckParts: append([]scene.Part(nil), prototype.wreck...),
		Muzzles:    prototype.muzzles,
		TurretPart: prototype.turretPart, TurretPivot: prototype.turretPivot,
		BarrelPart: prototype.barrelPart, BarrelPivot: prototype.barrelPivot,
		VisualTurretPivot: prototype.visualTurretPivot, VisualBarrelPivot: prototype.visualBarrelPivot,
		Boxes: boxes, Detail: prototype.detail, Targetable: !prototype.visualOnly, Hittable: !prototype.visualOnly,
		HitPoints: prototype.hitPoints, DisableAfter: prototype.disableAfter,
	}, boxes
}

// TurbolaserInspection returns the exact intact cannon artwork used by Death
// Star surface tiles as a non-physical scene object. It exists solely for the
// catalogue/showcase: no alternate display mesh is maintained, so visual
// proportion refinements can be examined before they appear in surface flight.
func TurbolaserInspection(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	prototype := installationPrototypes["cannon"]
	return scene.Object{
		ID: id, Name: "Death Star turbolaser emplacement", Team: scene.TeamEmpire, Pose: pose,
		Parts:   append([]scene.Part(nil), prototype.parts...),
		Anchors: map[string]kinematics.Pose{"center": {Orientation: math3d.IdentityQuaternion()}},
		// This is presentation-only, so it intentionally has no collision or
		// combat state. Its radius only keeps normal showcase culling reliable.
		VisualRadius: 1,
	}
}

// QuarterCircleHangarInspection exposes the exact production passive-hangar
// artwork in the carousel. It has no collision, target, or simulation state;
// the object exists only to make the low-angle and top-down silhouette easy to
// inspect before judging it among dense surface furniture.
func QuarterCircleHangarInspection(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	prototype := installationPrototypes["quarter-hangar"]
	return scene.Object{
		ID: id, Name: "Death Star quarter-circle service hangar", Team: scene.TeamEmpire, Pose: pose,
		Parts:        append([]scene.Part(nil), prototype.parts...),
		Anchors:      map[string]kinematics.Pose{"center": {Orientation: math3d.IdentityQuaternion()}},
		// The carousel enlarges this mesh twelvefold. Keep its view bound in
		// step with the broad quarter-turn so it cannot disappear while sliding
		// between adjacent showcase entries.
		VisualRadius: 16,
	}
}

func installationSeed(coordinate TileCoordinate, index int) uint64 {
	seed := uint64(int64(coordinate.X))*0x9e3779b97f4a7c15 ^ uint64(int64(coordinate.Z))*0xbf58476d1ce4e5b9 ^ uint64(index+1)*0x94d049bb133111eb
	seed ^= seed >> 30
	seed *= 0xbf58476d1ce4e5b9
	seed ^= seed >> 27
	seed *= 0x94d049bb133111eb
	return seed ^ (seed >> 31)
}

func installationYaw(seed uint64) float64 {
	return float64((seed>>8)&3) * math.Pi / 2
}
