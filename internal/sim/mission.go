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
		return "ORBITAL BATTLE"
	case MissionApproach:
		return "APPROACH THE DEATH STAR"
	case MissionSurfaceAssault:
		return "FIND THE EXHAUST TRENCH"
	case MissionTrenchRun:
		return "BEGIN ATTACK RUN"
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
