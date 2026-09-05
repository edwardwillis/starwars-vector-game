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

Static environment collision is asymmetric: an indestructible surface or
structure survives while the impacting fighter or projectile receives the
configured response. Collision participation, targetability, damageability,
and destructibility are independent capabilities. Collision resolution returns
the earliest time of impact, contact point, surface normal, and environment
feature ID so the simulation can place the moving object outside the collider,
apply damage once, and avoid repeated damage from persistent overlap.

Fragment count, directions, speeds, and spin must be deterministic from stable
simulation data such as the destroyed object ID and impact tick. This keeps
local tests, replay, and the later authoritative server consistent. Networked
clients receive the destruction and fragment spawn/removal events rather than
generating authoritative debris locally. If a camera targets a destroyed
object, it follows an explicitly selected fragment when supported. Player
destruction uses a three-second external pullback view of the disintegration,
then follows a deterministic random surviving swarm fighter until respawn.

## Object Shields

The initially controlled fighter will have a shield strength that starts at 8, shown as eight
mirrored segments on each side. A laser-bolt
hit decrements the shield by 1, while a physical collision decrements it by 3.
The shield recharges by 1 point after 20 seconds without receiving damage,
without exceeding its maximum of 8. The player is destroyed when shield strength
falls below zero; reaching exactly zero leaves the fighter barely operational.
Damage resets the recharge timer, and a pending recharge is cancelled by any
subsequent hit.

Shield strength and recharge state belong to the controlled world object, not
to a global player singleton. Each shield-capable spacecraft has independent
state so multiple players can be damaged, destroyed, recharged, and respawned
concurrently. In cockpit view, the locally observed object's current shield
strength is displayed at the top center as a
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

The current TIE fighter follows this convention: its cockpit window faces
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

### Large-object exterior-to-local-environment transitions

The orbital/surface/orbital flow is a reusable large-object capability, not a
Death Star-specific game mode. A large-object definition may expose named local
environments, entry and exit volumes, transition anchors, coordinate frames,
and representation policies. The Death Star is the first application; future
capital ships and stations can use the same mechanism for hull surfaces,
hangars, trenches, bridges, reactor spaces, and other close-flight areas. The
generic flow is `exterior -> transition -> local environment -> transition ->
exterior`.

Step 20 brings forward the smallest cut-scene subset needed to reproduce this
change of scale. The Death Star has two coordinated registered representations
rather than one uniformly detailed sphere:

1. An exterior arcade representation for far and approach views. This is one
   normalized, camera-facing 2D vector drawing containing the circular outline,
   symbolic equatorial stripe, and superlaser dish. Distance changes its
   projected scale rather than swapping it for a different drawing. Stable
   optional lines and dots are progressively revealed as it approaches, giving
   the original game's apparently random increase in surface detail without
   frame-to-frame flicker. The stripe is a visual identifier only and does not
   imply that the finite local trench encircles the station.
2. A local surface/trench environment for close flight, using a tangent-space
   coordinate frame and reusable tiles containing panels, towers, cannon
   emplacements, trench walls, and floor geometry. Only nearby and potentially
   visible tiles are selected for rendering; distance tiers reduce line density
   toward the horizon.

The logical Death Star remains an ordinary authoritative world object even when
its exterior appearance is a billboard. It retains stable identity, host pose,
radius, collision volume, targeting anchor, transition anchors, and attached
environments. The billboard is client presentation and must never become a HUD
overlay or substitute for simulation state.

Exterior appearance is profile-selectable. Register at least
`builtin/death-star-arcade-billboard` and
`builtin/death-star-orbital-wireframe` presentations against the same logical
object definition. The arcade presentation is the default; retain the existing
sparse 3D sphere-and-dish representation as a supported alternative rather than
dead code. The generic presentation mechanism must remain suitable for 3D
orbital models such as Imperial Star Destroyers.

The arcade drawing uses normalized 2D vector coordinates and a generic
world-anchored billboard renderer: project the Death Star centre, derive its
screen radius from world radius and depth, then scale the same drawing about
that point facing each client's active camera. Base silhouette lines are always
visible. Solid billboard presentations may request a filled occlusion silhouette
behind their vector lines so background stars do not show through the object;
the mask is rendered as a presentation pass before nearby world geometry. Each
optional detail primitive receives a deterministic seed-derived
reveal threshold; projected size/proximity progressively reveals more lines and
dots. Apply hysteresis or stable thresholding so details do not shimmer at a
boundary. Detail selection affects presentation only, not collision, targeting,
network state, or the local surface layout.

The local environment is fully flyable. Entry places the fighter above ordinary
surface rather than inside the trench. Fighters may continue indefinitely over
the surface, attack towers and cannon installations, locate the trench as a
specific landmark, descend through its open top, and fly along its axis. Adopt one explicit
local convention—for example `+Z` along the trench, `+X` across it, and `+Y`
outward/up from the Death Star surface. Outside the trench the surface deck is
at `Y=0`; the trench has finite side walls and a recessed bottom at negative
`Y`. The trench is a finite surface feature, not an equatorial channel and not
a repeating property of every tile. It terminates at a closed end containing a
small addressable exhaust port in the floor. Entry into the trench is ordinary
continuous flight through the open top, not a scripted snap into a corridor.

