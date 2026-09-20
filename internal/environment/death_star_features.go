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

// Installation prototypes are authored once in unit coordinates. Generated
// tiles share their prepared topology and use an instance pose/scale; collider
// components use the same coordinates, so visual variation cannot silently
// detach the hit volume from the model.
type installationPrototype struct {
	parts, wreck []scene.Part
	boxes        []installationBox
	muzzles      []math3d.Vec3
	turretPart   int
	turretPivot  math3d.Vec3
	barrelPart   int
	barrelPivot  math3d.Vec3
	hitPoints    int
	disableAfter int
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
		hitPoints: 2,
	},
	"cannon": {
		parts: append(append(installationParts("cannon foundation", steppedInstallation(
			installationRing{-0.5, 0.42, 0.39}, installationRing{-0.27, 0.42, 0.39},
		)), installationParts("twin laser turret", installationOpenBottom(steppedInstallation(
			installationRing{-0.27, 0.31, 0.31}, installationRing{0.17, 0.31, 0.31},
			installationRing{0.17, 0.22, 0.26}, installationRing{0.31, 0.22, 0.26},
		)))...), installationParts("twin laser barrels", model.Merge(
			installationUnitBox(math3d.Vec3{X: -0.19, Y: 0.20, Z: 0.585}, math3d.Vec3{X: 0.12, Y: 0.12, Z: 0.65}, math3d.Vec3{Z: -1}),
			installationUnitBox(math3d.Vec3{X: 0.19, Y: 0.20, Z: 0.585}, math3d.Vec3{X: 0.12, Y: 0.12, Z: 0.65}, math3d.Vec3{Z: -1}),
		))...),
		wreck: installationWreck(),
		boxes: []installationBox{
			{center: math3d.Vec3{Y: -0.095}, half: math3d.Vec3{X: 0.42, Y: 0.405, Z: 0.39}},
			{center: math3d.Vec3{X: -0.19, Y: 0.20, Z: 0.585}, half: math3d.Vec3{X: 0.06, Y: 0.06, Z: 0.325}},
			{center: math3d.Vec3{X: 0.19, Y: 0.20, Z: 0.585}, half: math3d.Vec3{X: 0.06, Y: 0.06, Z: 0.325}},
		},
		muzzles:    []math3d.Vec3{{X: -0.19, Y: 0.20, Z: 0.91}, {X: 0.19, Y: 0.20, Z: 0.91}},
		turretPart: 1, turretPivot: math3d.Vec3{Y: -0.27},
		barrelPart: 2, barrelPivot: math3d.Vec3{Y: 0.20, Z: 0.26},
		hitPoints: 3, disableAfter: 2,
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
		hitPoints: 1,
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
		hitPoints: 2,
	},
}

func installationParts(name string, mesh model.Model) []scene.Part {
	return []scene.Part{{
		Name: name, Mesh: mesh, Color: color.RGBA{R: 120, G: 120, B: 120, A: 255}, LineWidth: 1.5,
		Surface:       deathStarSurfaceFill,
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
	return model.Transform(unit, math3d.Translation(center.X, center.Y, center.Z).Mul(math3d.Scaling(size.X, size.Y, size.Z)))
}

// steppedInstallation connects rectangular rings without hidden coincident
// caps. Equal-height rings form real horizontal ledges, not overlapping boxes.
func steppedInstallation(rings ...installationRing) model.Model {
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
		Boxes: boxes, Detail: scene.DetailMedium, Targetable: true, Hittable: true,
		HitPoints: prototype.hitPoints, DisableAfter: prototype.disableAfter,
	}, boxes
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
