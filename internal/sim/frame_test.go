package sim

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestTransferPreservesWorldPoseAcrossHostFrame(t *testing.T) {
	host := catalog.DeathStar(1, kinematics.Pose{Position: math3d.Vec3{Z: 400}, Orientation: math3d.IdentityQuaternion()})
	fighter := catalog.TIEFighter(2, kinematics.Pose{Position: math3d.Vec3{X: 4, Y: 3, Z: 90}, Orientation: math3d.IdentityQuaternion()})
	world, err := New([]scene.Object{host, fighter})
	if err != nil {
		t.Fatal(err)
	}
	frameID := scene.FrameID("death-star/trench")
	if err := world.Apply(RegisterFrame{Frame: Frame{ID: frameID, HostID: host.ID, Environment: "test/trench", Pose: kinematics.Pose{Position: math3d.Vec3{Z: -300}, Orientation: math3d.IdentityQuaternion()}}}); err != nil {
		t.Fatal(err)
	}
	localBefore, err := world.PoseInFrame(fighter.ID, frameID)
	if err != nil {
		t.Fatal(err)
	}
	if localBefore.Position != (math3d.Vec3{X: 4, Y: 3, Z: -10}) {
		t.Fatalf("local pose = %+v", localBefore.Position)
	}
	before, err := world.WorldPose(fighter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := world.Apply(Transfer{ObjectID: fighter.ID, Destination: frameID, Anchor: "approach"}); err != nil {
		t.Fatal(err)
	}
	after, err := world.WorldPose(fighter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Position.Sub(before.Position).Length() > 1e-9 {
		t.Fatalf("world pose moved: before=%+v after=%+v", before, after)
	}
	if len(world.Transitions) != 1 || world.Transitions[0].Destination != frameID ||
		world.Transitions[0].HostID != host.ID || world.Transitions[0].Environment != "test/trench" ||
		world.Transitions[0].Anchor != "approach" {
		t.Fatalf("transitions=%+v", world.Transitions)
	}
}

func TestTransferRejectsUnknownFrame(t *testing.T) {
	fighter := catalog.TIEFighter(1, kinematics.Pose{Orientation: math3d.IdentityQuaternion()})
	world, err := New([]scene.Object{fighter})
	if err != nil {
		t.Fatal(err)
	}
	if err := world.Apply(Transfer{ObjectID: fighter.ID, Destination: "missing"}); err == nil {
		t.Fatal("unknown destination accepted")
	}
}