Near-surface flight should appear unbounded at gameplay scale. Generate a
deterministic two-dimensional neighborhood of tangent-space surface tiles around
each relevant fighter and discard distant render/collision tiles. Tile identity
comes from stable integer coordinates so revisiting an area reproduces the same
layout and preserves authoritative installation/destruction state. This first
representation may use a locally planar tangent patch; later curvature or
host-surface remapping must preserve the same environment and tile contracts.

Rendered tiles and collision tiles are generated from the same validated
module definitions so visible floors, trench sides, trench bottoms, towers,
cannon emplacements, antennae, and block structures have matching collision
geometry. Use a small set of reusable collider primitives: swept fighter
spheres against finite planes/rectangles for decks and trench walls, and
oriented boxes or other simple convex bounds for structures. Projectiles use
the same earliest-time-of-impact query. Broad-phase tile bounds reject distant
features before narrow-phase tests; collision geometry is independent of visual
LOD, so a hidden far-detail feature cannot become non-physical accidentally.

If a shielded fighter survives contact, resolve it to the contact boundary and
apply a deterministic deflection, slide, or stop response plus a short contact
grace interval. Do not leave it embedded where the next fixed tick repeats the
same collision damage. At full impact damage, use the normal disintegration
lifecycle with inherited trajectory and the contact normal influencing debris
spread.

Crossing a configurable surface-distance threshold starts a deterministic
two-second approach cut scene, unless a mission explicitly suppresses or
overrides it. Suspend player steering and firing, move the craft along the
declared approach path, and perform one deliberate 180-degree local-axis roll
while aligning into the local surface-flight orientation as the camera closes on
a named surface anchor. At the declared final authoritative tick, atomically transfer the craft
into the local surface environment with its declared pose and motion and resume
control above ordinary surface. This transition preserves player identity,
ownership, speed, shields, score, and other gameplay state. It changes
representation and coordinate frame, not the logical craft. Skipping applies
the exact same final state immediately.

Local-environment entry is per fighter, not a global scene switch. The
authoritative world assigns each object a spatial zone/frame identifier such as
`exterior` or a specific host environment. In multiplayer, one fighter may be
inside a trench or hangar while another remains outside; each client receives
and renders the relevant zone plus transition events. The cut scene is client
presentation, while the zone/frame transfer occurs at an explicit authoritative
tick.

Leaving a configured exit volume performs the inverse mapping and returns the
fighter to the exterior frame with continuous orientation and velocity.
For surface environments, the default exit volume is an altitude band: climbing
above its configurable upper boundary is an explicit, intuitive return-to-space
control. The same contract can later expose hangar doors, trench ends, or other
named exits without changing the transfer mechanism.
Transfers are bidirectional and idempotent: an object cannot occupy both frames
or trigger duplicate entry/exit events. Each event records the host object ID,
environment ID, transition anchor, source and destination frames, and
authoritative tick. Spherical hosts may use a spherical/tangent mapping; other
hosts supply an ordinary local-to-host transform.

Local environments are anchored to the host object's pose. For a static body
this is a fixed transform; for a moving or rotating capital ship, composing the
environment frame with the host pose keeps hangars and hull environments
attached without changing the transition contract. The initial implementation
may support static hosts, but the data model must not assume that every host is
spherical, planetary, or immobile.

The initial transition implementation needs only a fixed camera/actor path, a
two-second duration, deliberate half-turn roll, input suppression, skip action,
and final-state transfer. The general registered cut-scene
timeline, text, branching events, and level-transition system remain in the
later cut-scene step. Orbital and surface scenes use ordinary catalog objects,
camera transforms, targeting, controllers, and rendering profiles; neither the
renderer nor HUD may branch on a Death Star type.

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

### First-class player identity and ownership

Multiple players are a core world-model requirement, not a later duplication of
the current single-player adapter. Introduce a stable `PlayerID` (or the more
general `ParticipantID`) distinct from `scene.ObjectID`. A participant record
identifies its controlled object, team/faction, connection/ready state, score,
respawn state, and permissions. Damageable state such as shields belongs to the
controlled object; camera selection and HUD preferences remain client-local.

The authoritative simulation must not depend on a global `fighterID`, singular
`Player` profile, global shield/destroyed flags, or controller-map membership to
infer whether an object is a player, swarm member, or static object. Maintain
explicit ownership and role metadata instead. One participant may control one
object initially, but the model must allow control transfer, spectators, and a
participant selecting a viewpoint different from its controlled object.

Input commands carry participant identity, target object ID, client sequence,
and intended simulation tick. The server authenticates the participant, checks
ownership/control permission, and applies validated intent; clients never send
authoritative transforms. Projectiles retain their firing object ID, with score
credit resolved through ownership at the authoritative event tick. Object
spawn/removal, damage, destruction, respawn, and scoring events identify all
affected stable IDs explicitly.

Local single-player mode is the one-participant case of the same session model.
Step 20 and later object work must not add new singleton-player assumptions.
Before the authoritative-server step, migrate existing `fighterID`, shield,
destruction, respawn, targeting, and controller-role state into participant- and
object-keyed structures with headless two-player tests.

## Customization and Extension Architecture

