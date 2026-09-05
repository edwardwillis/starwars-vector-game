// Package catalog assembles reusable wireframe models into styled scene objects.
package catalog

import (
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
	vectorBlue  = color.RGBA{R: 48, G: 96, B: 255, A: 255}
	windowAmber = color.RGBA{R: 255, G: 192, B: 48, A: 255}

	tieFighterCore          = model.Transform(model.TIEFighterCore(), math3d.Scaling(0.72, 0.72, 0.72))
	tieFighterLeftFoil      = model.Transform(model.TIEFighterFoil(-1), math3d.Scaling(0.72, 0.72, 0.72))
	tieFighterRightFoil     = model.Transform(model.TIEFighterFoil(1), math3d.Scaling(0.72, 0.72, 0.72))
	tieFighterWindow        = model.Transform(model.TIEFighterWindow(), math3d.Scaling(0.72, 0.72, 0.72))
	tieFighterDebris        = model.TIEFighterFragments()
	xWingCoreParts           = model.XWingCoreParts()
	xWingFoilParts           = model.XWingFoilParts()
	xWingWindow              = model.XWingWindow()
	xWingDebris              = model.XWingFragments()
	laserBoltRays           = model.LaserBoltRays()
	laserBoltTips           = model.LaserBoltBranches()
	tieFighterPolygonShards = buildTIEFighterPolygonShards()
	deathStarGeometry       = model.DeathStar(300)
)

const (
	CubeName       = "builtin/cube"
	TIEFighterName = "builtin/tie-fighter"
	XWingName      = "builtin/x-wing"
	LaserBoltName  = "builtin/laser-bolt"
	DeathStarName  = "builtin/death-star"
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
	default:
		return Specification{}, false
	}
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
		{Name: "fuselage", Mesh: xWingCoreParts[0], Color: vectorGreen, LineWidth: standardLineWidth},
		{Name: "canopy", Mesh: xWingCoreParts[1], Color: vectorGreen, LineWidth: standardLineWidth},
	}
	foilNames := [...]string{"upper-right S-foil", "upper-left S-foil", "lower-left S-foil", "lower-right S-foil"}
	componentNames := [...]string{"wing panel", "rear engine", "forward engine", "cannon"}
	for index, foil := range xWingFoilParts {
		parts = append(parts, scene.Part{Name: foilNames[index/4] + " " + componentNames[index%4], Mesh: foil, Color: vectorGreen, LineWidth: standardLineWidth})
	}
	parts = append(parts, scene.Part{Name: "cockpit window", Mesh: xWingWindow, Color: windowAmber, LineWidth: standardLineWidth})
	return scene.Object{
		ID: id, Name: "X-Wing", Definition: XWingName, Pose: pose,
		Parts: parts,
		Anchors: map[string]kinematics.Pose{
			"center": {Orientation: math3d.IdentityQuaternion()},
			"cockpit": {Position: math3d.Vec3{Y: 0.34, Z: 0.78}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
			"chase": {Position: math3d.Vec3{Y: 1.8, Z: -5.8}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
			"muzzle-upper-left": {Position: math3d.Vec3{X: -2.35, Y: 0.72, Z: 1.02}, Orientation: math3d.IdentityQuaternion()},
			"muzzle-upper-right": {Position: math3d.Vec3{X: 2.35, Y: 0.72, Z: 1.02}, Orientation: math3d.IdentityQuaternion()},
			"muzzle-lower-left": {Position: math3d.Vec3{X: -1.65, Y: -0.72, Z: 1.02}, Orientation: math3d.IdentityQuaternion()},
			"muzzle-lower-right": {Position: math3d.Vec3{X: 1.65, Y: -0.72, Z: 1.02}, Orientation: math3d.IdentityQuaternion()},
		},
		CollisionRole: scene.CollisionSolid, CollisionRadius: 2.4,
		Physical: true, Hittable: true, Targetable: true, Destructible: true,
		DestructionStage: scene.DestructionIntact, VisualRadius: 4.3,
		DetailThresholds: scene.DetailThresholds{MediumPixels: 32, NearPixels: 90},
	}
}

func XWingFragment(id scene.ObjectID, index int, pose kinematics.Pose) scene.Object {
	if index < 0 || index >= len(xWingDebris) { panic("catalog: X-Wing fragment index out of range") }
	return scene.Object{ID: id, Name: "X-Wing debris", Definition: XWingName, Pose: pose,
		Parts: []scene.Part{{Name: "fragment", Mesh: xWingDebris[index], Color: vectorGreen, LineWidth: standardLineWidth}},
		CollisionRole: scene.CollisionDebris, CollisionRadius: 1.2, Hittable: true, Destructible: true,
		DestructionStage: scene.DestructionComponent}
}

func XWingPolygonCount(component int) int {
	if component < 0 || component >= len(xWingDebris) { return 0 }
	return len(xWingDebris[component].PolygonModels())
}

func XWingPolygon(id scene.ObjectID, component, polygon int, pose kinematics.Pose) scene.Object {
	if component < 0 || component >= len(xWingDebris) { panic("catalog: X-Wing polygon component out of range") }
	polygons := xWingDebris[component].PolygonModels()
	if polygon < 0 || polygon >= len(polygons) { panic("catalog: X-Wing polygon index out of range") }
	return scene.Object{ID: id, Name: "X-Wing polygon shard", Definition: XWingName, Pose: pose,
		Parts: []scene.Part{{Name: "polygon", Mesh: polygons[polygon], Color: vectorGreen, LineWidth: standardLineWidth}},
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
		Anchors: map[string]kinematics.Pose{
			"center": {
				Orientation: math3d.IdentityQuaternion(),
			},
			"cockpit": {
				Position:    math3d.Vec3{Y: -0.05, Z: -0.22},
				Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0),
			},
			"chase": {
				Position:    math3d.Vec3{Y: 0.8, Z: -3},
				Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0),
			},
			"muzzle-upper-left": {
				Position:    math3d.Vec3{X: -0.42, Y: 0.18, Z: 0.82},
				Orientation: math3d.IdentityQuaternion(),
			},
			"muzzle-upper-right": {
				Position:    math3d.Vec3{X: 0.42, Y: 0.18, Z: 0.82},
				Orientation: math3d.IdentityQuaternion(),
			},
			"muzzle-lower-left": {
				Position:    math3d.Vec3{X: -0.42, Y: -0.28, Z: 0.82},
				Orientation: math3d.IdentityQuaternion(),
			},
			"muzzle-lower-right": {
				Position:    math3d.Vec3{X: 0.42, Y: -0.28, Z: 0.82},
				Orientation: math3d.IdentityQuaternion(),
			},
		},
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

// LaserBolt returns a fast projectile rendered as red 3D rays with blue branch
// clusters at every ray tip.
func LaserBolt(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	return scene.Object{
		ID:         id,
		Name:       "laser bolt",
		Definition: LaserBoltName,
		Pose:       pose,
		Parts: []scene.Part{
			{
				Name:      "rays",
				Mesh:      laserBoltRays,
				Color:     vectorRed,
				LineWidth: standardLineWidth,
			},
			{
				Name:      "branches",
				Mesh:      laserBoltTips,
				Color:     vectorBlue,
				LineWidth: standardLineWidth,
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
