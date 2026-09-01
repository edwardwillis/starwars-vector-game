# Star Wars Vector Arcade Homage — Go Build Plan

Non-commercial learning project. Homage to 1983 Atari arcade "Star Wars". Wireframe vector style, not photo-realism. All models hand-defined, no copied assets.

## Stack
- Language: Go
- Rendering: Ebiten (window, input, line drawing)
- Math: custom Vec3/Mat4 package, no 3D engine

## Render Pipeline (extensible, stage-based)

```
Scene (independent objects)
  -> Object (transform + styled model parts)
  -> Model (verts + edges + optional faces)
  -> Transform (object world matrix)
  -> Camera (view matrix)
  -> Backface Cull (optional, camera space)
  -> Project (perspective -> screen)
  -> Clip (near plane, screen bounds)
  -> Hidden-Line Resolve (optional, projected depth)
  -> Draw (line draw)
```

The renderer holds pipeline configuration and runs these operations in order.
Visibility processing is selected by a feature setting and defaults to drawing
all edges for the original arcade look.

Core types:

```go
type Vec3 struct{ X, Y, Z float64 }
type Edge struct{ A, B int } // vertex indices
type Face struct {
    Vertices []int // ordered vertex indices for one polygonal surface
}
type Model struct {
    Verts []Vec3
    Edges []Edge
    Faces []Face // optional until a visibility mode requires surfaces
}

type Part struct {
    Mesh      Model
    Color     color.RGBA
    LineWidth float32
}

type Object struct {
    Name   string
    Pose   Pose
    Motion Motion
    Parts  []Part
}

type VisibilityMode int

const (
    VisibilityAll VisibilityMode = iota
    VisibilityBackfaces
    VisibilityHiddenLines
)

type Pipeline struct {
    Visibility VisibilityMode
}
```

## Switchable Visibility and Hidden-Line Removal

“Optional stage” means the pipeline always has a defined visibility decision,
but its configured mode determines which implementation runs:

```go
switch pipeline.Visibility {
case VisibilityAll:
    // Bypass visibility processing and draw every clipped edge.
case VisibilityBackfaces:
    // Remove geometry belonging only to camera-facing-away surfaces.
case VisibilityHiddenLines:
    // Resolve each line against projected face depth and return visible pieces.
}
```

Switching this setting does not require game code, scene objects, controllers,
camera code, or networking to change. They continue submitting the same models
and poses to `Pipeline.Render`. Only the pipeline's conversion of geometry into
final line segments changes. That stable caller contract is what it means for
the architecture to support the feature cleanly.

The three modes provide different results:

- `VisibilityAll` ignores `Faces` and draws every edge. This is the default and
  preserves the transparent original-arcade wireframe style.
- `VisibilityBackfaces` uses camera-space face normals to omit surfaces facing
  away from the camera. It is inexpensive but cannot hide an edge behind a
  different front-facing surface.
- `VisibilityHiddenLines` performs a face-depth prepass, compares projected line
  depth against that surface depth, and splits partially occluded edges into
  visible line segments. This is full hidden-line removal.

The existing edge-only `Culler` hook is sufficient only for basic whole-edge
filtering and will be replaced by these visibility modes. Models can continue to
omit faces while using `VisibilityAll`; catalog entries must provide valid faces
before selecting either surface-aware mode. Face indices and winding are
validated with the rest of each model.

Hidden-line removal operates after projection and clipping because it compares
screen-space coverage, while retaining interpolated depth for every projected
endpoint. Its output is a list of drawable line segments rather than model
edges, since one edge may be visible in several separated pieces. The initial
implementation can use a small software depth buffer; later implementations may
use Ebitengine shaders without changing the visibility setting or caller API.

### Interactive Realism Profiles

The user can change rendering realism at runtime with a slider or equivalent
keyboard control. The slider uses discrete, named levels rather than blending
incompatible algorithms continuously. Moving it to the right cumulatively
enables more pipeline processing:

