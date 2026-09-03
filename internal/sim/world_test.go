package sim

import (
	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"testing"
)

func TestWorldStepAndSnapshot(t *testing.T) {
	object := catalog.TIEFighter(2, kinematics.Pose{})
	object.Motion = kinematics.Motion{Speed: 4}
	w, err := New([]scene.Object{object})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Step(0.5); err != nil {
		t.Fatal(err)
	}
	if w.Tick != 1 || w.Time != 0.5 || w.Objects[0].Pose.Position.Z != 2 {
		t.Fatalf("unexpected world state: %+v", w)
	}
	snapshot := w.Snapshot()
	snapshot.Objects[0].Pose.Position = math3d.Vec3{}
	if w.Objects[0].Pose.Position.Z == 0 {
		t.Fatal("snapshot aliases world objects")
	}
}

func TestWorldCommandsAreValidatedAndOrdered(t *testing.T) {
	a := catalog.TIEFighter(3, kinematics.Pose{})
	b := catalog.TIEFighter(1, kinematics.Pose{})
	w, err := New([]scene.Object{a})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Apply(Add{Object: b}, SetMotion{ID: 1, Motion: kinematics.Motion{Speed: 2}}); err != nil {
		t.Fatal(err)
	}
	if w.Objects[0].ID != 1 || w.Objects[1].ID != 3 {
		t.Fatalf("objects not ordered: %+v", w.Objects)
	}
	if err := w.Apply(Remove{ID: 99}); err == nil {
		t.Fatal("missing removal accepted")
	}
}
