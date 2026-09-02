package catalog

import (
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"testing"
)

func TestDefaultRegistryCreatesLifecycleObjects(t *testing.T) {
	r := DefaultRegistry()
	object, err := r.Create(TwinPanelFighterName, 1, kinematics.Pose{})
	if err != nil || object.Definition != TwinPanelFighterName {
		t.Fatalf("create fighter: %v, definition=%q", err, object.Definition)
	}
	fragment, err := r.CreateFragment(TwinPanelFighterName, 2, 0, kinematics.Pose{})
	if err != nil || fragment.Definition != TwinPanelFighterName {
		t.Fatalf("create fragment: %v", err)
	}
	count, err := r.PolygonCount(TwinPanelFighterName, 0)
	if err != nil || count == 0 {
		t.Fatalf("polygon count: %d, %v", count, err)
	}
	if _, err := r.CreatePolygon(TwinPanelFighterName, 3, 0, 0, kinematics.Pose{}); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryRejectsUnknownAndDuplicateDefinitions(t *testing.T) {
	r := NewRegistry()
	def := Definition{Name: "test/object", Create: TwinPanelFighter}
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