| Level | Name | Enabled behavior |
|---:|---|---|
| 0 | Arcade/X-ray | Draw every clipped wireframe edge |
| 1 | Facing surfaces | Add backface rejection |
| 2 | Solid objects | Add hidden-line removal within each object |
| 3 | Solid scene | Add depth occlusion between different objects |
| 4 | Depth cues | Add distance-based brightness and line-width attenuation |

Near-plane and screen clipping remain enabled at every level because they are
rendering correctness requirements rather than realism effects. The default is
level 0, preserving the transparent vector-arcade presentation.

```go
type RealismLevel int

const (
    RealismArcade RealismLevel = iota
    RealismFacingSurfaces
    RealismSolidObjects
    RealismSolidScene
    RealismDepthCues
)

type PipelineProfile struct {
    BackfaceCulling bool
    SelfOcclusion   bool
    SceneOcclusion  bool
    DepthCues       bool
}
```

`RealismLevel.Profile()` maps each slider position to a complete immutable
pipeline configuration. The UI changes only the selected level; it does not
directly wire individual rendering stages. This makes levels easy to test,
serialize in settings, synchronize for demonstrations, and extend later.

The renderer displays the active level name beside the slider and permits
switching while objects are moving. This makes the visual contribution and
performance cost of each stage directly observable. Advanced settings may
eventually expose individual stage toggles, but the ordered slider remains the
primary user-facing control.

## Scene and Object Scope

The renderer is object-agnostic. Every visible world item is an independently
transformable `Object` assembled from one or more styled wireframe `Part`s.
The same update and render path supports:

- player and enemy spacecraft with multiple colored details;
- short-lived projectiles such as laser bolts and impact bursts;
- fixed emplacements such as laser cannons and trench towers;
- large compound structures such as the Death Star and trench geometry;
- background objects such as stars, debris, and navigation markers.

Object behavior remains separate from appearance. Simulation systems own
position, velocity, lifetime, collision bounds, faction, and AI; scene objects
own transforms and renderable parts. Models can be shared by many instances so
enemy squadrons and repeated laser bolts do not duplicate geometry.

Catalog constructors assemble geometry, default vector styling, and multipart
details into ready-to-place scene objects. Game code selects catalog entries and
supplies transforms instead of knowing how each object is built.

## Destruction and Disintegration

Objects that are destroyed by a laser hit or a physical object-to-object
collision may disintegrate instead of disappearing immediately. Destructible
catalog entries define two or three reusable fragment meshes that together
preserve the recognizable outline of the intact object. A deterministic
fallback may partition an object's existing parts or edges when bespoke
fragments are not available.

When a destructive hit is resolved, the simulation atomically removes the
intact object and spawns two or three transient component objects at its pose.
Each component inherits the object's motion, receives a different outward
linear velocity and angular velocity, and spins away independently for up to
two seconds. Components have no controller, cannot fire, and do not participate
in physical collisions, but remain laser targets.

A laser hit on a component consumes the bolt and replaces that component with
its constituent polygon faces. Each polygon becomes an independently moving and
spinning final-stage shard with a fresh two-second lifetime, irrespective of how
long the parent component had left. Polygon shards are non-physical,
non-targetable visual debris and cannot disintegrate recursively. Catalog models
therefore retain explicit face topology in addition to vertices and edges.
Every disintegration piece preserves the parent's signed travel vector as its
dominant velocity component, with a deterministic random blast perturbation added
per piece so explosions spread naturally without breaking replay determinism.

Opposing laser projectiles may also intercept one another in flight using swept
segment collision tests. Intercepted bolts are both removed without affecting
their owners or spawning destruction debris; projectiles from the same owner do
not collide.

Physical collisions are resolved symmetrically: when two participating objects
collide, both objects disintegrate and each produces its own two or three
fragments. This includes collisions such as fighter against fighter. The
collision system resolves each object at most once per simulation tick so a
single pile-up cannot spawn duplicate fragment sets. Background-only items such
as starfield points and non-physical navigation markers do not participate in
collision detection. Transient components and polygon shards also do not
participate in physical collisions. Autonomous respawns are wave-based: pending
swarm replacements wait until no autonomous controllers remain, then respawn
together after their normal delay.