The next architectural milestone is to turn the existing extension hooks into
a coherent customization API before adding major new object classes, network
transport, or external agents. Contributors should be able to add a behavior,
object definition, rendering profile, or complete game profile without editing
the central game loop. Built-in features use the same registration and
configuration paths offered to contributors so extension points remain tested
by normal gameplay.

Customization is divided into six stable layers:

| Layer | Responsibility | Primary extension mechanism |
|---|---|---|
| Game profile | Selects and tunes the overall experience | Named, versioned, validated configuration |
| Controller | Decides how one object behaves | Strategy factory registered by stable name |
| Object catalog | Defines identity, anchors, capabilities, collision, and destruction | Object-definition factory registered by stable name |
| Appearance | Selects a logical object's 3D, billboard, or other visual representation | Named presentation registered independently from the object definition |
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

Initial implementation status: complete. The four built-in profiles resolve to
validated Go values; `Pilot` preserves the established gameplay, the command
line selects a profile at launch, and `Game.NewWithProfile` clones the resolved
configuration before creating the session. External JSON/YAML loading and an
in-game selection screen remain later work after the schema has seen further
use.

### Registries and Factories

Controllers, catalog objects, appearances, rendering profiles, and cut scenes
are selected through registries keyed by stable, namespaced identifiers such as
`builtin/pursuit`, `builtin/tie-fighter`, `builtin/arcade`, and
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

Logical object definitions and appearances are independently selectable. A
single Death Star object definition may therefore use an arcade billboard or a
sparse orbital wireframe without duplicating identity, collision, targeting,
environment attachment, or multiplayer state. Presentation definitions declare
their geometry kind (`model-3d`, `vector-billboard`, or a later extension),
immutable detail layers, projected-size rules, and styling. Built-in appearances
use the same registry available to contributors; neither the game loop nor the
renderer switches on a Death Star object type.

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

### Large-object detail selection

Fixed large objects may provide immutable `far`, `medium`, and `near` geometry
or presentation layers. The renderer selects among them using configurable projected
screen size thresholds, rather than raw world distance, so selection accounts
for object radius, camera field of view, zoom, and viewport size. The Death Star
will be the first user, but its default arcade appearance uses one scalable
billboard drawing with stable optional detail primitives rather than discrete
replacement models. Its base outline, stripe, and dish remain visible while
additional seeded lines and dots reveal progressively with projected size.
Towers, cannons, panels, and the finite physical trench are rendered only by the
local near-surface environment rather than exterior whole-object LOD.

Thresholds belong to the object definition or resolved display profile and are
validated when the session is created. Use separate enter/leave thresholds (or
a small hysteresis band) to prevent detail flicker when a viewpoint sits near a
boundary. Detail selection is renderer-owned and must not change simulation,
collision, targeting, ownership, or network snapshot state. All representations
are generated or cached once; crossing a threshold selects existing geometry
rather than rebuilding it during a frame. Objects without detail variants
continue through the current rendering path unchanged.

For the Death Star, whole-object LOD applies only to orbital viewing. Close
surface gameplay transitions to a separate local tiled representation rather
than selecting an impractically dense whole-sphere mesh.

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

Initial implementation status: complete. The control package now exposes the
atomic `Decision` contract, shared `Limits` application, and a registry with
`Static`, `Manual`, and `Pursuit` built-ins. The game accepts a caller-supplied
registry through `NewWithProfileAndRegistry`; the legacy `Step` and
`AttackIntent` methods remain as compatibility helpers while contributors move
to `Decide`.

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
5. First fighter model — hardcoded original TIE-style verts/edges, render wireframe
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
19. Simulation extraction — stable IDs and renderer-independent fixed-tick world updates behind snapshot and command APIs; move gameplay tests into this headless layer wherever possible
20. Death Star and reusable large-object environments — canonical TIE naming; generic bidirectional exterior/local-frame transitions; default scalable arcade billboard with deterministic proximity-revealed detail; optional sparse 3D orbital presentation; configurable approach threshold; per-fighter two-second roll transition; fully flyable tiled tangent-space surface with a finite trench, matched collision floors, walls, structures, towers, cannons, panels, targeting, and distance detail
21. Generalized camera anchors — cockpit, chase, spectator, and Death Star viewpoints selected independently from control ownership
22. General cut-scene orchestration — expand the Step 20 approach-transition subset into registered actor, path, camera, text, event, skip, and level-transition timelines
23. Authoritative server — first-class participant IDs, ownership and control authorization, fixed ticks, autonomous objects, sessions, profiles, snapshots, and headless multi-player tests
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
- Later geometry refinement: consider extruding thin fighter wing panels to a
  small finite depth, with front/back/side faces and consistent winding. This
  would make back-face and hidden-line culling reliable from every orientation,
  at the cost of additional geometry and updated fragment/shard definitions.
- Steps 15–19 are the modularity checkpoint. Major new world content and
  networking build on those contracts rather than adding more special cases to
  the Ebitengine game adapter.
- Headless testing is a first-class requirement: renderer-independent simulation,
  catalog, controller, and profile tests should run without a display. Ebiten
  integration tests may use an `integration` build tag and run under Xvfb (for
  example, `xvfb-run -a go test -tags=integration ./...`) in local development
  and CI environments without physical graphics hardware.
