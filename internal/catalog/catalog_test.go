package catalog

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestCubeReturnsValidObject(t *testing.T) {
	cube := Cube(1, 2, kinematics.Pose{Position: math3d.Vec3{X: 1, Y: 2, Z: 3}})
	if err := cube.Validate(); err != nil {
		t.Fatalf("Cube returned an invalid object: %v", err)
	}
	if len(cube.Parts) != 1 {
		t.Fatalf("Cube returned %d parts, want 1", len(cube.Parts))
	}
}

func TestTIEFighterReturnsValidMultipartObject(t *testing.T) {
	fighter := TIEFighter(1, kinematics.Pose{})
	if err := fighter.Validate(); err != nil {
		t.Fatalf("TIEFighter returned an invalid object: %v", err)
	}
	if len(fighter.Parts) != 4 {
		t.Fatalf("TIEFighter returned %d parts, want 4", len(fighter.Parts))
	}
	if fighter.Parts[0].Color == fighter.Parts[3].Color || fighter.Parts[1].Color == fighter.Parts[3].Color || fighter.Parts[2].Color == fighter.Parts[3].Color {
		t.Fatal("fighter hull and window use the same color")
	}
	if fighter.CollisionRole != scene.CollisionSolid || fighter.CollisionRadius <= 0 ||
		!fighter.Physical || !fighter.Hittable || !fighter.Destructible ||
		fighter.DestructionStage != scene.DestructionIntact {
		t.Fatalf("fighter has incorrect collision metadata")
	}
	if fighter.Parts[0].VisibleInCockpit || fighter.Parts[1].VisibleInCockpit || fighter.Parts[2].VisibleInCockpit || fighter.Parts[3].VisibleInCockpit {
		t.Fatal("fighter hull or windscreen is visible from inside the cockpit")
	}
	for _, name := range []string{
		"center", "cockpit", "chase",
		"muzzle-upper-left", "muzzle-upper-right",
		"muzzle-lower-left", "muzzle-lower-right",
	} {
		if _, ok := fighter.Anchor(name); !ok {
			t.Fatalf("fighter is missing %q camera anchor", name)
		}
	}
}

func TestTIEInterceptorReturnsValidMultipartObject(t *testing.T) {
	fighter := TIEInterceptor(1, kinematics.Pose{})
	if err := fighter.Validate(); err != nil {
		t.Fatalf("TIE Interceptor returned an invalid object: %v", err)
	}
	if fighter.Definition != TIEInterceptorName || fighter.Appearance != TIEInterceptorAppearance {
		t.Fatalf("definition=%q appearance=%q", fighter.Definition, fighter.Appearance)
	}
	if len(fighter.Parts) != 13 {
		t.Fatalf("TIE Interceptor returned %d parts, want 13", len(fighter.Parts))
	}
	if fighter.CollisionRole != scene.CollisionSolid || fighter.CollisionRadius <= 0 ||
		!fighter.Physical || !fighter.Hittable || !fighter.Targetable || !fighter.Destructible {
		t.Fatalf("interceptor has incorrect collision metadata")
	}
	for _, name := range []string{
		"center", "cockpit", "chase",
		"muzzle-upper-left", "muzzle-upper-right",
		"muzzle-lower-left", "muzzle-lower-right",
	} {
		if _, ok := fighter.Anchor(name); !ok {
			t.Fatalf("interceptor is missing %q anchor", name)
		}
	}
}

func TestMillenniumFalconReturnsValidMultipartObject(t *testing.T) {
	fighter := MillenniumFalcon(1, kinematics.Pose{})
	if err := fighter.Validate(); err != nil {
		t.Fatalf("MillenniumFalcon returned an invalid object: %v", err)
	}
	if len(fighter.Parts) != 6 {
		t.Fatalf("Millennium Falcon returned %d parts, want 6", len(fighter.Parts))
	}
	if fighter.CollisionRole != scene.CollisionSolid || fighter.CollisionRadius <= 0 ||
		!fighter.Physical || !fighter.Hittable || !fighter.Targetable || !fighter.Destructible {
		t.Fatalf("Falcon has incorrect collision metadata")
	}
	var hull, details *scene.Part
	for index := range fighter.Parts {
		switch fighter.Parts[index].Name {
		case "saucer hull, mandibles, and cargo assembly":
			hull = &fighter.Parts[index]
		case "sensor dish and hull details":
			details = &fighter.Parts[index]
		}
	}
	if hull == nil || details == nil ||
		hull.SelfOcclusion != scene.SelfOcclusionInterior ||
		hull.SelfOccluding || details.SelfOcclusion != scene.SelfOcclusionNone {
		t.Fatal("Falcon hull/mandibles/cargo assembly or sensor dish has incorrect self-occlusion policies")
	}
	for _, name := range []string{"center", "cockpit", "chase", "muzzle-upper-left", "muzzle-upper-right", "muzzle-lower-left", "muzzle-lower-right"} {
		if _, ok := fighter.Anchor(name); !ok {
			t.Fatalf("Falcon is missing %q anchor", name)
		}
	}
}

