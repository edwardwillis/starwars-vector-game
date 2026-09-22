package sim

import (
	"fmt"

	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

type MissionPhase uint8

const (
	MissionInactive MissionPhase = iota
	MissionOrbitalBattle
	MissionApproach
	MissionSurfaceAssault
	MissionTrenchRun
	MissionExhaustPortAttack
	MissionEscape
	MissionSucceeded
	MissionFailed
)

func (phase MissionPhase) String() string {
	switch phase {
	case MissionOrbitalBattle:
		return "BREAK THROUGH IMPERIAL DEFENCES"
	case MissionApproach:
		return "APPROACH THE DEATH STAR"
	case MissionSurfaceAssault:
		return "FIND THE EXHAUST TRENCH"
	case MissionTrenchRun:
		return "FLY THE TRENCH"
	case MissionExhaustPortAttack:
		return "TARGET THE EXHAUST PORT"
	case MissionEscape:
		return "ESCAPE THE BLAST"
	case MissionSucceeded:
		return "MISSION COMPLETE"
	case MissionFailed:
		return "MISSION FAILED"
	default:
		return "AWAITING LAUNCH"
	}
}

func (phase MissionPhase) terminal() bool {
	return phase == MissionSucceeded || phase == MissionFailed
}

type MissionState struct {
	ID               string
	Phase            MissionPhase
	PlayerID         scene.ObjectID
	HostID           scene.ObjectID
	StartedTick      uint64
	PhaseStartedTick uint64
	Revision         uint64
	Reason           string
	Progress         MissionProgress
}

// MissionProgress holds compact, renderer-independent evidence used by an
// active mission.  It is deliberately mission-neutral: gameplay decides what
// constitutes an observation, while the simulation preserves the ordered
// state that snapshots and later authoritative hosts need to reproduce.
type MissionProgress struct {
	// OrbitalInitialHostDistance and OrbitalClosestHostDistance let an
	// approach rule require real closure on the mission host without using a
	// kill quota.  They are measured in the player's current simulation frame.
	OrbitalInitialHostDistance float64
	OrbitalClosestHostDistance float64
	OrbitalEngagementTicks     uint64

	// TrenchCheckpoint is the number of forward route checkpoints passed in
	// order.  It is intentionally a count rather than a world position so the
	// authored environment remains the source of geometric truth.
	TrenchCheckpoint int
}

type MissionEvent struct {
	Tick     uint64
	Mission  string
	From, To MissionPhase
	Reason   string
}

type StartMission struct {
	ID       string
	PlayerID scene.ObjectID
	HostID   scene.ObjectID
}

func (command StartMission) Apply(world *World) error {
	if command.ID == "" || command.PlayerID == 0 {
		return fmt.Errorf("mission requires ID and player")
	}
	if world.Mission.Phase != MissionInactive {
		return fmt.Errorf("mission %q is already active", world.Mission.ID)
	}
	if _, ok := world.byID(command.PlayerID); !ok {
		return fmt.Errorf("mission player %d not found", command.PlayerID)
	}
	if command.HostID != 0 {
		if _, ok := world.byID(command.HostID); !ok {
			return fmt.Errorf("mission host %d not found", command.HostID)
		}
	}
	world.Mission = MissionState{
		ID: command.ID, Phase: MissionOrbitalBattle, PlayerID: command.PlayerID, HostID: command.HostID,
		StartedTick: world.Tick, PhaseStartedTick: world.Tick, Revision: 1,
	}
	world.appendMissionEvent(MissionInactive, MissionOrbitalBattle, "launch")
	return nil
}

type AdvanceMission struct {
	To     MissionPhase
	Reason string
}

func (command AdvanceMission) Apply(world *World) error {
	mission := &world.Mission
	if mission.Phase == MissionInactive || mission.Phase.terminal() {
		return fmt.Errorf("mission is not advanceable from %s", mission.Phase)
	}
	if command.To != mission.Phase+1 || command.To > MissionSucceeded {
		return fmt.Errorf("invalid mission transition %s -> %s", mission.Phase, command.To)
	}
	world.setMissionPhase(command.To, command.Reason)
	return nil
}

type FailMission struct{ Reason string }

func (command FailMission) Apply(world *World) error {
	if world.Mission.Phase == MissionInactive {
		return fmt.Errorf("inactive mission cannot fail")
	}
	if world.Mission.Phase.terminal() {
		return nil
	}
	world.setMissionPhase(MissionFailed, command.Reason)
	return nil
}

// ReportMissionFeedback records authoritative, non-terminal objective
// feedback without pretending that the mission changed phase. It is suitable
// for rejected weapon attempts and is included in snapshots for remote pilots.
type ReportMissionFeedback struct{ Reason string }

func (command ReportMissionFeedback) Apply(world *World) error {
	if world.Mission.Phase == MissionInactive || world.Mission.Phase.terminal() {
		return fmt.Errorf("mission cannot report feedback from %s", world.Mission.Phase)
	}
	world.Mission.Reason = command.Reason
	world.Mission.Revision++
	world.appendMissionEvent(world.Mission.Phase, world.Mission.Phase, command.Reason)
	return nil
}

// ObserveMission records fixed-tick, authoritative evidence without changing
// phase.  Orbital observations establish and refine host closure, while a
// trench observation can advance by exactly one previously validated route
// checkpoint.  Geometry validation stays with the environment/gameplay layer;
// this command protects ordering in the simulation state.
type ObserveMission struct {
	HostDistance     float64
	UnderEngagement  bool
	TrenchCheckpoint int // one-based; zero means no checkpoint observation
}

func (command ObserveMission) Apply(world *World) error {
	mission := &world.Mission
	if mission.Phase == MissionInactive || mission.Phase.terminal() {
		return fmt.Errorf("mission cannot record progress from %s", mission.Phase)
	}
	if command.HostDistance > 0 && mission.Phase == MissionOrbitalBattle {
		if mission.Progress.OrbitalInitialHostDistance == 0 {
			mission.Progress.OrbitalInitialHostDistance = command.HostDistance
			mission.Progress.OrbitalClosestHostDistance = command.HostDistance
		} else if command.HostDistance < mission.Progress.OrbitalClosestHostDistance {
			mission.Progress.OrbitalClosestHostDistance = command.HostDistance
		}
	}
	if command.UnderEngagement && mission.Phase == MissionOrbitalBattle {
		mission.Progress.OrbitalEngagementTicks++
	}
	if command.TrenchCheckpoint != 0 {
		if mission.Phase != MissionTrenchRun {
			return fmt.Errorf("mission cannot record trench checkpoint from %s", mission.Phase)
		}
		if command.TrenchCheckpoint != mission.Progress.TrenchCheckpoint+1 {
			return fmt.Errorf("invalid trench checkpoint %d after %d", command.TrenchCheckpoint, mission.Progress.TrenchCheckpoint)
		}
		mission.Progress.TrenchCheckpoint = command.TrenchCheckpoint
	}
	return nil
}

type ResetMission struct{}

func (ResetMission) Apply(world *World) error {
	if world.Mission.Phase == MissionInactive {
		return nil
	}
	previous := world.Mission.Phase
	world.Mission.Phase = MissionOrbitalBattle
	world.Mission.PhaseStartedTick = world.Tick
	world.Mission.Revision++
	world.Mission.Reason = "restart"
	world.Mission.Progress = MissionProgress{}
	world.appendMissionEvent(previous, MissionOrbitalBattle, "restart")
	return nil
}

func (world *World) setMissionPhase(phase MissionPhase, reason string) {
	previous := world.Mission.Phase
	world.Mission.Phase = phase
	world.Mission.PhaseStartedTick = world.Tick
	world.Mission.Revision++
	world.Mission.Reason = reason
	world.appendMissionEvent(previous, phase, reason)
}

func (world *World) appendMissionEvent(from, to MissionPhase, reason string) {
	world.MissionEvents = append(world.MissionEvents, MissionEvent{
		Tick: world.Tick, Mission: world.Mission.ID, From: from, To: to, Reason: reason,
	})
}