- Vector-rendering guidance: retain homogeneous transforms, perspective
  projection, near/screen line clipping, back-face detection for closed solids,
  and vector-adapted depth ordering/hidden-line removal. Treat classic
  scan-line, area-subdivision, and raster-oriented algorithms as reference
  material rather than direct implementation targets; do not turn the project
  into a pixel/raster renderer. A lightweight vector depth buffer or segment
  splitting may be used when hidden-line removal advances, while BSP/octree
  structures are reserved for larger static environments or spatial indexing.
- Geometry prerequisite for visibility: important thin shapes such as fighter
  panels should eventually be modeled as very thin extruded solids with
  consistent face winding. This supplies the planes, normals, and depth needed
  by vector back-face and hidden-line algorithms while preserving the classic
  outline aesthetic.
- Reference text: Foley, van Dam, Feiner, and Hughes, *Computer Graphics:
  Principles and Practice*, 2nd ed. (Addison-Wesley, 1990), especially the
  chapters on transformations, clipping, visible-surface determination,
  z-buffering, list-priority methods, spatial subdivision, and animation.
  Use it for algorithmic foundations while selecting vector-appropriate
  adaptations rather than copying raster display pipelines.

Implementation status: step 17 is complete. Catalog definitions now have stable
names and lifecycle factories for intact objects, component fragments, and
polygon shards. Games accept an injectable object registry, and profiles select
player and swarm definitions independently of the game loop. Built-in fighter
and laser-bolt definitions remain available as defaults while custom aliases
and future object classes can be registered by contributors.

Step 18 implementation is complete. The renderer now exposes composable stages
and built-in progressive profiles (`arcade`, `culled`, `hidden-line`, and
`depth-cue`). Profiles are selected from `Display.RenderingProfile`; legacy
culler support remains compatible while back-face culling and later visibility
stages can evolve independently.

Step 19 implementation is complete. `internal/sim` provides a validated,
deterministically ordered world, fixed-tick kinematic stepping, immutable-style
snapshots, and validated add/remove/motion commands. The Ebitengine adapter now
uses that world as the authoritative motion boundary and exposes snapshots and
command application without coupling callers to rendering. Collision, combat,
controller, and respawn policies remain adapter-owned systems that can be moved
behind the same command boundary incrementally.

Before Step 20 geometry work, correct the original fighter naming. The current
`TwinPanelFighter` is the project's TIE-style Imperial fighter and will become
the canonical `TIEFighter` model and `builtin/tie-fighter` catalog definition.
Remove the old `builtin/twin-panel-fighter` identifier completely and migrate
profiles, tests, documentation, and all call sites as one atomic rename; the
project is early enough that a compatibility alias would add needless cruft.
The distinct Rebel X-Wing is now implemented as `XWing` and registered as
`builtin/x-wing`, with its own geometry, anchors, collision bounds, fragments,
and style. The player profile uses it while the swarm remains
`builtin/tie-fighter`.
The game also includes a presentation-only fighter showcase toggled with `C`;
it instantiates registered fighter definitions and rotates them independently
of gameplay, providing a foundation for a later faction/fighter selection
screen.
Do not use a generic `fighter` type switch: player and swarm roles must continue
to select independently registered object definitions so either faction's craft
can occupy either role.

Step 20 exterior prototype is ready for replacement. The fighter has been
canonically renamed to `TIEFighter`/`builtin/tie-fighter` with no legacy alias.
Profiles can place arbitrary registered world objects, and scene objects now
separate targetability from hit/destruction behavior and carry renderer-only
detail tiers. Generic model transform/merge and spherical-placement helpers
support reusable surface modules. The current registered orbital
`builtin/death-star` uses only a sparse spherical body and recessed
upper-hemisphere superlaser dish; retain it as the optional
`builtin/death-star-orbital-wireframe` appearance. Replace its default exterior
presentation with one scalable 2D arcade drawing whose seeded lines and dots are
progressively revealed by projected size. All other physical surface geometry
belongs exclusively to the local environment. Projected-size detail selection
uses hysteresis, and the cockpit HUD can mark any
targetable object intersected by its aim ray. Visual density, scale, colors, and
thresholds remain tuning items after interactive inspection.

The first scale refinement models the Death Star at radius 300 world units and
places its centre 400 units ahead, leaving the approaching fighters roughly 100
units from the near surface. The current sparse sphere-and-dish geometry remains
available as an alternate presentation. Towers, cannon emplacements, panels,
trench geometry, and their collision state are generated by nearby
local-environment tiles rather than retained in either exterior appearance.

Step 20 local-environment implementation is in progress. The first vertical
slice adds host-relative spatial frames to the authoritative simulation,
host-specific environment instances, pose-preserving frame transfers, and
transition events carrying object, host, environment, anchor, frame, and tick
identity. `internal/environment` now owns registered large-object environment
definitions and generated tiles; the initial Death Star tangent frame maps
local `+Y` away from the near surface and local `+Z` along the trench. Generated
surface tiles provide sparse vector decks and addressable tower/cannon features.
Nearby deterministic tiles are generated in two dimensions around
local craft, producing effectively unbounded surface flight without retaining
the whole surface. A finite four-tile trench run replaces the surface only at
its declared coordinates and provides an open top, recessed floor, side walls,
closed ends, and an addressable exhaust port in the terminal floor. Local entry
starts above ordinary surface with the trench nearby rather than placing the
fighter inside it.