Respawn placement favors a safe point well away from the nearest surviving swarm
member, with the replacement oriented toward that member so it re-enters the
engagement rather than appearing inside an existing dogfight.

Fragment count, directions, speeds, and spin must be deterministic from stable
simulation data such as the destroyed object ID and impact tick. This keeps
local tests, replay, and the later authoritative server consistent. Networked
clients receive the destruction and fragment spawn/removal events rather than
generating authoritative debris locally. If a camera targets a destroyed
object, it follows an explicitly selected fragment when supported. Player
destruction uses a three-second external pullback view of the disintegration,
then follows a deterministic random surviving swarm fighter until respawn.

## Player Shields

The player fighter will have a shield strength that starts at 8, shown as eight
mirrored segments on each side. A laser-bolt
hit decrements the shield by 1, while a physical collision decrements it by 3.
The shield recharges by 1 point after 20 seconds without receiving damage,
without exceeding its maximum of 8. The player is destroyed when shield strength
falls below zero; reaching exactly zero leaves the fighter barely operational.
Damage resets the recharge timer, and a pending recharge is cancelled by any
subsequent hit.

In cockpit view, the current shield strength is displayed at the top center as a
compact arcade-style `SHIELD` indicator with eight mirrored triangular segments per side, following the visual
language of the original vector game. The shield state is simulation-owned and
must be included in future snapshots, replay events, and difficulty-profile
configuration even though the initial values are fixed.

## Object Coordinates and Kinematics

Directional catalog models use a shared local coordinate convention:

- `+Z` is forward (the front or nose of the object);
- `-Z` is backward;
- `+Y` is up;
- `+X` is right.

The current twin-panel fighter follows this convention: its cockpit window faces
`+Z`. Models that have no meaningful front, such as the Death Star or a static
piece of scenery, still use the convention for consistency but may never move.

An object's simulation state is represented independently from its render
matrix:

```go
type Pose struct {
    Position    Vec3
    Orientation Quaternion
}

type Motion struct {
    Speed     float64 // signed units per second along the local forward axis
    YawRate   float64 // rotation about local +Y
    PitchRate float64 // rotation about local +X
    RollRate  float64 // rotation about local +Z
}
```

Orientation is stored as a normalized quaternion to avoid Euler-angle gimbal
lock. Manual controls and controller intents remain intuitive yaw, pitch, and
roll values. Each fixed simulation tick applies angular rates to orientation,
derives the forward direction by rotating local `+Z`, and advances position by:

```text
position += forward * speed * tickDuration
```

The render transform is then derived from `Pose` as translation multiplied by
orientation. Geometry, rendering, and networking do not independently maintain
position or rotation. Server snapshots transmit pose and motion state, while
clients interpolate poses for display.

Positive speed moves an object front-first along local `+Z`; negative speed
moves it backward along local `-Z`; zero speed is stationary. Yaw and pitch
redirect the object's local axes and therefore either direction of travel. Roll
changes the object's up/right frame without directly changing its current
position. Static objects use zero speed and zero angular rates. Later flight
models may add acceleration, inertia, or strafing without changing the pose or
controller boundaries.

## Viewpoints and Camera Anchors

The camera will be independently attachable to any scene object or fixed world
anchor. A viewpoint consists of a target object or anchor ID plus a local
position and orientation relative to that target. This supports:

- an external chase or orbit view of any spacecraft;
- a cockpit view positioned behind a specific fighter's window;
- spectator views attached to autonomous or user-controlled objects;
- fixed or moving observation points on the Death Star, trench, or towers;
- switching viewpoints at runtime without transferring control of the object.

Catalog objects may expose named camera anchors such as `cockpit`, `chase`,
`turret`, or `surface-observer`. The camera system resolves the selected anchor
against the object's current world transform. View selection remains separate
from object ownership and input, allowing a user to observe one object while
controlling another.

## Cut Scenes and Orchestrated Set Pieces

