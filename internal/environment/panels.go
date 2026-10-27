package environment

import (
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

// PanelFamilyID names an architectural visual language. It deliberately
// carries no scene or gameplay state: a family only describes the reusable
// material convention used by environment geometry.
type PanelFamilyID string

const (
	// DeathStarExteriorPanelFamily is the shared armour-panel language used by
	// the Death Star exterior. Later variants may share this identity while
	// selecting different textures or detail levels.
	DeathStarExteriorPanelFamily PanelFamilyID = "death-star-exterior"
)

// PanelFamily describes one repeatable panel-art material. TextureID is kept
// here rather than on placements so a family can evolve its artwork without
// changing the geometry that maps it. RepeatDistance is measured in the same
// local units as the mapping axes.
//
// This is intentionally small. Structural panel boundaries, feature geometry,
// and any near-detail vectors remain the responsibility of their environment
// definitions rather than becoming a second rendering system.
type PanelFamily struct {
	ID             PanelFamilyID
	TextureID      string
	RepeatDistance float64
}

// PanelSurfaceMapping maps a continuous architectural pattern onto an
// arbitrarily oriented surface. Origin and the two unit axes define a common
// coordinate frame shared by neighbouring panels; points mapped through the
// same frame therefore meet without resetting their decorative pattern.
//
// AxisU and AxisV are expected to be unit-length and perpendicular. Keeping
// that convention explicit avoids hidden scaling in UV generation and makes
// repeat distance meaningful across floors, walls, and ceilings.
type PanelSurfaceMapping struct {
	Family        PanelFamily
	Origin        math3d.Vec3
	AxisU, AxisV  math3d.Vec3
}

// UV maps one point into the family pattern field. An invalid repeat distance
// falls back to one unit so malformed internal definitions cannot generate
// non-finite UVs; concrete family definitions are covered by focused tests.
func (mapping PanelSurfaceMapping) UV(point math3d.Vec3) model.UV {
	repeatDistance := mapping.Family.RepeatDistance
	if repeatDistance <= 0 {
		repeatDistance = 1
	}
	relative := point.Sub(mapping.Origin)
	return model.UV{
		U: relative.Dot(mapping.AxisU) / repeatDistance,
		V: relative.Dot(mapping.AxisV) / repeatDistance,
	}
}

// UVs maps a face's vertices in their existing winding order. It performs no
// tessellation and allocates only the face-sized UV slice created with the
// model, so static tile construction remains the only point at which this work
// occurs.
func (mapping PanelSurfaceMapping) UVs(points []math3d.Vec3) []model.UV {
	uvs := make([]model.UV, len(points))
	for index, point := range points {
		uvs[index] = mapping.UV(point)
	}
	return uvs
}
