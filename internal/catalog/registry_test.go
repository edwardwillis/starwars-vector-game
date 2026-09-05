package catalog

import (
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"testing"
)

func TestDefaultRegistryCreatesLifecycleObjects(t *testing.T) {
	r := DefaultRegistry()
	object, err := r.Create(TIEFighterName, 1, kinematics.Pose{})
	if err != nil || object.Definition != TIEFighterName {
		t.Fatalf("create fighter: %v, definition=%q", err, object.Definition)
	}
	fragment, err := r.CreateFragment(TIEFighterName, 2, 0, kinematics.Pose{})
	if err != nil || fragment.Definition != TIEFighterName {
		t.Fatalf("create fragment: %v", err)
	}
	count, err := r.PolygonCount(TIEFighterName, 0)
	if err != nil || count == 0 {
		t.Fatalf("polygon count: %d, %v", count, err)
	}
	if _, err := r.CreatePolygon(TIEFighterName, 3, 0, 0, kinematics.Pose{}); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultRegistryCreatesXWingLifecycleObjects(t *testing.T) {
	r := DefaultRegistry()
	object, err := r.Create(XWingName, 10, kinematics.Pose{})
	if err != nil || object.Definition != XWingName { t.Fatalf("create X-Wing: %v, definition=%q", err, object.Definition) }
	fragment, err := r.CreateFragment(XWingName, 11, 0, kinematics.Pose{})
	if err != nil { t.Fatalf("create X-Wing fragment: %v", err) }
	if count := XWingPolygonCount(0); count == 0 || fragment.Definition != XWingName { t.Fatalf("X-Wing lifecycle incomplete: count=%d", count) }
}

func TestRegistryRejectsUnknownAndDuplicateDefinitions(t *testing.T) {
	r := NewRegistry()
	def := Definition{Name: "test/object", Create: TIEFighter}
	if err := r.Register(def); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(def); err == nil {
		t.Fatal("duplicate definition accepted")
	}
	if _, err := r.Create("missing", 1, kinematics.Pose{}); err == nil {
		t.Fatal("unknown definition accepted")
	}
}

func TestDefaultRegistryCreatesDeathStar(t *testing.T) {
	object, err := DefaultRegistry().Create(DeathStarName, 20, kinematics.Pose{})
	if err != nil {
		t.Fatal(err)
	}
	if object.Definition != DeathStarName {
		t.Fatalf("definition=%q", object.Definition)
	}
}
