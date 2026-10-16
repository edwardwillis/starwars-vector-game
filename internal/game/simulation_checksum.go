package game

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sort"

	"github.com/edwardwillis/starwars-vector-game/internal/control"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/edwardwillis/starwars-vector-game/internal/sim"
)

const simulationChecksumSchema = "starwars-vector-game/simulation-state/v1"

// simulationChecksum serializes the authoritative, future-affecting state in
// canonical order and hashes it. It deliberately excludes camera, renderer,
// telemetry, cached geometry, and other presentation-only state.
func (g *Game) simulationChecksum() (string, error) {
	if g.checksumObserver != nil {
		g.checksumObserver()
	}
	writer := newCanonicalStateWriter()
	writer.string(simulationChecksumSchema)
	if g.world == nil {
		writer.bool(false)
		return writer.sum()
	}
	writer.bool(true)
	writer.uint64(g.world.Tick)
	writer.float64("world.time", g.world.Time)
	writer.float64("simulation_time", g.simulationTime)
	writer.uint64(uint64(g.mode))
	writer.bool(g.surfaceAutoLevel)
	writer.bool(g.coordinatedTurnBank)
	writer.bool(g.benchmarkSafe)
	writer.bool(g.swarmLaunched)
	writer.bool(g.playerDestroyed)
	writer.uint64(uint64(g.nextObjectID))
	writer.uint64(g.respawnSequence)
	writer.int(g.shieldStrength)
	writer.float64("shield_quiet_time", g.shieldQuietTime)
	writer.float64("fire_cooldown", g.fireCooldown)
	writer.float64("torpedo_cooldown", g.torpedoCooldown)
	writer.int(g.torpedoesRemaining)
	writer.int(g.nextMuzzlePair)
	writer.int(g.kills)
	writer.int(g.collisions)
	writer.float64Slice("fire_history", g.fireHistory)
	writeMissionState(writer, g.world.Mission)
	writeWorldFrames(writer, g.world.Frames)
	writeObjects(writer, g.objects)
	writeControllers(writer, g)
	writeObjectFloatMap(writer, g.projectiles)
	writeObjectIDMap(writer, g.owners)
	writeObjectIDMap(writer, g.controllerTargets)
	writeObjectFloatMap(writer, g.environmentContacts)
	writeObjectVec3Map(writer, g.missionLastPosition)
	writeTransitions(writer, g)
	writeTransitionCommitments(writer, g.transitionCommitments)
	writeDebris(writer, g.debris)
	writeRespawns(writer, g.respawns)
	writeEnvironments(writer, g)
	return writer.sum()
}

type canonicalStateWriter struct {
	hash hashWriter
	err  error
}

type hashWriter struct {
	hash   [sha256.Size]byte
	buffer []byte
}

func newCanonicalStateWriter() *canonicalStateWriter { return &canonicalStateWriter{} }

func (writer *canonicalStateWriter) bytes(value []byte) {
	writer.hash.buffer = append(writer.hash.buffer, value...)
}

func (writer *canonicalStateWriter) uint64(value uint64) {
	var encoded [8]byte
	binary.LittleEndian.PutUint64(encoded[:], value)
	writer.bytes(encoded[:])
}