The game will support deterministic cut scenes for movie-style introductions,
mission briefings, level transitions, victory sequences, and other authored set
pieces. A cut scene places predefined catalog objects, text, and camera
viewpoints on a timeline and assigns them scripted paths or actions. It uses the
normal object catalog and rendering pipeline, so fighters, laser bolts, Death
Star geometry, starfields, themes, and realism settings retain their established
appearance instead of requiring a separate animation system.

A cut-scene definition contains:

- a stable name, version, duration, and deterministic seed;
- the catalog objects to spawn and their initial poses and styles;
- position and orientation tracks, with explicit interpolation and easing;
- timed actions such as firing, formation changes, text cues, and object removal;
- a fixed world viewpoint or timed camera track with optional cuts between
  named object or world anchors;
- vector text cards, captions, placement, color, and display intervals;
- optional audio and transition cues when sound is implemented; and
- completion and skip behavior, including the next game state or level.

```go
type CutScene struct {
    Name       string
    Version    int
    Duration   float64
    Seed       uint64
    Actors     []ActorTrack
    Cameras    []CameraCue
    Text       []TextCue
    Events     []TimedEvent
    Transition Transition
}
```

Tracks are evaluated from absolute cut-scene time rather than accumulated frame
deltas, preventing drift and making playback reproducible in tests, replays,
and synchronized multiplayer clients. The fixed simulation tick advances the
timeline, while rendering may interpolate between evaluated poses. Camera and
actor interpolation must specify its coordinate space, easing rule, and behavior
at track boundaries.

During a cut scene, orchestration temporarily supplies authoritative actor poses
or scripted decisions. Normal controllers do not compete with those tracks.
Actors can be marked cinematic-only or handed into the live simulation at the
end of a transition with an explicit final pose, motion, controller, health, and
ownership state. Conversely, a transition can capture selected live objects as
actors without mutating unrelated world state.

Cut scenes are registered by stable identifier and selected by the active game
or level profile. Definitions should be data-driven once the schema stabilizes,
allowing contributors to create introductions and transitions without changing
the game loop. Initial definitions may use validated Go values; versioned JSON
or YAML loading follows the same policy as game profiles.

Playback includes a clear skip input. Skipping applies the declared transition
atomically rather than fast-forwarding every visual event. Interactive gameplay
input, collision damage, autonomous firing, and respawn processing are disabled
unless a cut-scene definition explicitly enables them. Text and camera framing
must respect the logical viewport and remain independent of operating-system
window size.

## Live Simulation and Multiplayer Server

A later networked mode will use an authoritative Go server that owns the live
world model. The server advances simulation ticks and tracks stable object IDs,
transforms, velocity, behavior, ownership, health, projectile lifetime, and
other gameplay state. Objects may be controlled by users, server-side AI, or
autonomous scripted behavior.

Clients will:

- connect as players or spectators;
- receive an initial world/catalog manifest and periodic state snapshots;
- send timestamped control intentions rather than authoritative transforms;
- render remote motion using snapshot buffering and interpolation;
- select any permitted object or world anchor as their viewpoint;
- receive object spawn, update, ownership, and removal events.

The first network implementation should favor clarity over scale: a single
authoritative server process, fixed simulation tick rate, WebSocket transport,
compact versioned messages, and in-memory sessions. Later work may add client
prediction, reconciliation, persistence, authentication, multiple rooms, replay
recording, and horizontal scaling.

The simulation core should remain independent of Ebitengine and networking so
the same deterministic update logic can run on the server, in local single-player
mode, and in tests. The current local game remains the first client and can use
an in-process simulation before a network transport is introduced.

## Customization and Extension Architecture

The next architectural milestone is to turn the existing extension hooks into
a coherent customization API before adding major new object classes, network
transport, or external agents. Contributors should be able to add a behavior,
object definition, rendering profile, or complete game profile without editing
the central game loop. Built-in features use the same registration and
configuration paths offered to contributors so extension points remain tested
by normal gameplay.

Customization is divided into five stable layers:

