package scene

import (
	"image/color"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

func TestObjectValidation(t *testing.T) {
	object := Object{
		ID:   1,
		Name: "test cube",
		Pose: kinematics.Pose{
			Position:    math3d.Vec3{X: 1, Y: 2, Z: 3},
			Orientation: math3d.IdentityQuaternion(),
		},
		Parts: []Part{{
			Mesh:      model.Cube(1),
			Color:     color.RGBA{R: 255, A: 255},
			LineWidth: 2,
		}},
		Anchors: map[string]kinematics.Pose{
			"top": {Position: math3d.Vec3{Y: 1}},
		},
	}
	if err := object.Validate(); err != nil {
		t.Fatalf("valid object was rejected: %v", err)
	}
	transformedOrigin := object.WorldMatrix().TransformPoint(math3d.Vec3{})
	if transformedOrigin != object.Pose.Position {
		t.Fatalf("WorldMatrix transformed origin to %+v, want %+v", transformedOrigin, object.Pose.Position)
	}
	anchor, ok := object.Anchor("top")
	if !ok || anchor.Position != (math3d.Vec3{X: 1, Y: 3, Z: 3}) {
		t.Fatalf("resolved anchor is %+v, %v", anchor, ok)
	}

	object.Parts[0].LineWidth = 0
	if err := object.Validate(); err == nil {
		t.Fatal("object with zero line width was accepted")
	}
}

func TestFlatOpaqueSurfaceValidation(t *testing.T) {
	part := Part{
		Mesh: model.Cube(1), Color: color.RGBA{G: 255, A: 255}, LineWidth: 1,
		Surface: SurfaceMaterial{Mode: SurfaceFlatOpaque, Color: color.RGBA{R: 8, G: 16, B: 24, A: 255}},
	}
	if err := part.Validate(); err != nil {
		t.Fatalf("valid flat opaque part: %v", err)
	}
	part.Surface.Color.A = 128
	if err := part.Validate(); err == nil {
		t.Fatal("translucent color accepted by opaque material")
	}
	part.Surface = SurfaceMaterial{Mode: SurfaceFlatOpaque, Color: color.RGBA{A: 255}}
	part.Mesh = model.Model{Verts: []math3d.Vec3{{}, {X: 1}}, Edges: []model.Edge{{A: 0, B: 1}}}
	if err := part.Validate(); err == nil {
		t.Fatal("line-only mesh accepted as a filled surface")
	}
}

func TestTexturedOpaqueSurfaceRequiresMappedFaces(t *testing.T) {
	part := Part{
		Mesh: model.Model{
			Verts: []math3d.Vec3{{}, {X: 1}, {Y: 1}},
			Faces: []model.Face{{Vertices: []int{0, 1, 2}, UVs: []model.UV{{}, {U: 1}, {V: 1}}}},
		},
		Color: color.RGBA{G: 255, A: 255}, LineWidth: 1,
		Surface: SurfaceMaterial{Mode: SurfaceTexturedOpaque, Color: color.RGBA{A: 255}, TextureID: "test/panel"},
	}
	if err := part.Validate(); err != nil {
		t.Fatalf("mapped textured face rejected: %v", err)
	}
	part.Mesh.Faces[0].UVs = nil
	if err := part.Validate(); err == nil {
		t.Fatal("textured face without UVs was accepted")
	}
}

func TestTranslucentSurfaceValidation(t *testing.T) {
	part := Part{Mesh: model.Cube(1), LineWidth: 1,
		Surface: SurfaceMaterial{Mode: SurfaceTranslucent, Color: color.RGBA{R: 80, A: 96}}}
	if err := part.Validate(); err != nil {
		t.Fatalf("valid translucent surface: %v", err)
	}
	for _, alpha := range []uint8{0, 255} {
		part.Surface.Color.A = alpha
		if err := part.Validate(); err == nil {
			t.Fatalf("accepted translucent alpha %d", alpha)
		}
	}
	part.Surface.Color.A = 96
	part.Surface.TextureID = "test/glass"
	if err := part.Validate(); err == nil {
		t.Fatal("accepted textured glass without face UVs")
	}
}
