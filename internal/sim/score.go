package sim

import (
	"fmt"

	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

// ScoreKind identifies an authored, score-bearing Yavin outcome. It is kept
// intentionally small: this is a mission score record, not a general economy.
type ScoreKind uint8

const (
	ScoreFighterKill ScoreKind = iota + 1
	ScoreSurfaceInstallation
	ScoreMissionCompletion
)

const (
	FighterKillPoints         = 100
	SurfaceInstallationPoints = 250
	MaximumCombatPoints       = 5000
	MissionCompletionBonus    = 10000
)

// MissionScore is a snapshot-safe, authoritative breakdown for one mission
// run. Combat awards are capped so endlessly respawning defenders can never
// outscore completion of the Death Star objective.
type MissionScore struct {
	FighterKills         int
	SurfaceInstallations int
	CombatPoints         int
	CompletionBonus      int
}

func (score MissionScore) Total() int {
	return score.CombatPoints + score.CompletionBonus
}

// ScoreEvent retains the player attribution for the score work observed on a
// simulation tick. The accumulated MissionScore is the durable run result;
// events are cleared at the next fixed tick like other world events.
type ScoreEvent struct {
	Tick     uint64
	SourceID scene.ObjectID
	Kind     ScoreKind
	Points   int
}

// AwardMissionScore is the only scoring command exposed by the simulation.
// It credits only the mission player and rejects inactive or terminal runs.
type AwardMissionScore struct {
	SourceID scene.ObjectID
	Kind     ScoreKind
}

func (command AwardMissionScore) Apply(world *World) error {
	mission := &world.Mission
	if mission.Phase == MissionInactive || mission.Phase.terminal() {
		return fmt.Errorf("mission cannot receive score from %s", mission.Phase)
	}
	if command.SourceID == 0 || command.SourceID != mission.PlayerID {
		return fmt.Errorf("score source %d does not control mission player %d", command.SourceID, mission.PlayerID)
	}
	if command.Kind == ScoreMissionCompletion {
		return fmt.Errorf("mission completion score requires mission success")
	}
	points, err := world.awardMissionScore(command.SourceID, command.Kind)
	if err != nil {
		return err
	}
	mission.Revision++
	world.ScoreEvents = append(world.ScoreEvents, ScoreEvent{Tick: world.Tick, SourceID: command.SourceID, Kind: command.Kind, Points: points})
	return nil
}

func (world *World) awardMissionScore(sourceID scene.ObjectID, kind ScoreKind) (int, error) {
	score := &world.Mission.Score
	switch kind {
	case ScoreFighterKill:
		score.FighterKills++
		return awardCombatPoints(score, FighterKillPoints), nil
	case ScoreSurfaceInstallation:
		score.SurfaceInstallations++
		return awardCombatPoints(score, SurfaceInstallationPoints), nil
	case ScoreMissionCompletion:
		if score.CompletionBonus != 0 {
			return 0, nil
		}
		score.CompletionBonus = MissionCompletionBonus
		world.ScoreEvents = append(world.ScoreEvents, ScoreEvent{Tick: world.Tick, SourceID: sourceID, Kind: kind, Points: MissionCompletionBonus})
		return MissionCompletionBonus, nil
	default:
		return 0, fmt.Errorf("unknown score kind %d", kind)
	}
}

func awardCombatPoints(score *MissionScore, requested int) int {
	available := MaximumCombatPoints - score.CombatPoints
	awarded := min(requested, max(0, available))
	score.CombatPoints += awarded
	return awarded
}