func TestMillenniumFalconSpecificationUsesFullReferenceFields(t *testing.T) {
	spec, ok := SpecificationFor(MillenniumFalconName)
	if !ok {
		t.Fatal("Millennium Falcon specification is not registered")
	}
	for field, value := range map[string]string{
		"title":      spec.Title,
		"length":     spec.Length,
		"max speed":  spec.MaxSpeed,
		"hyperdrive": spec.Hyperdrive,
		"weapons":    spec.Weapons,
		"ordnance":   spec.Ordnance,
	} {
		if value == "" {
			t.Fatalf("Millennium Falcon %s specification is empty", field)
		}
	}
}

func TestTIEInterceptorInstancesShareImmutableGeometry(t *testing.T) {
	first := TIEInterceptor(1, kinematics.Pose{})
	second := TIEInterceptor(2, kinematics.Pose{})
	if &first.Parts[0].Mesh.Verts[0] != &second.Parts[0].Mesh.Verts[0] {
		t.Fatal("interceptor instances do not share core geometry")
	}
	if &first.Parts[1].Mesh.Verts[0] != &second.Parts[1].Mesh.Verts[0] {
		t.Fatal("interceptor instances do not share panel geometry")
	}
}

func TestXWingMuzzleAnchorsFollowAssemblyGeometry(t *testing.T) {
	object := XWing(1, kinematics.Pose{})
	names := map[string]string{
		"upper-right S-foil": "muzzle-upper-right",
		"upper-left S-foil":  "muzzle-upper-left",
		"lower-left S-foil":  "muzzle-lower-left",
		"lower-right S-foil": "muzzle-lower-right",
	}
	for _, assembly := range xWingFoilAssemblies {
		anchorName := names[assembly.Name]
		anchor, ok := object.Anchor(anchorName)
		if !ok {
			t.Fatalf("missing anchor %q", anchorName)
		}
		if anchor.Position != assembly.Muzzle {
			t.Fatalf("anchor %q=%+v, want assembly muzzle %+v", anchorName, anchor.Position, assembly.Muzzle)
		}
	}
}

func TestTIEFighterFragmentIsNonCollidingDebris(t *testing.T) {
	for index := range 3 {
		fragment := TIEFighterFragment(scene.ObjectID(index+1), index, kinematics.Pose{})
		if err := fragment.Validate(); err != nil {
			t.Fatalf("fragment %d is invalid: %v", index, err)
		}
		if fragment.CollisionRole != scene.CollisionDebris || fragment.Physical ||
			!fragment.Hittable || !fragment.Destructible ||
			fragment.DestructionStage != scene.DestructionComponent {
			t.Fatalf("fragment %d has incorrect collision metadata", index)
		}
	}
}

func TestTIEFighterPolygonsAreFinalVisualDebris(t *testing.T) {
	for component := range 3 {
		count := TIEFighterPolygonCount(component)
		if count == 0 {
			t.Fatalf("component %d has no constituent polygons", component)
		}
		polygon := TIEFighterPolygon(1, component, 0, kinematics.Pose{})
		if err := polygon.Validate(); err != nil {
			t.Fatalf("component %d polygon is invalid: %v", component, err)
		}
		if polygon.CollisionRole != scene.CollisionDebris || polygon.Physical ||
			polygon.Hittable || polygon.Destructible ||
			polygon.DestructionStage != scene.DestructionPolygon {
			t.Fatalf("component %d polygon has incorrect collision metadata", component)
		}
	}
}

