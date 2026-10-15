# Death Star Assault: Deterministic Flight Recording and Replay

## Step 1: Architecture Assessment and Minimal Implementation Plan

No code was modified as part of this assessment. The current architecture is close to supporting deterministic replay, but one simulation-order issue should be corrected before recording is added.

## A. Current architecture

- Startup accepts only `-profile` and `-telemetry` in `main.go`, constructs a `game.Game`, then runs standard Ebitengine updates and draws.
- `Game.Update` uses `profile.Simulation.TickSeconds` (currently `1/60` for Pilot) as its simulation delta. Physics advances through `world.Step(seconds * MotionScale)`.
- Session construction is already centralized:
  - `constructYavinSession`
  - `adoptYavinSession`
  - `launchYavinSession`
- The surface-development shortcut uses that same launch path, then calls `startInSurfaceMode` and `startYavinMission`. It is reproducible provided `surfaceDevelopment` becomes recorded session configuration.
- Player flight input is sampled directly in `readIntent`, then applied through the existing `control.Apply` path in `Game.Update`.
- Other simulation-affecting inputs are currently sampled separately in `Game.Update`: mode, auto-level, torpedo, fire, reset, and transition skip. UI, camera, and rendering commands are also sampled there and in `updateApplicationFlow`.
- Rendering is separate from simulation. `Game.Draw` prepares and renders the world; telemetry only observes timing and renderer statistics.

## B. Determinism assessment

| Source | Status | Action |
| --- | --- | --- |
| Fixed simulation delta | Deterministic | Record and validate tick duration. |
| Player physics | Deterministic once input is supplied | Introduce one normalized gameplay-input snapshot. |
| AI PRNG | Deterministic | Each pursuit controller is seeded from its object ID in `NewWithRegistriesAndAppearances`. |
| Starfield | Deterministic | Profile-owned seed; presentation-only for replay checksum purposes. |
| Death Star feature generation | Deterministic | Tile features use deterministic coordinate-derived seeds. |
| Autonomous controller iteration | **Not deterministic** | `updateAutonomous` ranges over `g.controllers`, a Go map. Decision/fire ordering can alter projectile IDs and later collisions. Sort controller IDs before stepping. |
| Transition iteration | Potentially nondeterministic | `advanceEnvironmentTransitions` ranges over a map. Canonicalize IDs before mutating or completing transitions. |
| Surface tile iteration | Low but real tie-order risk | Collision and cannon scans range tile maps. Cannon candidates are later sorted, but collision ties should eventually use a stable feature/tile tie-breaker. |
| Wall clock / telemetry | Non-simulation | `time.Now`, FPS/TPS, and telemetry do not affect game state; exclude from checksums. |
| Camera controls | Presentation, with caveat | Camera target can influence streamed tile selection. Benchmark playback should lock cockpit view and reject live camera inputs. |
| Rendering profile | Presentation-only | Permit changes for A/B rendering tests, but report them and keep them outside the simulation compatibility fingerprint. |

The map iteration in `updateAutonomous` is the only confirmed high-impact replay blocker.

## C. Minimal recording format

Use versioned JSON initially:

```json
{
  "format_version": 2,
  "build": { "commit": "2fe0513" },
  "session": {
    "mission": "battle-of-yavin",
    "surface_development_start": true,
    "profile_name": "builtin/pilot",
    "simulation_profile_digest": "sha256:...",
    "tick_seconds": 0.016666666666666666,
    "initial_view": "cockpit",
    "initial_realism": 5
  },
  "ticks": [
    {
      "throttle": 1,
      "yaw": -0.3,
      "pitch": 0,
      "roll": 0,
      "cockpit_aim_x": 0.5,
      "cockpit_aim_y": 0.5,
      "fire": true,
      "torpedo": false,
      "commands": ["manual"]
    }
  ],
  "checksums": [
    { "tick": 0, "state": "sha256:..." },
    { "tick": 300, "state": "sha256:..." }
  ],
  "total_ticks": 3600
}
```

Store normalized flight intent and the normalized, clamped cockpit aim point
rather than raw keys or mouse coordinates. JSON preserves inspectability;
later versions can add compact encoding without changing the gameplay boundary.

## D. Recording and playback integration