Local collision uses swept fighter/projectile spheres against finite planes
and oriented boxes generated alongside the visible tile geometry. Surviving
fighter contacts resolve outside the collider, deflect and slow the craft, and
receive a short contact grace interval; lethal contacts reuse the normal shield
and disintegration flow. Rendering, targeting, combat projectiles, debris,
solid collisions, and autonomous-controller context are frame-aware, so objects
in different local environments cannot interact merely because their local
coordinates overlap. Environment frame IDs include the concrete host object ID,
allowing multiple instances and per-fighter zone membership.

The appearance registry and default arcade billboard are implemented. The
approach-volume crossing now starts a per-fighter deterministic transition: the
craft is held out of normal input and simulation, the camera follows it in an
external chase view, its world pose eases toward the declared local entry pose,
and orientation uses smooth alignment plus one deliberate 180-degree local-axis
roll. During the roll, the presentation also renders a compact host-transformed
preview of the destination surface, so the approach visibly closes onto the
Death Star rather than revealing the local environment only after the cut scene.
The same transition path is used by autonomous fighters, allowing swarm
members to follow the player into surface flight without controller-specific
or renderer-specific branches. At the two-second endpoint (or on Escape) the authoritative frame transfer
occurs and the prior camera mode, motion, and gameplay state are restored. The
Pursuit intent is a simulation concern separate from frame membership: an
actively pursuing fighter may be scheduled to follow its target through either
direction of an exterior/local transition, and resume pursuit after the
authoritative transfer. This must work for surface entry and surface exit and
must not make unrelated swarm members cross environments automatically.
The controller API now exposes this capability through the optional
`PursuitFollower` interface; the game uses it to propagate player entry
transitions to eligible swarm members while preserving frame isolation for all
other controllers.
Non-pursuit autonomous objects now receive a generic proximity-and-heading
commitment path: when travelling toward a transitionable host within the
configured approach radius, they latch the intent and enter through the normal
transition pipeline. This keeps the decision reusable for future randomized or
scripted strategies.
Transition decisions also have a host-approach path: an autonomous controller
may make a deliberate commitment when it is close to a transitionable large
object and its velocity is directed toward the object's approach volume. Once
committed, it continues along that heading until the normal transition is
entered, rather than allowing wander or avoidance noise to cancel the intent.
This is separate from target pursuit and random transition requests, and the
proximity, heading, commitment duration, and cancellation rules remain
configurable by the controller strategy.
Swarm wave respawns are anchored just outside a host Death Star hangar, with
outward-facing poses and host-relative spacing; the default initial profile
uses a matching launch formation so fighters visibly emerge together before
engaging.
The player starts well back in open space while the initial formation is
already active near the hangar, making the launch and approach visible before
the first player shot.
next tuning work is procedural surface density, landmark visibility, trench
dimensions, and installation combat feedback through interactive play.

## Approved renderer migration — model-switch handoff (2026-09-05)

Status: Phase 1 inspection and architectural review complete; user approved the
plan and requested a model switch before implementation. The renderer/model
migration is now underway against the sequence below; no further architecture
approval is required for this agreed scope.
This section supersedes earlier rendering plans wherever they conflict,
especially optional back-face culling and the old four-level realism mapping.

### Objective and non-negotiable requirements

Improve visual fidelity AND aggressively reduce geometry reaching vector draw
submission. Preserve sparse Atari-style luminous lines, strong silhouettes,
negative space, and readable fast movement. Maximum realism must not disable
optimisation. The slider remains the control, with at most five coherent modes.
Physical surface geometry must support mandatory back-face classification and
culling at EVERY level, including retro. Do not retain model-specific culling
exceptions, normal-axis heuristics, automatic reversed-visibility fallbacks, or
missing-face workarounds. Explicit line art (lasers, HUD, reticles, billboard
artwork) is the legitimate exception. Do not implement a TIE Interceptor yet:
it does not exist in the catalog and is the first intended post-migration model.

Keep topology truth, visual policy, and visibility mechanisms separate. Avoid
unrelated controller, gameplay, physics, camera-control, or multiplayer rewrites.

### Verified Phase 1 findings

- `internal/model/model.go`: `Model` contains `Verts`, `Edges`, `Faces`;
  `Edge` contains only A/B indices; `Face` contains only polygon vertex indices.
  Validation checks index range and at least three face entries, not distinct
  vertices, degeneracy, planarity, manifoldness, or winding. No cached normals,
  adjacency, edge kinds, model bounds, or explicit sidedness exists.
- `internal/render/pipeline.go`: each Render call allocates/transforms every
  vertex in a part using View*World, then processes edges and projects endpoints
  repeatedly. Near/far camera-space line clipping and Cohen–Sutherland viewport
  clipping exist. Polygon and complete pre-projection frustum clipping do not.
- Current BackfaceStage recomputes cross products and edge maps each call. An
  X-dominant normal can bypass culling for a whole mesh; an empty front set can
  be replaced by the back set. These are defects to remove after topology repair.
  Arcade has no stages. HiddenLineStage and DepthCueStage are no-op placeholders.