| Layer | Responsibility | Primary extension mechanism |
|---|---|---|
| Game profile | Selects and tunes the overall experience | Named, versioned, validated configuration |
| Controller | Decides how one object behaves | Strategy factory registered by stable name |
| Object catalog | Defines appearance, anchors, capabilities, and destruction | Object-definition factory registered by stable name |
| Rendering | Selects optional visibility and presentation processing | Named pipeline profile assembled from rendering stages |
| Cinematic | Orchestrates actors, cameras, text, and transitions | Named cut-scene definition with validated timeline tracks |

### Unified Game Profiles

Values that currently live in game constants and assembly functions will move
into one immutable root configuration. This includes manual flight limits,
world-motion scale, swarm count and placement, controller selection, pursuit
tuning, aim error, weapon behavior, shields, collision and respawn rules,
difficulty, and the selected rendering profile.

```go
type GameProfile struct {
    Name       string
    Version    int
    Simulation SimulationConfig
    Player     PlayerConfig
    Swarm      SwarmConfig
    Combat     CombatConfig
    Difficulty DifficultyConfig
    Rendering  string
    Cinematics CinematicConfig
}
```

Profiles are validated before a session starts, treated as immutable during a
run unless a setting is explicitly runtime-switchable, and recorded with the
session seed for replay and multiplayer agreement. Initial built-in profiles
remain Go values for type safety and straightforward testing. Versioned JSON or
YAML loading can be added after the schema stabilizes. A custom profile should
be sufficient to change game tempo and feature selection without modifying
`game.go`.

Difficulty presets are curated overlays on a complete game profile rather than
a second independent configuration system. Applying `Cadet`, `Pilot`, `Ace`, or
`Nightmare` produces a fully resolved and validated profile before the world is
created.

### Registries and Factories

Controllers, catalog objects, rendering profiles, and cut scenes are selected
through registries keyed by stable, namespaced identifiers such as
`builtin/pursuit`, `builtin/twin-panel-fighter`, `builtin/arcade`, and
`builtin/opening-flyby`. Registry entries contain factories or immutable
definitions plus configuration validation, not shared mutable instances.
Per-object controller state and pseudo-random state remain isolated on the
object instance.

```go
type ControllerFactory func(seed uint64, config Config) (Controller, error)

type ObjectDefinition struct {
    Create            ObjectFactory
    CreateFragments   FragmentFactory
    DefaultController string
}
```

An object definition owns the knowledge needed to construct its intact form,
destruction components, and final polygon shards. The simulation asks the
definition for fragments instead of calling fighter-specific catalog functions.
This permits a new spacecraft or Death Star component to participate in the
generic spawn, collision, rendering, and destruction systems without a type
switch in the game loop.

Catalog styling will be parameterized independently from geometry. A style or
theme selects colors, line widths, and faction presentation while the same
model, anchors, collision properties, and destruction topology are reused.

### Composable Rendering Profiles

The current edge-level `Culler` is an interim hook. It can remove complete
edges, but it cannot implement partial hidden-line resolution, scene-wide
occlusion, or depth cues. Rendering will evolve toward ordered stages with a
stable input/output contract. Mandatory correctness stages such as camera
transformation, near-plane clipping, projection, and screen clipping always
run. Optional stages are selected by a named profile.

```go
type RenderStage interface {
    Apply(RenderContext, Geometry) Geometry
}

type RenderingProfile struct {
    Name   string
    Stages []RenderStageFactory
}
```

The realism slider selects the built-in cumulative profiles described above;
it does not manipulate renderer internals directly. Contributors may register
alternative visibility resolvers or presentation stages such as vector glow,
provided they preserve the stage contract and declare whether they operate per
object or across the complete scene.

### Simulation Boundary

The Ebitengine `Game` will become an adapter for input, audio, window lifecycle,
and drawing. Renderer-independent world state and fixed-tick rules move into a
simulation package that owns objects, controller decisions, integration,
weapons, collisions, damage, destruction, and spawning. This is required both
for modular customization and for the later authoritative server.

The simulation depends on controller and catalog interfaces, never on concrete
built-in implementations. Rendering consumes read-only world snapshots and
does not own gameplay state. User input, local rule-driven controllers, and
remote agents all produce the same validated decision type.