func TestTIEInterceptorPolygonsAreFinalVisualDebris(t *testing.T) {
	for component := range 3 {
		count := TIEInterceptorPolygonCount(component)
		if count == 0 {
			t.Fatalf("component %d has no constituent polygons", component)
		}
		polygon := TIEInterceptorPolygon(1, component, 0, kinematics.Pose{})
		if err := polygon.Validate(); err != nil {
			t.Fatalf("component %d polygon is invalid: %v", component, err)
		}
		if polygon.CollisionRole != scene.CollisionDebris || polygon.Physical || polygon.Hittable || polygon.Destructible || polygon.DestructionStage != scene.DestructionPolygon {
			t.Fatalf("component %d polygon has incorrect metadata", component)
		}
	}
}

func TestTIEFighterInstancesShareImmutableGeometry(t *testing.T) {
	first := TIEFighter(1, kinematics.Pose{})
	second := TIEFighter(2, kinematics.Pose{})
	if &first.Parts[0].Mesh.Verts[0] != &second.Parts[0].Mesh.Verts[0] {
		t.Fatal("fighter instances do not share catalog core geometry")
	}
	if &first.Parts[1].Mesh.Verts[0] != &second.Parts[1].Mesh.Verts[0] {
		t.Fatal("fighter instances do not share catalog foil geometry")
	}
	if &first.Parts[2].Mesh.Verts[0] != &second.Parts[2].Mesh.Verts[0] {
		t.Fatal("fighter instances do not share catalog foil geometry")
	}
	if &first.Parts[3].Mesh.Verts[0] != &second.Parts[3].Mesh.Verts[0] {
		t.Fatal("fighter instances do not share catalog window geometry")
	}
}

func TestLaserBoltReturnsValidMultipartObject(t *testing.T) {
	bolt := LaserBolt(2, kinematics.Pose{})
	if err := bolt.Validate(); err != nil {
		t.Fatalf("LaserBolt returned an invalid object: %v", err)
	}
	if len(bolt.Parts) != 2 {
		t.Fatalf("LaserBolt returned %d parts, want 2", len(bolt.Parts))
	}
	if bolt.Parts[0].Color == bolt.Parts[1].Color {
		t.Fatal("laser rays and branches use the same color")
	}
	if bolt.CollisionRole != scene.CollisionProjectile || bolt.CollisionRadius <= 0 {
		t.Fatal("laser bolt has incorrect collision metadata")
	}
}

func TestLaserBoltStylesDistinguishRebelAndImperialFire(t *testing.T) {
	rebel := LaserBoltForShooter(2, kinematics.Pose{}, XWingName)
	imperial := LaserBoltForShooter(3, kinematics.Pose{}, TIEFighterName)
	if rebel.Appearance != RebelLaserBoltAppearance || imperial.Appearance != ImperialLaserBoltAppearance {
		t.Fatalf("appearances rebel=%q imperial=%q", rebel.Appearance, imperial.Appearance)
	}
	if rebel.Parts[0].Color == imperial.Parts[0].Color || rebel.Parts[1].Color == imperial.Parts[1].Color {
		t.Fatal("faction laser styles share colors")
	}
}

func TestTIEInterceptorUsesImperialLaserStyle(t *testing.T) {
	if style := LaserBoltStyleForShooter(TIEInterceptorName); style.Appearance != ImperialLaserBoltAppearance {
		t.Fatalf("interceptor style=%q, want imperial", style.Appearance)
	}
}

func TestDeathStarIsAValidStaticLargeObject(t *testing.T) {
	object := DeathStar(10, kinematics.Pose{})
	if err := object.Validate(); err != nil {
		t.Fatal(err)
	}
	if object.Motion != (kinematics.Motion{}) {
		t.Fatalf("motion=%+v", object.Motion)
	}
	if !object.Physical || !object.Hittable || !object.Targetable || object.Destructible {
		t.Fatalf("unexpected capabilities: %+v", object)
	}
	if len(object.Parts) != 2 || object.VisualRadius != object.CollisionRadius {
		t.Fatalf("parts=%d visual=%v collision=%v", len(object.Parts), object.VisualRadius, object.CollisionRadius)
	}
	if object.Parts[0].Name != "sphere" || object.Parts[1].Name != "superlaser dish" {
		t.Fatalf("orbital parts=%q, %q", object.Parts[0].Name, object.Parts[1].Name)
	}
}
