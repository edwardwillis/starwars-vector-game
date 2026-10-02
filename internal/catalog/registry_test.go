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

func TestDefaultRegistryCreatesTIEWreckageWheelForShowcase(t *testing.T) {
	wheel, err := DefaultRegistry().Create(TIEWreckageWheelName, 4, kinematics.Pose{})
	if err != nil || wheel.Definition != TIEWreckageWheelName || wheel.Name != "TIE fighter wreckage wheel" ||
		len(wheel.Parts) != 1 || !wheel.Parts[0].Surface.Opaque() || !wheel.Parts[0].SelfOccluding {
		t.Fatalf("create TIE wreckage wheel: %v, object=%+v", err, wheel)
	}
}

func TestDefaultRegistryCreatesXWingLifecycleObjects(t *testing.T) {
	r := DefaultRegistry()
	object, err := r.Create(XWingName, 10, kinematics.Pose{})
	if err != nil || object.Definition != XWingName {
		t.Fatalf("create X-Wing: %v, definition=%q", err, object.Definition)
	}
	fragment, err := r.CreateFragment(XWingName, 11, 0, kinematics.Pose{})
	if err != nil {
		t.Fatalf("create X-Wing fragment: %v", err)
	}
	if count := XWingPolygonCount(0); count == 0 || fragment.Definition != XWingName {
		t.Fatalf("X-Wing lifecycle incomplete: count=%d", count)
	}
}

func TestDefaultRegistryCreatesMillenniumFalconLifecycleObjects(t *testing.T) {
	r := DefaultRegistry()
	object, err := r.Create(MillenniumFalconName, 15, kinematics.Pose{})
	if err != nil || object.Definition != MillenniumFalconName {
		t.Fatalf("create Millennium Falcon: %v, definition=%q", err, object.Definition)
	}
	fragment, err := r.CreateFragment(MillenniumFalconName, 16, 0, kinematics.Pose{})
	if err != nil || fragment.Definition != MillenniumFalconName {
		t.Fatalf("create Millennium Falcon fragment: %v", err)
	}
	count, err := r.PolygonCount(MillenniumFalconName, 0)
	if err != nil || count == 0 {
		t.Fatalf("Falcon polygon count: %d, %v", count, err)
	}
	if _, err := r.CreatePolygon(MillenniumFalconName, 17, 0, 0, kinematics.Pose{}); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultRegistryCreatesTIEInterceptorLifecycleObjects(t *testing.T) {
	r := DefaultRegistry()
	object, err := r.Create(TIEInterceptorName, 12, kinematics.Pose{})
	if err != nil || object.Definition != TIEInterceptorName {
		t.Fatalf("create interceptor: %v, definition=%q", err, object.Definition)
	}
	fragment, err := r.CreateFragment(TIEInterceptorName, 13, 0, kinematics.Pose{})
	if err != nil || fragment.Definition != TIEInterceptorName {
		t.Fatalf("create interceptor fragment: %v", err)
	}
	count, err := r.PolygonCount(TIEInterceptorName, 0)
	if err != nil || count == 0 {
		t.Fatalf("interceptor polygon count: %d, %v", count, err)
	}
	if _, err := r.CreatePolygon(TIEInterceptorName, 14, 0, 0, kinematics.Pose{}); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultRegistryCreatesTIEAdvancedX1CockpitCheckpoint(t *testing.T) {
	registry := DefaultRegistry()
	object, err := registry.Create(TIEAdvancedX1Name, 18, kinematics.Pose{})
	if err != nil || object.Definition != TIEAdvancedX1Name {
		t.Fatalf("create x1: %v, definition=%q", err, object.Definition)
	}
	fragment, err := registry.CreateFragment(TIEAdvancedX1Name, 19, 0, kinematics.Pose{})
	if err != nil || fragment.Definition != TIEAdvancedX1Name {
		t.Fatalf("create x1 fragment: %v, definition=%q", err, fragment.Definition)
	}
	if count, err := registry.PolygonCount(TIEAdvancedX1Name, 0); err != nil || count == 0 {
		t.Fatalf("x1 polygon count=%d, err=%v", count, err)
	}
	if _, err := registry.CreatePolygon(TIEAdvancedX1Name, 20, 0, 0, kinematics.Pose{}); err != nil {
		t.Fatalf("create x1 polygon: %v", err)
	}
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