### Public and External Extensions

Packages under Go's `internal/` convention are suitable while APIs are still
changing and allow contributors working inside this repository to add built-in
extensions. Once the controller, configuration, catalog, and snapshot contracts
stabilize, the minimal extension-facing types will move to importable packages
so separately maintained Go modules can compile against them.

The first plugin mechanism will use normal Go imports and compile-time
registration. Go's native dynamic-plugin mechanism is not the default because
of its platform and exact-build compatibility constraints. Runtime and
language-independent extensions use the later versioned external-agent
protocol, including MCP adapters, and remain out of process behind simulation
validation, timeouts, and deterministic fallbacks.

Every extension point requires:

- a stable identifier and validated configuration;
- deterministic behavior for a fixed seed where applicable;
- focused contract tests plus at least one registry/assembly test;
- no direct mutation of authoritative state outside the simulation API; and
- documented compatibility and fallback behavior.

## Pluggable Intelligence and Control

Every controllable object may be assigned an interchangeable controller plugin.
Controllers observe a restricted snapshot of world state and produce control
intentions such as thrust, rotation, aim, fire, or idle. They never mutate world
state or submit authoritative transforms directly; the simulation validates and
applies their intentions through the same movement and gameplay rules.

Initial controller strategies include:

- `Static`: produces no movement, suitable for the Death Star, scenery, and
  fixed emplacements that do not currently need targeting behavior;
- `Manual`: consumes the latest validated input from a human player's client;
- `RuleDriven`: runs deterministic local rules for patrol, pursuit, evasion,
  targeting, formations, and other simple autonomous behavior;
- `Agent`: delegates higher-level decisions to an external AI service, possibly
  through MCP, while retaining server-side authority and safety limits.

The current rule-driven fighter strategy includes independently seeded attack
runs against a designated target. Each run randomly selects its delay, duration,
arc radius, orbit direction, and firing cadence. Independent schedules may
overlap, allowing two or more swarm members to attack together without requiring
a centrally scripted formation. Controllers request fire through an optional
capability; the simulation remains responsible for spawning owned projectiles,
resolving hits, and applying destruction. Autonomous shots include deterministic
per-volley lateral and vertical aim error, preserving replayability while giving
the designated target an opportunity to evade.

If the designated player target is temporarily absent after destruction, swarm
controllers continue to run their avoidance and motion updates against a neutral
target; autonomous firing remains disabled until the player respawns.

## Curated Difficulty Profiles

Difficulty will be selectable rather than exposing dozens of independent tuning
sliders. A profile is a named, versioned bundle applied when a session starts
and recorded with the run for replay and multiplayer agreement. Profiles curate
the parameters that most affect pressure:

- swarm size and respawn policy;
- fighter minimum/maximum speed and acceleration;
- attack delay, attack duration, arc-radius range, and volley cadence;
- autonomous aim-error radius and target lead time;
- collision-avoidance strength and reaction horizon; and
- player respawn clearance and invulnerability grace period.

The initial presets should be `Cadet` (few, slower fighters with wide aim error),
`Pilot` (the balanced default), `Ace` (faster attacks and tighter aim), and an
optional `Nightmare` profile (full swarm pressure with minimal recovery time).
Each profile must remain deterministic for a fixed seed, and all values should
be validated at load time. A later settings screen can select a profile before
starting a game; the authoritative server will reject mismatched profiles in a
multiplayer session.

The core controller boundary is intentionally small. The current
`Strategy.Step(Context) Motion` and follow-up `Attacker.AttackIntent()` hooks are
an intermediate implementation; the customization milestone replaces them with
one atomic decision so every controller passes through common flight and weapon
validation:

```go
type Controller interface {
    Decide(Context) Decision
}

type Intent struct {
    Throttle float64
    Yaw      float64
    Pitch    float64
    Roll     float64
    Stop     bool
}

type Decision struct {
    Flight Intent
    Aim    Vec3
    Fire   bool
}
```