- `internal/scene/object.go`: objects have manual VisualRadius/CollisionRadius;
  parts have style, cockpit flags and DetailTier. There are no geometry-derived
  conservative bounds used for early object culling. Do not use collision radii
  as rendering bounds: they serve a different gameplay purpose.
- `internal/game/game.go`: immediate per-part Render/draw in gameplay, showcase,
  environment tiles/features and transition preview. objectDetailTier has
  projected-size thresholds and hysteresis, but environment paths bypass it.
  HUD counts total/visible objects, not stage work. Four profile IDs, friendly
  labels and stage mappings are spread across game/render code.
- `internal/catalog/registry.go`: lifecycle factories for intact objects,
  fragments and polygon shards; current registry includes TIE, X-Wing, laser,
  Death Star. Cube is a reusable primitive/catalog helper. Model templates are
  already shared in catalog package variables, but slices are not immutable.
- `internal/environment/death_star.go`: streamed tiles and features provide a
  useful hierarchy but no render bounds. Surface/trench have collision planes
  yet line-only render geometry; tower/cannon cube faces are explicitly erased.
- `internal/appearance`: default Death Star is an intentional vector billboard
  with deterministic detail reveal and a black circular starfield mask. This
  is not a shared depth buffer or inter-object hidden-line implementation.
- Camera uses -Z forward in view space; model flight forward is +Z. Existing
  pose/world/view/projection math can be retained. The installed Ebitengine
  v2.9.9 image/triangle API is a 2D draw interface, not a conventional exposed
  application depth attachment; prefer CPU depth initially.

### Required model migrations

| Model/generator | Correction |
| --- | --- |
| Cube and appendBox | Current face winding is inward under the new outward convention; fix shared primitives first. |
| TIE fighter | Correct cockpit/pylon winding, extrude zero-thickness hexagonal panels into thin solids, establish attachment topology and explicit brace/detail ownership. Never exempt whole panels from culling. |
| X-Wing | Correct inconsistent winding across fuselage sides/caps, canopy, prism and wings; validate/triangulate nonplanar polygons; close cannon cylinders as appropriate; remove duplicate nacelle edges; distinguish cap triangulation from authored structure. |
| Orbital Death Star sphere | Replace duplicate polar rings/degenerate polar quads with proper pole triangles; enforce outward winding. |
| Orbital dish | Construct concave face topology for currently line-only dish; separate meaningful ring/spoke detail. Integrate dish opening/occluder geometry so a sphere does not hide its own recessed dish. |
| Surface/trench | Generate decks, floor, walls and finite end faces from the same dimensions as colliders; orient toward navigable space, preserve open trench top, classify grid markings as surface-associated detail. |
| Towers/cannons | Retain corrected cube faces instead of setting Faces=nil. |
| Fragments/shards | Current edges/faces are partitioned independently; rebuild coherent adjacency per fragment, compact unused vertices, handle exposed fracture surfaces deliberately. Detached polygon shards are explicitly double-sided. |
| Laser/HUD/reticles/billboard | Explicit line-art representation; retain clipping, projected-size filtering and suitable depth policy, without fabricated faces. |

The whole fighter need not be a single manifold solid: individually valid
closed components can intersect as an assembly. Open surfaces must declare
their intended side(s). An arbitrary object's centroid cannot establish outward
normals for concave or disconnected components. Do not blindly flip all faces.

### Approved representation direction

Use simple procedural source geometry followed by a compilation step producing
immutable shared render meshes. Exact Go field names are implementation choices.

- Canonical winding: counter-clockwise viewed from outside; right-hand normal
  points outward. Open terrain faces point from material into navigable space.
- Cache face normals/plane constants and validated triangulations once. Depth
  triangles must not automatically become visible wireframe diagonals.
- Derive edge adjacency from faces; retain all incident-face information to
  diagnose non-manifold geometry rather than silently dropping a third face.
- Store authored structural/crease, detail/decorative and internal/construction
  edge intent plus importance. Silhouette and front/back are runtime outcomes.
- Standalone decorative lines bypass face classification; surface-associated
  markings follow their owner's visibility and depth rules.
- Derive conservative local bounds, LOD geometry/group sets and bounded child
  nodes with local transforms. Cache static tiles/modules and reuse geometry.
- Validate finite coordinates, distinct indices, zero-area faces, planarity,
  shared-edge winding, adjacency coverage, and closed-component orientation
  where feasible. Use generator-specific tests for open/concave outward intent.
- Transform/Merge, showcase scaling, fragmentation and factory registration
  must preserve metadata or rebuild compiled topology. Handle reflection and
  nonuniform scale correctly; runtime object poses are normally rigid.

### Approved frame pipeline

1. Gather objects only from the active coordinate frame/presentation context.
2. Cull conservative object bounds against the frustum before vertex work.
3. Traverse bounded regions/modules; select projected-size LOD with hysteresis.
4. Classify cached face planes against camera position expressed in model space
   for rigid instances, before transforming unnecessary vertices.
5. Classify candidate edges through adjacency and profile policy. Back/back
   rejects, front/back preserves silhouette; front/front obeys authored policy.
6. Transform surviving line vertices and vertices required by visible occluders;
   clip lines and polygons to near/far/side frustum planes before perspective.
