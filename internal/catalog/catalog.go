// Package catalog assembles reusable wireframe models into styled scene objects.
package catalog

import (
	"fmt"
	"image/color"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

var (
	vectorRed   = color.RGBA{R: 255, G: 48, B: 32, A: 255}
	vectorGreen = color.RGBA{R: 64, G: 255, B: 96, A: 255}
	windowAmber = color.RGBA{R: 255, G: 192, B: 48, A: 255}

	tieFighterGeometry            = model.TIEFighterGeometryData()
	tieFighterCore                = model.Transform(tieFighterGeometry.Core, math3d.Scaling(tieFighterScale, tieFighterScale, tieFighterScale))
	tieFighterLeftFoil            = model.Transform(tieFighterGeometry.LeftFoil, math3d.Scaling(tieFighterScale, tieFighterScale, tieFighterScale))
	tieFighterRightFoil           = model.Transform(tieFighterGeometry.RightFoil, math3d.Scaling(tieFighterScale, tieFighterScale, tieFighterScale))
	tieFighterWindow              = model.Transform(tieFighterGeometry.Window, math3d.Scaling(tieFighterScale, tieFighterScale, tieFighterScale))
	tieFighterDebris              = transformTIEFighterFragments(tieFighterGeometry.Fragments, math3d.Scaling(tieFighterScale, tieFighterScale, tieFighterScale))
	xWingGeometry                 = model.XWingGeometryData()
	xWingFoilAssemblies           = xWingGeometry.Foils
	xWingWindow                   = xWingGeometry.Window
	xWingDebris                   = xWingGeometry.Fragments
	laserBoltRays                 = model.LaserBoltRays()
	laserBoltTips                 = model.LaserBoltBranches()
	tieFighterPolygonShards       = buildTIEFighterPolygonShards()
	tieInterceptorGeometry        = model.TIEInterceptorGeometryData()
	tieInterceptorCockpit         = model.Transform(tieInterceptorGeometry.Cockpit, math3d.Scaling(tieInterceptorScale, tieInterceptorScale, tieInterceptorScale))
	tieInterceptorPylons          = model.Transform(tieInterceptorGeometry.Pylons, math3d.Scaling(tieInterceptorScale, tieInterceptorScale, tieInterceptorScale))
	tieInterceptorWindow          = model.Transform(tieInterceptorGeometry.Window, math3d.Scaling(tieInterceptorScale, tieInterceptorScale, tieInterceptorScale))
	tieInterceptorPanels          = transformTIEInterceptorParts(tieInterceptorGeometry.Panels, math3d.Scaling(tieInterceptorScale, tieInterceptorScale, tieInterceptorScale))
	tieInterceptorEndPanels       = transformTIEInterceptorEndPanels(tieInterceptorGeometry.EndPanels, math3d.Scaling(tieInterceptorScale, tieInterceptorScale, tieInterceptorScale))
	tieInterceptorCannons         = transformTIEInterceptorParts(tieInterceptorGeometry.Cannons, math3d.Scaling(tieInterceptorScale, tieInterceptorScale, tieInterceptorScale))
	tieInterceptorDebris          = transformTIEInterceptorFragments(tieInterceptorGeometry.Fragments, math3d.Scaling(tieInterceptorScale, tieInterceptorScale, tieInterceptorScale))
	tieInterceptorPolygonShards   = buildTIEInterceptorPolygonShards()
	millenniumFalconStations      = model.MillenniumFalconStationsData()
	millenniumFalconGeometry      = model.MillenniumFalconGeometryData()
	millenniumFalconHullCore      = model.Transform(millenniumFalconGeometry.HullCore, math3d.Scaling(millenniumFalconScale, millenniumFalconScale, millenniumFalconScale))
	millenniumFalconLeftMandible  = model.Transform(millenniumFalconGeometry.LeftMandible, math3d.Scaling(millenniumFalconScale, millenniumFalconScale, millenniumFalconScale))
	millenniumFalconRightMandible = model.Transform(millenniumFalconGeometry.RightMandible, math3d.Scaling(millenniumFalconScale, millenniumFalconScale, millenniumFalconScale))
	millenniumFalconCargoRamp     = model.Transform(millenniumFalconGeometry.CargoRamp, math3d.Scaling(millenniumFalconScale, millenniumFalconScale, millenniumFalconScale))
	millenniumFalconCorridor      = model.Transform(millenniumFalconGeometry.Corridor, math3d.Scaling(millenniumFalconScale, millenniumFalconScale, millenniumFalconScale))
	millenniumFalconWindow        = model.Transform(millenniumFalconGeometry.Window, math3d.Scaling(millenniumFalconScale, millenniumFalconScale, millenniumFalconScale))
	millenniumFalconTurrets       = model.Transform(millenniumFalconGeometry.Turrets, math3d.Scaling(millenniumFalconScale, millenniumFalconScale, millenniumFalconScale))
	millenniumFalconHyperdrive    = model.Transform(millenniumFalconGeometry.Hyperdrive, math3d.Scaling(millenniumFalconScale, millenniumFalconScale, millenniumFalconScale))
	millenniumFalconDetails       = model.Transform(millenniumFalconGeometry.Details, math3d.Scaling(millenniumFalconScale, millenniumFalconScale, millenniumFalconScale))
	millenniumFalconDebris        = transformMillenniumFalconFragments(millenniumFalconGeometry.Fragments, math3d.Scaling(millenniumFalconScale, millenniumFalconScale, millenniumFalconScale))
	millenniumFalconPolygonShards = buildMillenniumFalconPolygonShards()
	deathStarGeometry             = model.DeathStar(300)
)

const tieFighterScale = 0.72
const tieInterceptorScale = 0.85

// Falcon geometry is authored directly in gameplay units so its 34.75 m
// length is approximately 2.8 times the 12.5 m X-Wing model.
const millenniumFalconScale = 1.0

const (
	CubeName                    = "builtin/cube"
	TIEFighterName              = "builtin/tie-fighter"
	TIEInterceptorName          = "builtin/tie-interceptor"
	XWingName                   = "builtin/x-wing"
	MillenniumFalconName        = "builtin/millennium-falcon"
	LaserBoltName               = "builtin/laser-bolt"
	DeathStarName               = "builtin/death-star"
	TIEInterceptorAppearance    = "builtin/tie-interceptor-model"
	RebelLaserBoltAppearance    = "builtin/laser-bolt-rebel"
	ImperialLaserBoltAppearance = "builtin/laser-bolt-imperial"
)

const standardLineWidth float32 = 2

// Specification is the compact technical data shown by the fighter showcase.
type Specification struct {
	Title, Type, Role, Description, Description2, Length, Crew, Passengers, MaxSpeed, Hyperdrive, Weapons, Ordnance, Shields string
}

func SpecificationFor(definition string) (Specification, bool) {
	switch definition {
	case XWingName:
		return Specification{Title: "ALLIANCE X WING STARFIGHTER", Type: "SPACE SUPERIORITY STARFIGHTER", Role: "REBEL ALLIANCE", Description: "VERSATILE ALLIANCE FIGHTER", Description2: "FOR FAST TORPEDO RUNS", Length: "12.5 METERS", Crew: "1 PILOT AND ASTROMECH", Passengers: "NONE", MaxSpeed: "1050 KM PER HOUR", Hyperdrive: "YES", Weapons: "4 LASER CANNONS", Ordnance: "2 PROTON TORPEDO LAUNCHERS", Shields: "YES"}, true
	case TIEFighterName:
		return Specification{Title: "IMPERIAL TIE FIGHTER", Type: "SPACE SUPERIORITY STARFIGHTER", Role: "GALACTIC EMPIRE", Description: "PRIMARY IMPERIAL SPACE", Description2: "SUPERIORITY FIGHTER", Length: "6.3 METERS", Crew: "1 PILOT", Passengers: "NONE", MaxSpeed: "1200 KM PER HOUR", Hyperdrive: "NO", Weapons: "2 LASER CANNONS", Ordnance: "NONE", Shields: "NO"}, true
	case TIEInterceptorName:
		return Specification{Title: "IMPERIAL TIE INTERCEPTOR", Type: "SPACE SUPERIORITY STARFIGHTER", Role: "GALACTIC EMPIRE", Description: "FAST IMPERIAL INTERCEPTOR", Description2: "FOR HIGH-SPEED PURSUIT", Length: "9.6 METERS", Crew: "1 PILOT", Passengers: "NONE", MaxSpeed: "111 MGLT", Hyperdrive: "NO", Weapons: "4 SFS L-S9.3 LASER CANNONS", Ordnance: "NONE", Shields: "NO"}, true
	case MillenniumFalconName:
		// Baseline values combine the official Databank length, the official
		// Ships of the Galaxy weapon fit, and the UCS/blueprint reference for
		// speed and hyperdrive class. They describe the familiar modified YT-1300
		// rather than every film-era configuration.
		return Specification{Title: "MILLENNIUM FALCON", Type: "YT-1300F LIGHT FREIGHTER", Role: "REBEL ALLIANCE", Description: "MODIFIED CORELLIAN LIGHT FREIGHTER", Description2: "FASTEST HUNK OF JUNK IN THE GALAXY", Length: "34.75 METERS", Crew: "1 PILOT 1 CO-PILOT 2 GUNNERS", Passengers: "6", MaxSpeed: "1050 KM PER HOUR", Hyperdrive: "CLASS 0.5", Weapons: "2 CEC AG-2G QUAD LASER CANNONS", Ordnance: "2 CONCUSSION MISSILE TUBES", Shields: "YES"}, true
	default:
		return Specification{}, false
	}
}

func transformTIEInterceptorParts(source [4]model.Model, transform math3d.Mat4) [4]model.Model {
	var result [4]model.Model
	for index, part := range source {
		result[index] = model.Transform(part, transform)
	}
	return result
}

func transformTIEInterceptorEndPanels(source [2]model.Model, transform math3d.Mat4) [2]model.Model {
	var result [2]model.Model
	for index, part := range source {
		result[index] = model.Transform(part, transform)
	}
	return result
}

func transformTIEInterceptorFragments(source [3]model.Model, transform math3d.Mat4) [3]model.Model {
	var result [3]model.Model
	for index, fragment := range source {
		result[index] = model.Transform(fragment, transform)
	}
	return result
}

func xWingAnchors(assemblies []model.XWingFoilAssembly) map[string]kinematics.Pose {
	anchors := map[string]kinematics.Pose{
		"center":  {Orientation: math3d.IdentityQuaternion()},
		"cockpit": {Position: math3d.Vec3{Y: 0.34, Z: 0.78}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
		"chase":   {Position: math3d.Vec3{Y: 1.8, Z: -5.8}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
	}
	muzzleNames := map[string]string{
		"upper-right S-foil": "muzzle-upper-right",
		"upper-left S-foil":  "muzzle-upper-left",
		"lower-left S-foil":  "muzzle-lower-left",
		"lower-right S-foil": "muzzle-lower-right",
	}
	for _, assembly := range assemblies {
		if name, ok := muzzleNames[assembly.Name]; ok {
			anchors[name] = kinematics.Pose{Position: assembly.Muzzle, Orientation: math3d.IdentityQuaternion()}
		}
	}
	return anchors
}

func tieFighterAnchors() map[string]kinematics.Pose {
	return map[string]kinematics.Pose{
		"center":             {Orientation: math3d.IdentityQuaternion()},
		"cockpit":            {Position: math3d.Vec3{Y: -0.05, Z: -0.22}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
		"chase":              {Position: math3d.Vec3{Y: 0.8, Z: -3}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
		"muzzle-upper-left":  {Position: math3d.Vec3{X: -0.42, Y: 0.18, Z: 0.82}, Orientation: math3d.IdentityQuaternion()},
		"muzzle-upper-right": {Position: math3d.Vec3{X: 0.42, Y: 0.18, Z: 0.82}, Orientation: math3d.IdentityQuaternion()},
		"muzzle-lower-left":  {Position: math3d.Vec3{X: -0.42, Y: -0.28, Z: 0.82}, Orientation: math3d.IdentityQuaternion()},
		"muzzle-lower-right": {Position: math3d.Vec3{X: 0.42, Y: -0.28, Z: 0.82}, Orientation: math3d.IdentityQuaternion()},
	}
}

func tieInterceptorAnchors() map[string]kinematics.Pose {
	anchors := map[string]kinematics.Pose{
		"center":  {Orientation: math3d.IdentityQuaternion()},
		"cockpit": {Position: math3d.Vec3{Z: 0.24 + model.TIEInterceptorCoreForwardZ}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
		"chase":   {Position: math3d.Vec3{Y: 1.15, Z: -3.8 + model.TIEInterceptorCoreForwardZ}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
	}
	// The four cannons are mounted symmetrically around the pod. The muzzle
	// anchors are derived from the same local placements as the cannon meshes.
	placements := []struct {
		hingeX float64
		hingeY float64
		angle  float64
	}{
		{hingeX: model.TIEInterceptorWingX, hingeY: model.TIEInterceptorWingY, angle: math.Pi - model.TIEInterceptorWingAngle},
		{hingeX: model.TIEInterceptorWingX, hingeY: -model.TIEInterceptorWingY, angle: math.Pi + model.TIEInterceptorWingAngle},
		{hingeX: -model.TIEInterceptorWingX, hingeY: model.TIEInterceptorWingY, angle: model.TIEInterceptorWingAngle},
		{hingeX: -model.TIEInterceptorWingX, hingeY: -model.TIEInterceptorWingY, angle: -model.TIEInterceptorWingAngle},
	}
	names := []string{"muzzle-upper-right", "muzzle-lower-right", "muzzle-upper-left", "muzzle-lower-left"}
	for index, placement := range placements {
		transform := math3d.Translation(placement.hingeX, placement.hingeY, 0).Mul(math3d.RotationZ(placement.angle))
		transform = math3d.Translation(0, 0, model.TIEInterceptorCoreForwardZ).Mul(transform)
		position := transform.TransformPoint(math3d.Vec3{X: 0.10, Z: model.TIEInterceptorCannonMuzzleZ})
		name := names[index]
		anchors[name] = kinematics.Pose{Position: position, Orientation: math3d.IdentityQuaternion()}
	}
	return anchors
}

// Cube returns a styled cube object suitable for pipeline demonstrations and
// scene-layout tests.
func Cube(id scene.ObjectID, size float64, pose kinematics.Pose) scene.Object {
	return scene.Object{
		ID:         id,
		Name:       "cube",
		Definition: CubeName,
		Pose:       pose,
		Parts: []scene.Part{{
			Name:      "hull",
			Mesh:      model.Cube(size),
			Color:     vectorRed,
			LineWidth: standardLineWidth,
		}},
		Anchors: map[string]kinematics.Pose{
			"center": {Orientation: math3d.IdentityQuaternion()},
		},
	}
}

// XWing returns the sparse Rebel fighter with four open S-foils, engines, and
// prominent wingtip cannons.
func XWing(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	parts := []scene.Part{
		{Name: "fuselage", Mesh: xWingGeometry.Fuselage, Color: vectorGreen, LineWidth: standardLineWidth},
		{Name: "canopy", Mesh: xWingGeometry.Canopy, Color: vectorGreen, LineWidth: standardLineWidth},
	}
	for _, assembly := range xWingFoilAssemblies {
		components := []struct {
			name string
			mesh model.Model
		}{
			{name: "wing panel", mesh: assembly.Wing},
			{name: "rear engine", mesh: assembly.RearEngine},
			{name: "forward engine", mesh: assembly.ForwardEngine},
			{name: "cannon", mesh: assembly.Cannon},
		}
		for _, component := range components {
			parts = append(parts, scene.Part{Name: assembly.Name + " " + component.name, Mesh: component.mesh, Color: vectorGreen, LineWidth: standardLineWidth})
		}
	}
	parts = append(parts, scene.Part{Name: "cockpit window", Mesh: xWingWindow, Color: windowAmber, LineWidth: standardLineWidth})
	return scene.Object{
		ID: id, Name: "X-Wing", Definition: XWingName, Pose: pose,
		Parts:         parts,
		Anchors:       xWingAnchors(xWingFoilAssemblies),
		CollisionRole: scene.CollisionSolid, CollisionRadius: 2.4,
		Physical: true, Hittable: true, Targetable: true, Destructible: true,
		DestructionStage: scene.DestructionIntact, VisualRadius: 4.3,
		DetailThresholds: scene.DetailThresholds{MediumPixels: 32, NearPixels: 90},
	}
}

func XWingFragment(id scene.ObjectID, index int, pose kinematics.Pose) scene.Object {
	if index < 0 || index >= len(xWingDebris) {
		panic("catalog: X-Wing fragment index out of range")
	}
	return scene.Object{ID: id, Name: "X-Wing debris", Definition: XWingName, Pose: pose,
		Parts:         []scene.Part{{Name: "fragment", Mesh: xWingDebris[index], Color: vectorGreen, LineWidth: standardLineWidth}},
		CollisionRole: scene.CollisionDebris, CollisionRadius: 1.2, Hittable: true, Destructible: true,
		DestructionStage: scene.DestructionComponent}
}

func XWingPolygonCount(component int) int {
	if component < 0 || component >= len(xWingDebris) {
		return 0
	}
	return len(xWingDebris[component].PolygonModels())
}

func XWingPolygon(id scene.ObjectID, component, polygon int, pose kinematics.Pose) scene.Object {
	if component < 0 || component >= len(xWingDebris) {
		panic("catalog: X-Wing polygon component out of range")
	}
	polygons := xWingDebris[component].PolygonModels()
	if polygon < 0 || polygon >= len(polygons) {
		panic("catalog: X-Wing polygon index out of range")
	}
	return scene.Object{ID: id, Name: "X-Wing polygon shard", Definition: XWingName, Pose: pose,
		Parts:         []scene.Part{{Name: "polygon", Mesh: polygons[polygon], Color: vectorGreen, LineWidth: standardLineWidth}},
		CollisionRole: scene.CollisionDebris, CollisionRadius: .15, DestructionStage: scene.DestructionPolygon}
}

// TIEFighterFragment returns one of three non-colliding debris pieces.
func TIEFighterFragment(id scene.ObjectID, index int, pose kinematics.Pose) scene.Object {
	if index < 0 || index >= len(tieFighterDebris) {
		panic("catalog: TIE fighter fragment index out of range")
	}
	parts := []scene.Part{{
		Name:      "fragment-hull",
		Mesh:      tieFighterDebris[index],
		Color:     vectorGreen,
		LineWidth: standardLineWidth,
	}}
	if index == 1 {
		parts = append(parts, scene.Part{
			Name:      "fragment-window",
			Mesh:      tieFighterWindow,
			Color:     windowAmber,
			LineWidth: standardLineWidth,
		})
	}
	return scene.Object{
		ID:               id,
		Name:             "TIE fighter debris",
		Definition:       TIEFighterName,
		Pose:             pose,
		Parts:            parts,
		CollisionRole:    scene.CollisionDebris,
		CollisionRadius:  1.25,
		Hittable:         true,
		Destructible:     true,
		DestructionStage: scene.DestructionComponent,
	}
}

type polygonShardTemplate struct {
	mesh  model.Model
	color color.RGBA
}

func transformTIEFighterFragments(source [3]model.Model, transform math3d.Mat4) [3]model.Model {
	var result [3]model.Model
	for index, fragment := range source {
		result[index] = model.Transform(fragment, transform)
	}
	return result
}

func buildTIEFighterPolygonShards() [3][]polygonShardTemplate {
	var shards [3][]polygonShardTemplate
	for component := range tieFighterDebris {
		for _, polygon := range tieFighterDebris[component].PolygonModels() {
			shards[component] = append(shards[component], polygonShardTemplate{mesh: polygon, color: vectorGreen})
		}
		if component == 1 {
			for _, polygon := range tieFighterWindow.PolygonModels() {
				shards[component] = append(shards[component], polygonShardTemplate{mesh: polygon, color: windowAmber})
			}
		}
	}
	return shards
}

func TIEFighterPolygonCount(component int) int {
	if component < 0 || component >= len(tieFighterPolygonShards) {
		return 0
	}
	return len(tieFighterPolygonShards[component])
}

// TIEFighterPolygon returns one final, non-targetable polygon shard.
func TIEFighterPolygon(id scene.ObjectID, component, polygon int, pose kinematics.Pose) scene.Object {
	if component < 0 || component >= len(tieFighterPolygonShards) ||
		polygon < 0 || polygon >= len(tieFighterPolygonShards[component]) {
		panic("catalog: TIE fighter polygon index out of range")
	}
	template := tieFighterPolygonShards[component][polygon]
	return scene.Object{
		ID:         id,
		Name:       "TIE fighter polygon shard",
		Definition: TIEFighterName,
		Pose:       pose,
		Parts: []scene.Part{{
			Name:      "polygon",
			Mesh:      template.mesh,
			Color:     template.color,
			LineWidth: standardLineWidth,
		}},
		CollisionRole:    scene.CollisionDebris,
		CollisionRadius:  0.15,
		DestructionStage: scene.DestructionPolygon,
	}
}

// TIEFighter returns the complete multipart fighter with its contrasting
// cockpit window.
func TIEFighter(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	return scene.Object{
		ID:         id,
		Name:       "TIE fighter",
		Definition: TIEFighterName,
		Pose:       pose,
		Parts: []scene.Part{
			{
				Name:      "cockpit and pylons",
				Mesh:      tieFighterCore,
				Color:     vectorGreen,
				LineWidth: standardLineWidth,
			},
			{
				Name:      "left solar-panel foil",
				Mesh:      tieFighterLeftFoil,
				Color:     vectorGreen,
				LineWidth: standardLineWidth,
			},
			{
				Name:      "right solar-panel foil",
				Mesh:      tieFighterRightFoil,
				Color:     vectorGreen,
				LineWidth: standardLineWidth,
			},
			{
				Name:      "windscreen",
				Mesh:      tieFighterWindow,
				Color:     windowAmber,
				LineWidth: standardLineWidth,
			},
		},
		Anchors:          tieFighterAnchors(),
		CollisionRole:    scene.CollisionSolid,
		CollisionRadius:  1.3,
		Physical:         true,
		Hittable:         true,
		Targetable:       true,
		Destructible:     true,
		DestructionStage: scene.DestructionIntact,
		VisualRadius:     1.3,
	}
}

// TIEInterceptor returns the four-panel Imperial interceptor. Its physical
// panels remain separate parts so projected geometry occlusion preserves the
// open spaces between them.
func TIEInterceptor(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	parts := []scene.Part{
		{Name: "command pod", Mesh: tieInterceptorCockpit, Color: vectorGreen, LineWidth: standardLineWidth},
		{Name: "wing pylons", Mesh: tieInterceptorPylons, Color: vectorGreen, LineWidth: standardLineWidth},
	}
	for index := range tieInterceptorEndPanels {
		parts = append(parts, scene.Part{Name: fmt.Sprintf("outer wing panel %d", index+1), Mesh: tieInterceptorEndPanels[index], Color: vectorGreen, LineWidth: standardLineWidth})
	}
	for index := range tieInterceptorPanels {
		parts = append(parts,
			scene.Part{Name: fmt.Sprintf("solar panel %d", index+1), Mesh: tieInterceptorPanels[index], Color: vectorGreen, LineWidth: standardLineWidth},
			scene.Part{Name: fmt.Sprintf("laser cannon %d", index+1), Mesh: tieInterceptorCannons[index], Color: vectorGreen, LineWidth: standardLineWidth, Detail: scene.DetailMedium},
		)
	}
	parts = append(parts, scene.Part{Name: "cockpit window", Mesh: tieInterceptorWindow, Color: windowAmber, LineWidth: standardLineWidth})
	return scene.Object{
		ID: id, Name: "TIE Interceptor", Definition: TIEInterceptorName, Appearance: TIEInterceptorAppearance, Pose: pose,
		Parts: parts, Anchors: tieInterceptorAnchors(),
		// The interceptor's elongated panel tips and wing cannons extend well
		// beyond the cockpit; keep the hit sphere aligned with that silhouette
		// so visible wing hits are not ignored by swept laser collision tests.
		CollisionRole: scene.CollisionSolid, CollisionRadius: 2.70,
		Physical: true, Hittable: true, Targetable: true, Destructible: true,
		DestructionStage: scene.DestructionIntact, VisualRadius: 2.80,
		DetailThresholds: scene.DetailThresholds{MediumPixels: 38, NearPixels: 105},
	}
}

// TIEInterceptorFragment returns one of the three component debris pieces.
func TIEInterceptorFragment(id scene.ObjectID, index int, pose kinematics.Pose) scene.Object {
	if index < 0 || index >= len(tieInterceptorDebris) {
		panic("catalog: TIE Interceptor fragment index out of range")
	}
	return scene.Object{
		ID: id, Name: "TIE Interceptor debris", Definition: TIEInterceptorName, Pose: pose,
		Parts:         []scene.Part{{Name: "fragment", Mesh: tieInterceptorDebris[index], Color: vectorGreen, LineWidth: standardLineWidth}},
		CollisionRole: scene.CollisionDebris, CollisionRadius: 1.35,
		Hittable: true, Destructible: true, DestructionStage: scene.DestructionComponent,
	}
}

func buildTIEInterceptorPolygonShards() [3][]polygonShardTemplate {
	var shards [3][]polygonShardTemplate
	for component := range tieInterceptorDebris {
		for _, polygon := range tieInterceptorDebris[component].PolygonModels() {
			shards[component] = append(shards[component], polygonShardTemplate{mesh: polygon, color: vectorGreen})
		}
	}
	return shards
}

func TIEInterceptorPolygonCount(component int) int {
	if component < 0 || component >= len(tieInterceptorPolygonShards) {
		return 0
	}
	return len(tieInterceptorPolygonShards[component])
}

// TIEInterceptorPolygon returns one final, non-targetable polygon shard.
func TIEInterceptorPolygon(id scene.ObjectID, component, polygon int, pose kinematics.Pose) scene.Object {
	if component < 0 || component >= len(tieInterceptorPolygonShards) || polygon < 0 || polygon >= len(tieInterceptorPolygonShards[component]) {
		panic("catalog: TIE Interceptor polygon index out of range")
	}
	template := tieInterceptorPolygonShards[component][polygon]
	return scene.Object{
		ID: id, Name: "TIE Interceptor polygon shard", Definition: TIEInterceptorName, Pose: pose,
		Parts:         []scene.Part{{Name: "polygon", Mesh: template.mesh, Color: template.color, LineWidth: standardLineWidth}},
		CollisionRole: scene.CollisionDebris, CollisionRadius: 0.15, DestructionStage: scene.DestructionPolygon,
	}
}

// MillenniumFalcon returns the multipart Rebel freighter with a closed hull,
// split mandibles, offset windscreen cockpit, quad turrets, hyperdrive segments,
// and sensor dish.
func MillenniumFalcon(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	return scene.Object{
		ID: id, Name: "Millennium Falcon", Definition: MillenniumFalconName, Pose: pose,
		Parts: []scene.Part{
			// The render layers share the welded authored boundaries from the
			// composite model, but use distinct depth owners and policies. This
			// preserves hull detail while making the extensions opaque.
			{Name: "saucer hull", Mesh: millenniumFalconHullCore, Color: vectorGreen, LineWidth: standardLineWidth, SelfOcclusion: scene.SelfOcclusionInterior},
			{Name: "left forward mandible", Mesh: millenniumFalconLeftMandible, Color: vectorGreen, LineWidth: standardLineWidth, SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll},
			{Name: "right forward mandible", Mesh: millenniumFalconRightMandible, Color: vectorGreen, LineWidth: standardLineWidth, SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll},
			{Name: "forward cargo ramp and roof", Mesh: millenniumFalconCargoRamp, Color: vectorGreen, LineWidth: standardLineWidth, SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll},
			{Name: "cockpit corridor", Mesh: millenniumFalconCorridor, Color: vectorGreen, LineWidth: standardLineWidth},
			{Name: "quad laser turrets", Mesh: millenniumFalconTurrets, Color: vectorGreen, LineWidth: standardLineWidth},
			{Name: "hyperdrive segments", Mesh: millenniumFalconHyperdrive, Color: windowAmber, LineWidth: standardLineWidth, SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll},
			{Name: "sensor dish and hull details", Mesh: millenniumFalconDetails, Color: vectorGreen, LineWidth: standardLineWidth, Detail: scene.DetailMedium},
			{Name: "cockpit windscreen", Mesh: millenniumFalconWindow, Color: windowAmber, LineWidth: standardLineWidth},
		},
		Anchors: map[string]kinematics.Pose{
			"center":             {Orientation: math3d.IdentityQuaternion()},
			"cockpit":            {Position: millenniumFalconStations.CockpitCenter, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
			"corridor-origin":    {Position: millenniumFalconStations.CorridorOrigin, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
			"corridor-bend":      {Position: millenniumFalconStations.CorridorBend, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
			"windscreen-mount":   {Position: millenniumFalconStations.WindscreenMount, Orientation: math3d.IdentityQuaternion()},
			"sensor-dish":        {Position: millenniumFalconStations.SensorDish, Orientation: math3d.IdentityQuaternion()},
			"chase":              {Position: math3d.Vec3{Y: 2.4, Z: -7.2}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
			"muzzle-upper-left":  {Position: math3d.Vec3{X: -0.20, Y: 0.72, Z: 0.82}, Orientation: math3d.IdentityQuaternion()},
			"muzzle-upper-right": {Position: math3d.Vec3{X: 0.20, Y: 0.72, Z: 0.82}, Orientation: math3d.IdentityQuaternion()},
			"muzzle-lower-left":  {Position: math3d.Vec3{X: -0.20, Y: -0.72, Z: 0.82}, Orientation: math3d.IdentityQuaternion()},
			"muzzle-lower-right": {Position: math3d.Vec3{X: 0.20, Y: -0.72, Z: 0.82}, Orientation: math3d.IdentityQuaternion()},
		},
		CollisionRole: scene.CollisionSolid, CollisionRadius: 6.4,
		Physical: true, Hittable: true, Targetable: true, Destructible: true,
		DestructionStage: scene.DestructionIntact, VisualRadius: 6.8,
		DetailThresholds: scene.DetailThresholds{MediumPixels: 44, NearPixels: 125},
	}
}

func MillenniumFalconFragment(id scene.ObjectID, index int, pose kinematics.Pose) scene.Object {
	if index < 0 || index >= len(millenniumFalconDebris) {
		panic("catalog: Millennium Falcon fragment index out of range")
	}
	return scene.Object{ID: id, Name: "Millennium Falcon debris", Definition: MillenniumFalconName, Pose: pose,
		Parts:         []scene.Part{{Name: "fragment", Mesh: millenniumFalconDebris[index], Color: vectorGreen, LineWidth: standardLineWidth}},
		CollisionRole: scene.CollisionDebris, CollisionRadius: 1.55, Hittable: true, Destructible: true,
		DestructionStage: scene.DestructionComponent}
}

func transformMillenniumFalconFragments(source [3]model.Model, transform math3d.Mat4) [3]model.Model {
	var result [3]model.Model
	for index, fragment := range source {
		result[index] = model.Transform(fragment, transform)
	}
	return result
}

func buildMillenniumFalconPolygonShards() [3][]polygonShardTemplate {
	var shards [3][]polygonShardTemplate
	for component := range millenniumFalconDebris {
		for _, polygon := range millenniumFalconDebris[component].PolygonModels() {
			shards[component] = append(shards[component], polygonShardTemplate{mesh: polygon, color: vectorGreen})
		}
	}
	return shards
}

func MillenniumFalconPolygonCount(component int) int {
	if component < 0 || component >= len(millenniumFalconPolygonShards) {
		return 0
	}
	return len(millenniumFalconPolygonShards[component])
}

func MillenniumFalconPolygon(id scene.ObjectID, component, polygon int, pose kinematics.Pose) scene.Object {
	if component < 0 || component >= len(millenniumFalconPolygonShards) || polygon < 0 || polygon >= len(millenniumFalconPolygonShards[component]) {
		panic("catalog: Millennium Falcon polygon index out of range")
	}
	template := millenniumFalconPolygonShards[component][polygon]
	return scene.Object{ID: id, Name: "Millennium Falcon polygon shard", Definition: MillenniumFalconName, Pose: pose,
		Parts:         []scene.Part{{Name: "polygon", Mesh: template.mesh, Color: template.color, LineWidth: standardLineWidth}},
		CollisionRole: scene.CollisionDebris, CollisionRadius: 0.15, DestructionStage: scene.DestructionPolygon}
}

// ProjectileStyle controls a projectile's visual identity independently of
// its shared geometry and combat behavior.
type ProjectileStyle struct {
	Appearance string
	RayColor   color.RGBA
	CoreColor  color.RGBA
	RayWidth   float32
	CoreWidth  float32
}

func rebelLaserBoltStyle() ProjectileStyle {
	return ProjectileStyle{
		Appearance: RebelLaserBoltAppearance,
		RayColor:   color.RGBA{R: 255, G: 64, B: 32, A: 255},
		CoreColor:  color.RGBA{R: 144, G: 224, B: 255, A: 255},
		RayWidth:   standardLineWidth,
		CoreWidth:  standardLineWidth,
	}
}

func imperialLaserBoltStyle() ProjectileStyle {
	return ProjectileStyle{
		Appearance: ImperialLaserBoltAppearance,
		RayColor:   color.RGBA{R: 64, G: 255, B: 96, A: 255},
		CoreColor:  color.RGBA{R: 255, G: 224, B: 64, A: 255},
		RayWidth:   standardLineWidth,
		CoreWidth:  standardLineWidth,
	}
}

// LaserBoltStyleForShooter selects the faction visual convention. Unknown
// definitions use the Rebel style until they choose a style explicitly.
func LaserBoltStyleForShooter(shooterDefinition string) ProjectileStyle {
	if shooterDefinition == TIEFighterName || shooterDefinition == TIEInterceptorName {
		return imperialLaserBoltStyle()
	}
	return rebelLaserBoltStyle()
}

// LaserBolt returns a Rebel-style fast projectile for callers without a
// shooter definition.
func LaserBolt(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	return LaserBoltWithStyle(id, pose, rebelLaserBoltStyle())
}

// LaserBoltForShooter creates a shared-geometry projectile styled for the
// shooter's faction.
func LaserBoltForShooter(id scene.ObjectID, pose kinematics.Pose, shooterDefinition string) scene.Object {
	return LaserBoltWithStyle(id, pose, LaserBoltStyleForShooter(shooterDefinition))
}

func LaserBoltWithStyle(id scene.ObjectID, pose kinematics.Pose, style ProjectileStyle) scene.Object {
	return scene.Object{
		ID:         id,
		Name:       "laser bolt",
		Definition: LaserBoltName,
		Appearance: style.Appearance,
		Pose:       pose,
		Parts: []scene.Part{
			{
				Name:      "rays",
				Mesh:      laserBoltRays,
				Color:     style.RayColor,
				LineWidth: style.RayWidth,
			},
			{
				Name:      "branches",
				Mesh:      laserBoltTips,
				Color:     style.CoreColor,
				LineWidth: style.CoreWidth,
			},
		},
		Anchors: map[string]kinematics.Pose{
			"center": {Orientation: math3d.IdentityQuaternion()},
		},
		CollisionRole:   scene.CollisionProjectile,
		CollisionRadius: 0.12,
	}
}

// DeathStar returns a large static object assembled from independently styled
// detail tiers. Its geometry remains ordinary object-local vector geometry.
func DeathStar(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	const radius = 300.0
	return scene.Object{
		ID: id, Name: "Death Star", Definition: DeathStarName, Pose: pose,
		Parts: []scene.Part{
			{Name: "sphere", Mesh: deathStarGeometry.Sphere, Color: vectorGreen, LineWidth: 1.5, Detail: scene.DetailPrimary},
			{Name: "superlaser dish", Mesh: deathStarGeometry.Dish, Color: vectorGreen, LineWidth: 2, Detail: scene.DetailMedium},
		},
		Anchors: map[string]kinematics.Pose{
			"center": {Orientation: math3d.IdentityQuaternion()},
			"target": {Position: math3d.Vec3{Z: -radius}, Orientation: math3d.IdentityQuaternion()},
			"dish":   {Position: math3d.Vec3{Y: 138, Z: -266}, Orientation: math3d.IdentityQuaternion()},
		},
		CollisionRole: scene.CollisionSolid, CollisionRadius: radius,
		Physical: true, Hittable: true, Targetable: true, Destructible: false,
		VisualRadius:     radius,
		DetailThresholds: scene.DetailThresholds{MediumPixels: 360, NearPixels: 430},
	}
}
