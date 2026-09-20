package sim

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestMissionCommandsEnforceYavinObjectiveOrder(t *testing.T) {
	player := catalog.XWing(1, kinematics.Pose{})
	host := catalog.DeathStar(9, kinematics.Pose{})
	world, err := New([]scene.Object{player, host})
	if err != nil {
		t.Fatal(err)
	}
	if err := world.Apply(StartMission{ID: "battle-of-yavin", PlayerID: player.ID, HostID: 9}); err != nil {
		t.Fatal(err)
	}
	if err := world.Apply(AdvanceMission{To: MissionTrenchRun, Reason: "shortcut"}); err == nil {
		t.Fatal("mission accepted an objective shortcut")
	}
	for _, phase := range []MissionPhase{MissionApproach, MissionSurfaceAssault, MissionTrenchRun, MissionExhaustPortAttack} {
		if err := world.Apply(AdvanceMission{To: phase, Reason: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	if world.Mission.Phase != MissionExhaustPortAttack || world.Mission.Revision != 5 || len(world.MissionEvents) != 5 {
		t.Fatalf("mission=%+v events=%+v", world.Mission, world.MissionEvents)
	}
	snapshot := world.Snapshot()
	snapshot.Mission.Phase = MissionFailed
	snapshot.MissionEvents[0].Reason = "changed"
	if world.Mission.Phase != MissionExhaustPortAttack || world.MissionEvents[0].Reason == "changed" {
		t.Fatal("mission snapshot aliases authoritative state")
	}
}

func TestMissionFailureIsIdempotentAndResettable(t *testing.T) {
	player := catalog.XWing(1, kinematics.Pose{})
	world, err := New([]scene.Object{player})
	if err != nil {
		t.Fatal(err)
	}
	if err := world.Apply(StartMission{ID: "battle-of-yavin", PlayerID: player.ID}, FailMission{Reason: "fighter-destroyed"}); err != nil {
		t.Fatal(err)
	}
	revision := world.Mission.Revision
	if err := world.Apply(FailMission{Reason: "duplicate"}); err != nil || world.Mission.Revision != revision {
		t.Fatal("duplicate failure changed terminal mission")
	}
	if err := world.Apply(ResetMission{}); err != nil {
		t.Fatal(err)
	}
	if world.Mission.Phase != MissionOrbitalBattle || world.Mission.Reason != "restart" || world.Mission.Revision != revision+1 {
		t.Fatalf("reset mission=%+v", world.Mission)
	}
	if err := world.Step(.1); err != nil {
		t.Fatal(err)
	}
	if len(world.MissionEvents) != 0 {
		t.Fatal("mission events persisted beyond their tick")
	}
}

func TestMissionFeedbackDoesNotAdvancePhase(t *testing.T) {
	player := catalog.XWing(1, kinematics.Pose{})
	world, err := New([]scene.Object{player})
	if err != nil {
		t.Fatal(err)
	}
	if err := world.Apply(StartMission{ID: "battle-of-yavin", PlayerID: player.ID}); err != nil {
		t.Fatal(err)
	}
	revision := world.Mission.Revision
	if err := world.Apply(ReportMissionFeedback{Reason: "attack-aim-off-center"}); err != nil {
		t.Fatal(err)
	}
	if world.Mission.Phase != MissionOrbitalBattle || world.Mission.Reason != "attack-aim-off-center" || world.Mission.Revision != revision+1 {
		t.Fatalf("feedback changed mission incorrectly: %+v", world.Mission)
	}
	if len(world.MissionEvents) != 2 || world.MissionEvents[1].From != world.MissionEvents[1].To {
		t.Fatalf("feedback events=%+v", world.MissionEvents)
	}
}