7. At high levels, resolve a shared depth pass from ALL surviving occluders.
8. Project candidates, reject insignificant projected lengths, depth-test and
   split partially hidden lines, then viewport-clip final segments and draw.

Replace immediate per-part output with frame collection/resolution so another
object's surfaces can occlude a line regardless of submission order. A surface
can occlude even if its own edges are removed by LOD/policy. Use reusable scratch
buffers and concrete structs, not per-frame topology maps or unnecessary stage
interfaces. Instrument allocation cost before clever optimisations.

Use an invisible CPU depth buffer first, pretriangulated/clipped front surfaces,
perspective-correct reciprocal-depth interpolation and line interval sampling.
Tune depth bias/resolution against thin panels, nearby trench walls and surface
markings. Keep final output vector strokes, not visible raster-filled surfaces.
Maximum mode improves useful detail/depth precision and optional restrained
depth cues. Future coarse occlusion/hierarchical-Z can be inserted after bounded
node collection using the shared depth representation; no BVH/octree required
initially. Cache topology, normals, adjacency, bounds and LODs, not dynamic
camera-relative visibility without explicit invalidation.

Billboard appearance remains first-class: define an appropriate generic depth
proxy/occlusion contract for it; do not accidentally depth-render an unrelated
whole orbital mesh behind the arcade drawing. Cockpit-excluded own-ship geometry
must not become an invisible blocker. HUD stays a final overlay. Transition
preview and showcase must use the same visibility mechanisms with correct frames.

### Five profiles (common baseline applies to all)

Common: mandatory surface back-face classification/culling, object/module bounds
rejection, robust clipping, projected-size detail filtering and tiny-edge tests.

| Level | Policy |
| --- | --- |
| 1 Retro Wireframe | Coarse authored geometry and sparse structural/decorative lines; aggressive detail reduction; no depth hidden lines. |
| 2 Clean Wireframe | Projected-size LOD, adjacency visibility, preserved silhouettes and controlled structural detail. |
| 3 Enhanced Vector | Suppress coplanar/internal edges, prioritize silhouettes, retain more useful close detail. |
| 4 Hidden-Line Vector | Shared depth, inter-object occlusion, partial line visibility, detailed useful geometry. |
| 5 Maximum Realism | Finest useful LOD and more precise depth/line sampling; restrained optional depth cues; all optimisations remain active. |

Centralize labels, IDs, policies and thresholds in render profiles, consumed by
the existing clickable slider and keyboard controls. Update existing profile
tests and documentation; do not carry obsolete no-op modes forward as if real.

### Implementation checkpoints and verification

- [ ] Capture baseline deterministic workloads BEFORE renderer changes: TIE,
  X-Wing, orbital sphere/dish, deck/trench/tower scene, repeated instances and
  several fixed/rotated camera poses. Record input topology, output segments,
  elapsed time and allocations. Compare like-for-like geometry separately from
  topology-migration changes; no invented percentage or blanket performance claim.
- [ ] Compile topology and add normal/winding/adjacency/degeneracy tests first.
- [ ] Repair primitives, then major models and debris. Test multiple viewpoints,
  transformed classification, boundary/silhouette/internal/decorative behavior.
- [ ] Remove bypasses; enforce baseline back-face behavior in every mode. Update
  old tests such as TestRenderCubeProducesEveryEdge to the approved semantics.
- [ ] Add bounds, frame queue, hierarchy and LOD. Test spheres outside/inside/
  intersecting frustum, off-centre/scaled bounds, hysteresis and module rejection.
- [ ] Add robust polygon/line frustum and viewport clipping and tiny-edge tests,
  including near-plane crossings and huge potential projected coordinates.
- [ ] Integrate five profiles across gameplay, showcase, surface and transition.
- [ ] Add shared depth and partial-line tests: crossing occluders, self-occlusion,
  submission-order independence, varying depth, bias and thin geometry.
- [ ] Expose per-frame HUD/debug counters only when debug info is requested:
  objects input/culled/surviving; modules culled; faces input/classified/back/front;
  vertices transformed; input edges; rejection by back adjacency, policy, LOD,
  tiny length and depth; clipped edges; final segments submitted; depth work.
  Separate rejected edges from generated visible segments to avoid misleading
  counts when hidden-line splitting increases segment count.
- [ ] Format touched code; run package/full tests and vet, plus benchmarks. Use
  virtual X11 if available for Ebitengine tests; otherwise report the limitation
  and compile the game tests separately. Exercise interactive slider, cockpit,
  showcase, surface/trench, transitions and disintegration when display permits.
- [ ] Report measured before/after workload and allocation results, regressions
  and tradeoffs. Document new-model authoring requirements and confirm that a
  future Interceptor needs only topology/LOD data, registration and its own tests.

### Resume notes

Phase 1 baseline checks passed (2026-09-05):
`go test ./internal/model ./internal/render ./internal/catalog
./internal/environment ./internal/appearance ./internal/camera ./internal/math3d`.
They establish current behavior, not valid topology. Full game tests and actual
rendering benchmarks were NOT run in Phase 1. Worktree was clean on inspection.

Workspace: `/home/ed/projects/starwars`. For reliable Go invocation here use
`env GOCACHE=/tmp/starwars-go-build /snap/go/current/bin/go ...`.
Use separate commands; user explicitly dislikes command chaining and long Snap
formatter waits. Check an available direct formatter before using `/snap/bin`.
Preserve any changes made after this handoff. Do not commit/push unless requested.
No subagents unless the user or applicable repository instructions request them.

