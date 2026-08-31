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

	twinPanelHull          = model.TwinPanelFighter()
	twinPanelWindow        = model.TwinPanelFighterWindow()
	twinPanelDebris        = model.TwinPanelFighterFragments()
	laserBoltRays          = model.LaserBoltRays()
	laserBoltTips          = model.LaserBoltBranches()
	twinPanelPolygonShards = buildTwinPanelPolygonShards()
)

const standardLineWidth float32 = 2

// Cube returns a styled cube object suitable for pipeline demonstrations and
// scene-layout tests.
func Cube(id scene.ObjectID, size float64, pose kinematics.Pose) scene.Object {
	return scene.Object{
		ID:   id,
		Name: "cube",
		Pose: pose,
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

// TwinPanelFighterFragment returns one of three non-colliding debris pieces.
func TwinPanelFighterFragment(id scene.ObjectID, index int, pose kinematics.Pose) scene.Object {
	if index < 0 || index >= len(twinPanelDebris) {
		panic("catalog: twin-panel fighter fragment index out of range")
	}
	parts := []scene.Part{{
		Name:      "fragment-hull",
		Mesh:      twinPanelDebris[index],
		Color:     vectorGreen,
		LineWidth: standardLineWidth,
	}}
	if index == 1 {
		parts = append(parts, scene.Part{
			Name:      "fragment-window",
			Mesh:      twinPanelWindow,
			Color:     windowAmber,
			LineWidth: standardLineWidth,
		})
	}
	return scene.Object{
		ID:               id,
		Name:             "twin-panel fighter debris",
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

func buildTwinPanelPolygonShards() [3][]polygonShardTemplate {
	var shards [3][]polygonShardTemplate
	for component := range twinPanelDebris {
		for _, polygon := range twinPanelDebris[component].PolygonModels() {
			shards[component] = append(shards[component], polygonShardTemplate{mesh: polygon, color: vectorGreen})
		}
		if component == 1 {
			for _, polygon := range twinPanelWindow.PolygonModels() {
				shards[component] = append(shards[component], polygonShardTemplate{mesh: polygon, color: windowAmber})
			}
		}
	}
	return shards
}

func TwinPanelFighterPolygonCount(component int) int {
	if component < 0 || component >= len(twinPanelPolygonShards) {
		return 0
	}
	return len(twinPanelPolygonShards[component])
}

// TwinPanelFighterPolygon returns one final, non-targetable polygon shard.
func TwinPanelFighterPolygon(id scene.ObjectID, component, polygon int, pose kinematics.Pose) scene.Object {
	if component < 0 || component >= len(twinPanelPolygonShards) ||
		polygon < 0 || polygon >= len(twinPanelPolygonShards[component]) {
		panic("catalog: twin-panel fighter polygon index out of range")
	}
	template := twinPanelPolygonShards[component][polygon]
	return scene.Object{
		ID:   id,
		Name: "twin-panel fighter polygon shard",
		Pose: pose,
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

// TwinPanelFighter returns the complete multipart fighter with its contrasting
// cockpit window.
func TwinPanelFighter(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	return scene.Object{
		ID:   id,
		Name: "twin-panel fighter",
		Pose: pose,
		Parts: []scene.Part{
			{
				Name:      "hull",
				Mesh:      twinPanelHull,
				Color:     vectorGreen,
				LineWidth: standardLineWidth,
			},
			{
				Name:      "windscreen",
				Mesh:      twinPanelWindow,
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
		CollisionRadius:  1.8,
		Physical:         true,
		Hittable:         true,
		Destructible:     true,
		DestructionStage: scene.DestructionIntact,
	}
}

// LaserBolt returns a fast projectile rendered as red 3D rays with blue branch
// clusters at every ray tip.
func LaserBolt(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	return scene.Object{
		ID:   id,
		Name: "laser bolt",
		Pose: pose,
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