func (writer *canonicalStateWriter) int(value int) { writer.uint64(uint64(int64(value))) }
func (writer *canonicalStateWriter) float64(field string, value float64) {
	if writer.err == nil && (math.IsNaN(value) || math.IsInf(value, 0)) {
		writer.err = fmt.Errorf("non-finite authoritative state at %s", field)
	}
	writer.uint64(math.Float64bits(value))
}
func (writer *canonicalStateWriter) bool(value bool) {
	if value {
		writer.uint64(1)
		return
	}
	writer.uint64(0)
}
func (writer *canonicalStateWriter) string(value string) {
	writer.uint64(uint64(len(value)))
	writer.bytes([]byte(value))
}
func (writer *canonicalStateWriter) float64Slice(field string, values []float64) {
	writer.uint64(uint64(len(values)))
	for index, value := range values {
		writer.float64(fmt.Sprintf("%s[%d]", field, index), value)
	}
}
func (writer *canonicalStateWriter) vec3(field string, value math3d.Vec3) {
	writer.float64(field+".x", value.X)
	writer.float64(field+".y", value.Y)
	writer.float64(field+".z", value.Z)
}
func (writer *canonicalStateWriter) pose(field string, value kinematics.Pose) {
	writer.vec3(field+".position", value.Position)
	writer.float64(field+".orientation.w", value.Orientation.W)
	writer.float64(field+".orientation.x", value.Orientation.X)
	writer.float64(field+".orientation.y", value.Orientation.Y)
	writer.float64(field+".orientation.z", value.Orientation.Z)
}
func (writer *canonicalStateWriter) motion(field string, value kinematics.Motion) {
	writer.float64(field+".speed", value.Speed)
	writer.vec3(field+".velocity", value.Velocity)
	writer.float64(field+".yaw_rate", value.YawRate)
	writer.float64(field+".pitch_rate", value.PitchRate)
	writer.float64(field+".roll_rate", value.RollRate)
}
func (writer *canonicalStateWriter) sum() (string, error) {
	if writer.err != nil {
		return "", writer.err
	}
	writer.hash.hash = sha256.Sum256(writer.hash.buffer)
	return hex.EncodeToString(writer.hash.hash[:]), nil
}

func writeMissionState(writer *canonicalStateWriter, mission sim.MissionState) {
	writer.string(mission.ID)
	writer.uint64(uint64(mission.Phase))
	writer.uint64(uint64(mission.PlayerID))
	writer.uint64(uint64(mission.HostID))
	writer.uint64(mission.StartedTick)
	writer.uint64(mission.PhaseStartedTick)
	writer.uint64(mission.Revision)
	writer.string(mission.Reason)
	writer.uint64(mission.Progress.ApproachDeadlineTick)
	writer.uint64(mission.Progress.AssaultDeadlineTick)
	writer.float64("mission.progress.orbital_initial_host_distance", mission.Progress.OrbitalInitialHostDistance)
	writer.float64("mission.progress.orbital_closest_host_distance", mission.Progress.OrbitalClosestHostDistance)
	writer.uint64(mission.Progress.OrbitalEngagementTicks)
	writer.int(mission.Progress.TrenchCheckpoint)
	writer.uint64(mission.Progress.EscapeDeadlineTick)
	writer.int(mission.Score.FighterKills)
	writer.int(mission.Score.SurfaceInstallations)
	writer.int(mission.Score.CombatPoints)
	writer.int(mission.Score.CompletionBonus)
}