### Migration progress (current session)

The migration foundation is now implemented. `model.Prepare` compiles immutable
face normals and plane constants, preserves all incident face adjacency,
classifies coplanar construction seams, and derives conservative local bounds.
Validation rejects non-finite vertices, self-loop edges, repeated face vertices,
and zero-area faces. Cube winding is outward; generated fighter, orbital Death
Star, surface deck, trench, tower, and cannon meshes preserve compiled topology
through Transform/Merge. TIE solar panels are now thin extruded solids rather
than zero-thickness sketches, and debris fragments rebuild their adjacency after
partitioning. The Death Star sphere now uses explicit pole vertices and
non-degenerate cap triangles; its recessed dish also has sparse concave face
topology for depth occlusion.

The mandatory back-face stage is adjacency-driven in all five profiles. The
former axis heuristic and reverse-front fallback are gone. Decorative/detail
lines remain explicit line art and are retained only when their owning surface
is visible. Higher profiles remove coplanar internal seams. The renderer now
supports conservative object bounds rejection, projected tiny-edge filtering,
five profile mappings, per-frame visibility counters, and cached model normals
transformed through the rigid view/object matrix.

The high profiles now build a shared CPU depth surface from polygon faces before
vector submission. Near/far depth polygons are clipped, reciprocal depth is
rasterized, and candidate lines are sampled into visible intervals so an edge
can be split around an occluding surface. Billboard artwork and cockpit-excluded
parts remain outside the depth pass. HUD debug text exposes object, transform,
back-face, depth, tiny-edge, and final-line counts. The showcase and streamed
surface modules use the same prepared topology, profile stages, projected-size
rejection, and (at higher levels) CPU depth surface. Environment module bounds
are tested before transforming their vertices; the active-frame object filter
also applies to depth generation so hidden or not-yet-launched swarm objects
cannot become invisible occluders.

The first interactive artifact pass also corrected procedural component winding
with an explicit generator-time `OrientOutward` step for convex X-Wing/TIE
components. The depth resolver now uses configurable relative bias and a small
neighborhood sample around boundary pixels, while maximum-realism tiny-line
thresholds retain more detail than the coarser profiles. These changes target
rotation-dependent edge gaps and sparkle without disabling culling.

The X-Wing S-foil slabs are now defined as coherent thin solids: the broad
panel corners are coplanar and the rear surface is a fixed-thickness extrusion.
Each surface is represented as planar triangles rather than warped
quadrilateral faces. Their triangulation diagonals are explicit internal
construction edges and are suppressed from vector submission at every profile;
this keeps face normals and depth coverage stable while the ship rotates.

Depth samples now carry a render-owner ID. A fighter's own polygons cannot erase
its structural edges through depth sampling, while surfaces from other objects
and environments remain valid occluders. This separates self-occlusion policy
from inter-object hidden-line removal and is especially important for compound
ships such as the X-Wing.

Depth ownership is now assigned per physical scene part rather than only per
object. This permits intentional intra-object occlusion: the TIE fighter's
cockpit/pylons and solar-panel foils are separate model parts, so a foil can
hide cockpit geometry while each part retains its own stable structural lines.
The X-Wing follows the same rule with separate fuselage/canopy, S-foil/engine/
cannon parts, with each of the four foil assemblies independently owned; this
is a reusable scene composition convention, not a fighter-specific renderer
branch. The same granularity is used for the TIE's left and right foils.
Within each X-Wing foil assembly, the wing panel, engine nacelle and cannon
are independently owned; the nacelle's rear and forward sections are separate
solids as well, so mounted hardware can correctly occlude (or be occluded by)
the panel and fuselage.
Those nacelle sections and cannon barrels now include explicit end caps; they
are closed solids rather than open side-only cylinders, preventing rear wing
geometry from showing through a forward intake.
The X-Wing fuselage and canopy are likewise separate scene parts, allowing the
cockpit shell to occlude rear-facing details without a renderer special case.
The S-foil geometry now uses a thin planar X-Z outline with Y-thickness and
dihedral, keeping both root and tip edges parallel to the fuselage while the
trailing edge sweeps aft into the fuselage's rear section.
The canonical wing root is positioned at the fuselage side rather than near
the centerline, matching the X-Wing front-view attachment geometry. Root and
tip centers now share a radial angle so each foil projects as a straight,
mirrored assembly rather than a kinked one.
The canopy front profile is centered across the fuselage cross-section rather
than being entirely above the centerline, keeping the cockpit visually aligned
with the symmetric front-view wing attachment.

Focused model, topology, winding, adjacency, culling, clipping, depth, profile,
catalog, environment, and registry tests pass; `go vet ./...` passes; the game
package compiles with `go test -c`. Full Ebitengine runtime tests and interactive
visual checks still require an X11 display in this environment. Remaining work
is robust side-frustum polygon clipping, tile/module bounds and hierarchy,
allocation/workload benchmarks, richer stage counters, visual regression checks,
and any model-specific topology corrections found during those checks. The TIE
Interceptor remains intentionally unimplemented and is the first consumer after
these foundations are stable.