Controllers are selected through a registry by stable strategy name and
configuration. This allows new strategies to be added without changing object,
physics, networking, or rendering code. Controller state belongs to each object
instance, while reusable controller factories and configuration schemas belong
to the plugin registry.

External AI/MCP controllers require an asynchronous adapter because network
responses cannot block the fixed simulation tick. The adapter uses deadlines,
rate limits, bounded context, validated output, and a deterministic fallback
intent when the service is unavailable or late. Only explicitly permitted world
state is exposed, and external agents cannot bypass ownership, collision,
movement, weapon, or server authorization rules.

Controller decisions can be recorded alongside simulation ticks for debugging,
replay, evaluation, and comparisons between strategies. Local deterministic
controllers remain the baseline for tests and offline play.

## Build Steps

1. Go basics refresher — structs, slices, methods, goroutines
2. Ebiten hello world — window, draw single line
3. Math package — Vec3, Mat4, rotate/translate/scale, perspective projection
4. Static wireframe cube — validate pipeline end to end
5. First fighter model — hardcoded original verts/edges, render wireframe
6. Scene objects — transforms, multipart styling, multiple object instances
7. Kinematics — pose, quaternion orientation, signed axial speed, yaw/pitch/roll
8. Input — keyboard/mouse intent, dead zone, reticle, throttle, yaw/pitch/roll
9. Camera system — stable object IDs, fixed/chase/cockpit/orbit views, anchors
10. Object catalog — fireable laser bolt with muzzle anchors, spin, and lifetime
11. Rendering topology preparation — model faces, clipping, and interim culling hook
12. Starfield — deterministic world points, wrapping, projection, motion reference
13. Cockpit targeting — pointer aim, right-button steering, firing cone, converging bolts
14. Dogfight — multiple catalog fighter instances, deterministic pursuit/wander/excursion, variable-radius overlapping attack runs, autonomous targeting and fire, variable-speed and predictive-avoidance heuristics, symmetric collisions, two-stage component-to-polygon disintegration, player and swarm respawn lifecycle
15. Unified customization profiles — extract game constants and tuning into immutable, versioned, validated `GameProfile` values; add curated difficulty overlays
16. Controller contract and registry — atomic validated decisions, named factories, per-instance state, static/manual/pursuit built-ins
17. Object-definition registry — generic construction, styles, anchors, capabilities, fragments, and polygon shards without fighter-specific game logic
18. Composable rendering profiles — replace the interim culler with optional backface, hidden-line, scene-occlusion, and depth-cue stages plus the interactive realism selector
19. Simulation extraction — stable IDs and renderer-independent fixed-tick world updates behind snapshot and command APIs
20. Death Star — reusable registered surface modules, trench, towers, targeting reticle
21. Generalized camera anchors — cockpit, chase, spectator, and Death Star viewpoints selected independently from control ownership
22. Cut-scene orchestration — registered actor, path, camera, text, event, skip, and level-transition timelines
23. Authoritative server — fixed ticks, autonomous objects, sessions, profiles, snapshots
24. Rule-driven intelligence library — patrol, pursuit, evasion, targeting, and formations registered through the controller API
25. Multiplayer client — control input, interpolation, ownership, view switching
26. External agent adapter — asynchronous AI/MCP decisions and safe fallback
27. Score, complete game states, difficulty-selection UI, and sound
28. Public extension API — stabilize importable controller, catalog, configuration, snapshot, and cinematic contracts
29. Stretch: prediction/reconciliation, replay, persistence, multiple rooms
30. Stretch: controller evaluation, tournaments, and strategy hot-loading
31. Stretch: CRT/vector-glow stages and fixed-point math (period-accurate)

## Notes
- Step 5 is first visually demonstrable milestone (target early win).
- Keep each step in its own commit/branch for incremental review.
- Visibility defaults to drawing every edge; backface and hidden-line modes
  arrive through the rendering-profile work in step 18 and remain
  runtime-switchable features.
- Steps 15–19 are the modularity checkpoint. Major new world content and
  networking build on those contracts rather than adding more special cases to
  the Ebitengine game adapter.