Add a narrow gameplay-input snapshot at the existing boundary:

1. Sample live controls once per active simulation tick.
2. Normalize to flight intent plus simulation commands:
   - throttle, yaw, pitch, roll
   - held laser fire
   - proton-torpedo pulse
   - manual/autopilot selection
   - surface auto-level state or change
3. Apply that snapshot using the existing `control.Apply`, projectile, and mission logic.
4. Recording serializes the snapshot.
5. Playback supplies the stored snapshot and ignores live gameplay input.

For initial benchmark recordings, begin after the shared session initializer has launched directly into `flowPlaying`; do not record title or briefing navigation. Pause, reset, camera changes, HUD toggles, realism changes, showcase controls, and quit controls should be disabled or rejected during playback rather than accidentally contaminating a run.

## E. Replay correctness and compatibility

Validate before replay:

- Supported recording format version.
- Mission and start mode.
- Tick duration.
- Simulation-profile digest.
- Required input count and checksum structure.
- Build identifier as diagnostic information, not a strict blocker: renderer-only changes must be replayable.

Checksums should use canonical sorted state and include:

- World tick, ordered objects, frames, mission state, and score state.
- Player shield, torpedoes, cooldowns, mode, and respawn sequence.
- Projectile lifetime/owner data sorted by object ID.
- Active authoritative surface-tile coordinates, feature damage, cannon/encounter state, and transitions sorted by ID.

Exclude GPU images, render statistics, telemetry, wall-clock values, cached cockpit artwork, depth buffers, horizon-only tiles, and camera presentation data.

Validate at a modest interval such as every 300 ticks. A mismatch should stop playback with the tick number and expected/actual digest; it must not silently resynchronize state.

## F. Telemetry integration

Existing telemetry remains separate from replay. CSV rows now include replay
provenance (`session_kind`, `recording_id`, `replay_verified`) and inclusive
simulation/input counter ranges (`simulation_tick_start`/`_end` and
`replay_input_start`/`_end`). This permits equivalent portions of two runs to
be compared even if FPS differs.

Suggested Windows workflow:

```text
starwars.exe -record surface-benchmark.json
starwars.exe -replay surface-benchmark.json -telemetry before.csv
starwars.exe -replay surface-benchmark.json -telemetry after.csv
```

For comparisons:

- Keep mission start, profile, logical resolution, cockpit view, and replay file identical.
- Permit rendering-only changes explicitly.
- Match CSV ranges by simulation tick, not wall-clock second.
- Confirm identical replay checksums and geometry counters before comparing draw/overlay/surface timing.
- Run at least three native Windows trials and compare median and worst sustained intervals; replay fixes simulation workload, not GPU scheduling or frame pacing.

## G. Incremental milestones

1. Canonicalize simulation ordering: sort autonomous controller IDs; add deterministic regression coverage.
   - Acceptance: repeated fresh sessions produce identical ordered simulation state.
   - Rollback: isolated ordering change.
2. Extract normalized per-tick gameplay input sampling/application without persistence.
   - Acceptance: ordinary controls behave identically; unit tests inject snapshots.
3. Add JSON recording for a direct surface-start Yavin session.
   - Acceptance: input count equals simulated ticks; corrupt files reject cleanly.
4. Add replay through the same input path.
   - Acceptance: live controls cannot change replay flight or weapons.
5. Add session validation and periodic checksums.
   - Acceptance: identical replay passes; deliberately altered input fails at the expected checkpoint.
6. Add replay tick/session metadata to telemetry and document the A/B workflow.
   - Completed: two renderer builds can produce comparable tick-aligned traces
     identified by the same recording fingerprint and verified replay status.

## H. First implementation recommendation

First make `updateAutonomous` iterate controller IDs in sorted order, retaining the existing controller and flight logic unchanged.

Files:

- `internal/game/game.go`
- `internal/game/game_test.go`

Tests:

- Construct several equivalent fresh sessions.
- Advance autonomous combat for a fixed number of ticks.
- Compare ordered object state, projectile IDs/owners, and mission-relevant results.
- Confirm the result is identical across repeated constructions.

Expected behaviour: no visible gameplay change, but autonomous firing and object insertion order become reproducible—the necessary deterministic foundation before recording inputs.
