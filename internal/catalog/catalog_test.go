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
	if len(fighter.Parts) != 7 {
		t.Fatalf("TIEFighter returned %d parts, want 7", len(fighter.Parts))
	}
	window := fighter.Parts[len(fighter.Parts)-1]
	for _, part := range fighter.Parts[:len(fighter.Parts)-1] {
		if part.Color == window.Color {
			t.Fatal("fighter hull and window use the same color")
		}
	}
	if fighter.Parts[0].SelfOcclusion != scene.SelfOcclusionInterior || !fighter.Parts[1].SelfOccluding || fighter.Parts[1].SelfOcclusion != scene.SelfOcclusionAll {
		t.Fatalf("fighter cockpit/pylon policies=%v/%v, want interior cockpit and full pylon occlusion", fighter.Parts[0].SelfOcclusion, fighter.Parts[1].SelfOcclusion)
	}
	if !fighter.Parts[2].SelfOccluding || fighter.Parts[2].SelfOcclusion != scene.SelfOcclusionAll {
		t.Fatalf("fighter reactor policy=%v, want full occlusion", fighter.Parts[2].SelfOcclusion)
	}
	if fighter.CollisionRole != scene.CollisionSolid || fighter.CollisionRadius <= 0 ||
		!fighter.Physical || !fighter.Hittable || !fighter.Destructible ||
		fighter.DestructionStage != scene.DestructionIntact {
		t.Fatalf("fighter has incorrect collision metadata")
	}
	for _, part := range fighter.Parts {
		if part.VisibleInCockpit {
			t.Fatalf("fighter part %q is visible from inside the cockpit", part.Name)
		}
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

func TestTIEAdvancedX1StartsAsCockpitAndFuselageCheckpoint(t *testing.T) {
	fighter := TIEAdvancedX1(1, kinematics.Pose{})
	if err := fighter.Validate(); err != nil {
		t.Fatalf("TIE Advanced x1 returned an invalid object: %v", err)
	}
	if fighter.Definition != TIEAdvancedX1Name || len(fighter.Parts) != 9 {
		t.Fatalf("x1 definition=%q parts=%d, want cockpit, reactor collar, quarter-disks, roots, two opaque arrays, laser barrels, and rear strut checkpoint", fighter.Definition, len(fighter.Parts))
	}
	if fighter.Parts[0].Name != "shared TIE command pod" || fighter.Parts[1].Name != "plain rear reactor collar" || fighter.Parts[2].Name != "primary central fuselage" || fighter.Parts[3].Name != "advanced wedge wing roots" || fighter.Parts[4].Name != "port folded solar array" || fighter.Parts[5].Name != "starboard folded solar array" || fighter.Parts[6].Name != "twin under-cockpit laser barrels" || fighter.Parts[7].Name != "centreline rear strut" || fighter.Parts[8].Name != "cockpit window" {
		t.Fatalf("x1 checkpoint parts=%q, %q, %q, %q, %q, %q, %q, %q, and %q, want shared pod, plain collar, quarter-disks, wedge roots, arrays, barrels, rear strut, and window", fighter.Parts[0].Name, fighter.Parts[1].Name, fighter.Parts[2].Name, fighter.Parts[3].Name, fighter.Parts[4].Name, fighter.Parts[5].Name, fighter.Parts[6].Name, fighter.Parts[7].Name, fighter.Parts[8].Name)
	}
	for _, parts := range [][]scene.Part{fighter.Parts[1:4], fighter.Parts[4:6], fighter.Parts[6:8]} {
		for _, part := range parts {
			if part.SelfOccluding || part.SelfOcclusion != scene.SelfOcclusionInterior {
				t.Fatalf("x1 structural part %q uses unstable self-occlusion policy", part.Name)
			}
		}
	}
	for _, array := range fighter.Parts[4:6] {
		if !array.Surface.Opaque() || array.SelfOccluding || array.SelfOcclusion != scene.SelfOcclusionInterior {
			t.Fatalf("solar-array opacity/occlusion=%+v/%v/%v, want opaque independent shells with stable interior self-occlusion", array.Surface, array.SelfOccluding, array.SelfOcclusion)
		}
	}
	for _, name := range []string{"center", "cockpit", "chase"} {
		if _, ok := fighter.Anchor(name); !ok {
			t.Fatalf("x1 checkpoint is missing %q", name)
		}
	}
	for _, name := range []string{"muzzle-upper-left", "muzzle-upper-right"} {
		anchor, ok := fighter.Anchor(name)
		if !ok || anchor.Position.Y >= 0 || anchor.Position.Z <= 0 {
			t.Fatalf("x1 cannon anchor %q=%+v/%v, want a forward under-cockpit muzzle", name, anchor, ok)
		}
	}
}

func TestTIEAdvancedX1SpecificationReflectsCurrentModel(t *testing.T) {
	specification, ok := SpecificationFor(TIEAdvancedX1Name)
	if !ok {
		t.Fatal("TIE Advanced x1 specification is not registered")
	}
	if specification.Type != "ADVANCED SPACE SUPERIORITY FIGHTER" ||
		specification.Description != "ELONGATED REAR FUSELAGE" ||
		specification.Description2 != "BENT SOLAR-ARRAY WINGS" ||
		specification.Length != "5.8 METERS" ||
		specification.Weapons != "2 FORWARD LASER CANNONS" {
		t.Fatalf("x1 technical data is stale: %+v", specification)
	}
}

func TestIntactFightersProvideBackingForFilledScenery(t *testing.T) {
	fighters := []struct {
		object scene.Object
		window string
	}{
		{XWing(1, kinematics.Pose{}), "cockpit window"},
		{TIEFighter(2, kinematics.Pose{}), "windscreen"},
		{TIEInterceptor(3, kinematics.Pose{}), "cockpit window"},
		{TIEAdvancedX1(4, kinematics.Pose{}), "cockpit window"},
		{MillenniumFalcon(5, kinematics.Pose{}), "cockpit windscreen"},
	}
	for _, fighter := range fighters {
		if err := fighter.object.Validate(); err != nil {
			t.Errorf("%s: %v", fighter.object.Name, err)
		}
		foundWindow := false
		for _, part := range fighter.object.Parts {
			if len(part.Mesh.Faces) == 0 {
				continue
			}
			if part.Name == fighter.window {
				foundWindow = true
				if !part.Surface.Translucent() || part.Surface.Color != windowGlass.Color {
					t.Errorf("%s window is not translucent glass: %+v", fighter.object.Name, part.Surface)
				}
			} else if !part.Surface.Opaque() {
				t.Errorf("%s part %q lost its opaque backing", fighter.object.Name, part.Name)
			}
		}
		if !foundWindow {
			t.Errorf("%s has no %q glass part", fighter.object.Name, fighter.window)
		}
	}
}

func TestMillenniumFalconReturnsValidMultipartObject(t *testing.T) {
	fighter := MillenniumFalcon(1, kinematics.Pose{})
	if err := fighter.Validate(); err != nil {
		t.Fatalf("MillenniumFalcon returned an invalid object: %v", err)
	}
	if len(fighter.Parts) != 9 {
		t.Fatalf("Millennium Falcon returned %d parts, want 9", len(fighter.Parts))
	}
	if fighter.CollisionRole != scene.CollisionSolid || fighter.CollisionRadius <= 0 ||
		!fighter.Physical || !fighter.Hittable || !fighter.Targetable || !fighter.Destructible {
		t.Fatalf("Falcon has incorrect collision metadata")
	}
	var hull, leftMandible, rightMandible, cargo, drive, details *scene.Part
	for index := range fighter.Parts {
		switch fighter.Parts[index].Name {
		case "saucer hull":
			hull = &fighter.Parts[index]
		case "left forward mandible":
			leftMandible = &fighter.Parts[index]
		case "right forward mandible":
			rightMandible = &fighter.Parts[index]
		case "forward cargo ramp and roof":
			cargo = &fighter.Parts[index]
		case "hyperdrive segments":
			drive = &fighter.Parts[index]
		case "sensor dish and hull details":
			details = &fighter.Parts[index]
		}
	}
	if hull == nil || leftMandible == nil || rightMandible == nil || cargo == nil || drive == nil || details == nil ||
		hull.SelfOcclusion != scene.SelfOcclusionInterior || hull.SelfOccluding ||
		!leftMandible.SelfOccluding || leftMandible.SelfOcclusion != scene.SelfOcclusionAll ||
		!rightMandible.SelfOccluding || rightMandible.SelfOcclusion != scene.SelfOcclusionAll ||
		!cargo.SelfOccluding || cargo.SelfOcclusion != scene.SelfOcclusionAll ||
		!drive.SelfOccluding || drive.SelfOcclusion != scene.SelfOcclusionAll ||
		details.SelfOcclusion != scene.SelfOcclusionNone {
		t.Fatal("Falcon hull, mandibles, cargo assembly, or sensor dish has incorrect self-occlusion policies")
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

func TestTIEAdvancedX1PolygonsAreFinalVisualDebris(t *testing.T) {
	for component := range 3 {
		count := TIEAdvancedX1PolygonCount(component)
		if count == 0 {
			t.Fatalf("x1 component %d has no constituent polygons", component)
		}
		polygon := TIEAdvancedX1Polygon(1, component, 0, kinematics.Pose{})
		if err := polygon.Validate(); err != nil {
			t.Fatalf("x1 component %d polygon is invalid: %v", component, err)
		}
		if polygon.CollisionRole != scene.CollisionDebris || polygon.Physical || polygon.Hittable || polygon.Destructible || polygon.DestructionStage != scene.DestructionPolygon {
			t.Fatalf("x1 component %d polygon has incorrect metadata", component)
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
	if bolt.CollisionRole != scene.CollisionProjectile || bolt.CollisionRadius <= 0 || bolt.ProjectileKind != scene.ProjectileLaser {
		t.Fatal("laser bolt has incorrect collision metadata")
	}
}

func TestProtonTorpedoHasDistinctAuthoritativePayload(t *testing.T) {
	torpedo := ProtonTorpedo(7, kinematics.Pose{})
	if err := torpedo.Validate(); err != nil {
		t.Fatalf("ProtonTorpedo returned an invalid object: %v", err)
	}
	if torpedo.Definition != ProtonTorpedoName || torpedo.Appearance != ProtonTorpedoAppearance ||
		torpedo.ProjectileKind != scene.ProjectileProtonTorpedo || torpedo.CollisionRole != scene.CollisionProjectile {
		t.Fatalf("incorrect torpedo identity: %+v", torpedo)
	}
	if _, err := DefaultRegistry().Lookup(ProtonTorpedoName); err != nil {
		t.Fatalf("torpedo is absent from default catalog: %v", err)
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
	if style := LaserBoltStyleForShooter(TIEAdvancedX1Name); style.Appearance != ImperialLaserBoltAppearance {
		t.Fatalf("x1 style=%q, want imperial", style.Appearance)
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
