package sim

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestMissionScoreCreditsOnlyPlayerAndCapsCombat(t *testing.T) {
	player := catalog.XWing(1, kinematics.Pose{})
	world, err := New([]scene.Object{player})
	if err != nil {
		t.Fatal(err)
	}
	if err := world.Apply(StartMission{ID: "battle-of-yavin", PlayerID: player.ID}); err != nil {
		t.Fatal(err)
	}
	if err := world.Apply(AwardMissionScore{SourceID: 99, Kind: ScoreFighterKill}); err == nil {
		t.Fatal("non-player score source was accepted")
	}
	if err := world.Apply(AwardMissionScore{SourceID: player.ID, Kind: ScoreMissionCompletion}); err == nil {
		t.Fatal("completion score was accepted before mission success")
	}
	for range 80 {
		if err := world.Apply(AwardMissionScore{SourceID: player.ID, Kind: ScoreFighterKill}); err != nil {
			t.Fatal(err)
		}
	}
	score := world.Mission.Score
	if score.FighterKills != 80 || score.CombatPoints != MaximumCombatPoints || score.Total() != MaximumCombatPoints {
		t.Fatalf("capped combat score=%+v", score)
	}
	if len(world.ScoreEvents) != 80 || world.ScoreEvents[0].SourceID != player.ID || world.ScoreEvents[0].Kind != ScoreFighterKill {
		t.Fatalf("score attribution events=%+v", world.ScoreEvents)
	}
	if err := world.Step(.1); err != nil {
		t.Fatal(err)
	}
	if len(world.ScoreEvents) != 0 {
		t.Fatal("score events persisted beyond their fixed tick")
	}
}

func TestMissionCompletionDominatesCappedCombatScore(t *testing.T) {
	player := catalog.XWing(1, kinematics.Pose{})
	world, err := New([]scene.Object{player})
	if err != nil {
		t.Fatal(err)
	}
	if err := world.Apply(StartMission{ID: "battle-of-yavin", PlayerID: player.ID}); err != nil {
		t.Fatal(err)
	}
	for range 30 {
		if err := world.Apply(AwardMissionScore{SourceID: player.ID, Kind: ScoreSurfaceInstallation}); err != nil {
			t.Fatal(err)
		}
	}
	for _, phase := range []MissionPhase{MissionApproach, MissionSurfaceAssault, MissionTrenchRun, MissionExhaustPortAttack} {
		if err := world.Apply(AdvanceMission{To: phase, Reason: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := world.Apply(BeginEscape{DeadlineTick: 10, Reason: "test"}, AdvanceMission{To: MissionSucceeded, Reason: "safe-distance-reached"}); err != nil {
		t.Fatal(err)
	}
	score := world.Mission.Score
	if score.CompletionBonus != MissionCompletionBonus || score.Total() <= MaximumCombatPoints || score.Total() != MaximumCombatPoints+MissionCompletionBonus {
		t.Fatalf("completion score does not dominate combat: %+v", score)
	}
	if err := world.Apply(AwardMissionScore{SourceID: player.ID, Kind: ScoreFighterKill}); err == nil {
		t.Fatal("terminal mission accepted combat score")
	}
}