func writeWorldFrames(writer *canonicalStateWriter, frames map[scene.FrameID]sim.Frame) {
	ids := make([]string, 0, len(frames))
	for id := range frames {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	writer.uint64(uint64(len(ids)))
	for _, id := range ids {
		frame := frames[scene.FrameID(id)]
		writer.string(id)
		writer.uint64(uint64(frame.HostID))
		writer.string(frame.Environment)
		writer.pose("frame["+id+"].pose", frame.Pose)
	}
}

func writeObjects(writer *canonicalStateWriter, objects []scene.Object) {
	ordered := append([]scene.Object(nil), objects...)
	sort.Slice(ordered, func(first, second int) bool { return ordered[first].ID < ordered[second].ID })
	writer.uint64(uint64(len(ordered)))
	for _, object := range ordered {
		writer.uint64(uint64(object.ID))
		writer.string(object.Definition)
		writer.string(string(object.Team))
		writer.string(string(object.Frame))
		prefix := fmt.Sprintf("object[%d]", object.ID)
		writer.pose(prefix+".pose", object.Pose)
		writer.motion(prefix+".motion", object.Motion)
		writer.uint64(uint64(object.CollisionRole))
		writer.float64(prefix+".collision_radius", object.CollisionRadius)
		writer.string(string(object.ProjectileKind))
		writer.float64(prefix+".projectile_travel", object.ProjectileTravel)
		writer.bool(object.Physical)
		writer.bool(object.Hittable)
		writer.bool(object.Targetable)
		writer.bool(object.Destructible)
		writer.uint64(uint64(object.DestructionStage))
	}
}

func writeControllers(writer *canonicalStateWriter, game *Game) {
	ids := game.orderedControllerIDs()
	writer.uint64(uint64(len(ids)))
	for _, id := range ids {
		controller := game.controllers[id]
		writer.uint64(uint64(id))
		writer.uint64(uint64(game.controllerTargets[id]))
		if state, ok := controller.(control.ChecksumState); ok {
			writer.bool(true)
			digest, err := state.StateChecksum()
			if writer.err == nil && err != nil {
				writer.err = fmt.Errorf("controller %d checksum: %w", id, err)
			}
			writer.bytes(digest[:])
		} else {
			writer.bool(false)
		}
	}
}

func writeObjectFloatMap(writer *canonicalStateWriter, values map[scene.ObjectID]float64) {
	ids := orderedObjectIDs(values)
	writer.uint64(uint64(len(ids)))
	for _, id := range ids {
		writer.uint64(uint64(id))
		writer.float64(fmt.Sprintf("object_float[%d]", id), values[id])
	}
}

func writeObjectVec3Map(writer *canonicalStateWriter, values map[scene.ObjectID]math3d.Vec3) {
	ids := orderedObjectIDs(values)
	writer.uint64(uint64(len(ids)))
	for _, id := range ids {
		writer.uint64(uint64(id))
		writer.vec3(fmt.Sprintf("mission_last_position[%d]", id), values[id])
	}
}

func writeObjectIDMap(writer *canonicalStateWriter, values map[scene.ObjectID]scene.ObjectID) {
	ids := orderedObjectIDs(values)
	writer.uint64(uint64(len(ids)))
	for _, id := range ids {
		writer.uint64(uint64(id))
		writer.uint64(uint64(values[id]))
	}
}

func orderedObjectIDs[V any](values map[scene.ObjectID]V) []scene.ObjectID {
	ids := make([]scene.ObjectID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(first, second int) bool { return ids[first] < ids[second] })
	return ids
}

func writeTransitions(writer *canonicalStateWriter, game *Game) {
	ids := game.orderedTransitionIDs()
	writer.uint64(uint64(len(ids)))
	for _, id := range ids {
		transition := game.transitions[id]
		writer.uint64(uint64(id))
		writer.string(string(transition.source))
		writer.string(string(transition.destination))
		writer.string(transition.anchor)
		prefix := fmt.Sprintf("transition[%d]", id)
		writer.float64(prefix+".duration", transition.duration)
		writer.float64(prefix+".elapsed", transition.elapsed)
		writer.pose(prefix+".start_pose", transition.startPose)
		writer.pose(prefix+".target_pose", transition.targetPose)
		writer.pose(prefix+".entry_pose", transition.entryPose)
		writer.motion(prefix+".motion", transition.motion)
		writer.vec3(prefix+".world_velocity", transition.worldVelocity)
		writer.float64(prefix+".roll_radians", transition.rollRadians)
		writer.bool(transition.preservePose)
		writer.vec3(prefix+".entry_offset", transition.entryOffset)
	}
}

func writeTransitionCommitments(writer *canonicalStateWriter, commitments map[scene.ObjectID]bool) {
	ids := orderedObjectIDs(commitments)
	writer.uint64(uint64(len(ids)))
	for _, id := range ids {
		writer.uint64(uint64(id))
		writer.bool(commitments[id])
	}
}

func writeDebris(writer *canonicalStateWriter, debris map[scene.ObjectID]destructionTransient) {
	ids := orderedObjectIDs(debris)
	writer.uint64(uint64(len(ids)))
	for _, id := range ids {
		transient := debris[id]
		writer.uint64(uint64(id))
		writer.float64(fmt.Sprintf("debris[%d].remaining", id), transient.remaining)
		writer.uint64(uint64(transient.rootObjectID))
		writer.int(transient.componentIndex)
		writer.uint64(uint64(transient.stage))
		writer.vec3(fmt.Sprintf("debris[%d].source_origin", id), transient.sourceOrigin)
	}
}

func writeRespawns(writer *canonicalStateWriter, respawns []autonomousRespawn) {
	writer.uint64(uint64(len(respawns)))
	for index, respawn := range respawns {
		writer.float64(fmt.Sprintf("respawn[%d].ready_at", index), respawn.readyAt)
		writer.string(respawn.definition)
	}
}

func writeEnvironments(writer *canonicalStateWriter, game *Game) {
	runtimes := append([]localEnvironment(nil), game.environments...)
	sort.Slice(runtimes, func(first, second int) bool { return runtimes[first].bound.FrameID < runtimes[second].bound.FrameID })
	writer.uint64(uint64(len(runtimes)))
	for _, runtime := range runtimes {
		writer.string(string(runtime.bound.FrameID))
		writer.uint64(uint64(runtime.bound.HostID))
		writer.string(runtime.bound.Definition.Name)
		writeFeatureStates(writer, runtime.featureStates)
		writeEncounter(writer, runtime.encounter)
		writeActiveTileCoordinates(writer, runtime)
	}
}

func writeFeatureStates(writer *canonicalStateWriter, states map[string]featureDamageState) {
	ids := make([]string, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	writer.uint64(uint64(len(ids)))
	for _, id := range ids {
		state := states[id]
		writer.string(id)
		writer.int(state.Hits)
		writer.bool(state.Disabled)
		writer.bool(state.Destroyed)
	}
}

func writeEncounter(writer *canonicalStateWriter, encounter surfaceEncounterState) {
	writer.bool(encounter.started)
	writer.uint64(uint64(encounter.participant))
	writer.float64("encounter.next_wave_at", encounter.nextWaveAt)
	writer.float64("encounter.attack_enabled_at", encounter.attackEnabledAt)
	writer.int(encounter.wave)
	writer.uint64(uint64(encounter.missionPhase))
	known := make(map[string]bool, len(encounter.cannonReadyAt)+len(encounter.cannonAim))
	for id := range encounter.cannonReadyAt {
		known[id] = true
	}
	for id := range encounter.cannonAim {
		known[id] = true
	}
	ids := make([]string, 0, len(known))
	for id := range known {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	writer.uint64(uint64(len(ids)))
	for _, id := range ids {
		writer.string(id)
		readyAt, ready := encounter.cannonReadyAt[id]
		writer.bool(ready)
		writer.float64("encounter.cannon["+id+"].ready_at", readyAt)
		aim := encounter.cannonAim[id]
		writer.float64("encounter.cannon["+id+"].aim.yaw", aim.yaw)
		writer.float64("encounter.cannon["+id+"].aim.pitch", aim.pitch)
		writer.uint64(aim.shots)
	}
}

func writeActiveTileCoordinates(writer *canonicalStateWriter, runtime localEnvironment) {
	tiles := make([]struct{ x, z int }, 0, len(runtime.tiles))
	for coordinate := range runtime.tiles {
		tiles = append(tiles, struct{ x, z int }{coordinate.X, coordinate.Z})
	}
	sort.Slice(tiles, func(first, second int) bool {
		if tiles[first].x != tiles[second].x {
			return tiles[first].x < tiles[second].x
		}
		return tiles[first].z < tiles[second].z
	})
	writer.uint64(uint64(len(tiles)))
	for _, tile := range tiles {
		writer.int(tile.x)
		writer.int(tile.z)
	}
}
