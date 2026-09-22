# Star Wars Vector Arcade Homage — Go Build Plan

Non-commercial learning project. Homage to 1983 Atari arcade "Star Wars". Wireframe vector style, not photo-realism. All models hand-defined, no copied assets.

## Current strategic priority — playable game first

The immediate project goal is a **complete playable Battle of Yavin vertical
slice**. The engine and renderer have reached the point where the next proof is
not another reusable capability in isolation, but one enjoyable end-to-end
single-player game loop.

The intended longer-term game contains three missions inspired by the original
film trilogy:

1. Episode IV — Battle of Yavin
2. Episode V — Battle of Hoth
3. Episode VI — Battle of Endor

Only Battle of Yavin is currently in active development. Endor and Hoth are future
consumers of architecture proven by a complete Yavin mission; their anticipated
requirements must not cause speculative abstractions to be introduced now.

This priority supersedes the former post-Step-20 ordering wherever it placed an
authoritative server, multiplayer client, external-agent integration, generic
cut-scene framework, or public extension API ahead of scoring, complete game
states, mission selection, outcome presentation, sound, and an end-to-end
single-player mission.

The working architectural rules are:

- `sim.MissionPhase` remains the authoritative gameplay state for an active
  mission.
- Application flow such as title, briefing, playing, outcome presentation and
  result remains outside mission simulation state.
- Reuse the existing host-bound large-object, local-environment and frame
  architecture. Do not add parallel `SpaceDomain`, `SurfaceDomain` or
  `InteriorDomain` abstractions.
- Preserve the logical Death Star host, its scalable orbital vector
  presentation, and the established exterior-to-surface transition. Do not
  replace it with a globally continuous detailed 3D sphere.
- Preserve the prepared rendering pipeline and all five realism profiles.
  Gameplay consumes those systems rather than bypassing them.
- Prefer integration and small focused extensions over rewrites. Do not build
  a generic mission scripting system merely to express Yavin.
- Keep stable IDs, teams, frames, commands, snapshots and events where they are
  already useful, but do not let future multiplayer requirements dictate the
  immediate single-player implementation.

The shortest intended playable loop is:

```text
TITLE / MISSION SELECT
    -> BATTLE OF YAVIN BRIEFING / LAUNCH
    -> ORBITAL COMBAT AND DEATH STAR APPROACH
    -> EXISTING SPACE-TO-SURFACE TRANSITION
    -> SURFACE ASSAULT
    -> LOCATE AND ENTER THE ATTACK TRENCH
    -> TRENCH RUN
    -> EXHAUST-PORT ATTACK
    -> SURVIVE THE ESCAPE
    -> DEATH STAR DESTRUCTION
    -> MISSION RESULT / SCORE
    -> TITLE
```

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
  -> Backface Cull (mandatory for surface geometry, camera space)
  -> Project (perspective -> screen)
  -> Clip (near plane, screen bounds)
  -> Hidden-Line Resolve (optional, projected depth)
  -> Draw (line draw)
```

The renderer holds pipeline configuration and runs these operations in order.
Visibility presentation is selected by a feature setting, but physical surface
geometry always receives baseline back-face classification. The retro arcade
look comes from sparse geometry and edge policy, not from drawing known
back-facing surfaces of closed solids.

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

- The historical `VisibilityAll` mode ignored `Faces`; it is retained only as
  an API-history note. Current arcade presentation still applies mandatory
  back-face culling to physical surfaces and preserves the retro look through
  sparse geometry and edge policy.
- `VisibilityBackfaces` uses camera-space face normals to omit surfaces facing
  away from the camera. It is inexpensive but cannot hide an edge behind a
  different front-facing surface.
- `VisibilityHiddenLines` performs a face-depth prepass, compares projected line
  depth against that surface depth, and splits partially occluded edges into
  visible line segments. This is full hidden-line removal.

The existing edge-only `Culler` hook is sufficient only for basic whole-edge
filtering and will be replaced by these visibility modes. The historical
edge-only models may omit faces only as a compatibility note; all new and
migrated surface-based catalog entries must provide valid faces and participate
in mandatory back-face classification. Face indices and winding are validated
with the rest of each model.

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
primary user-facing control. The current gameplay default is the highest
(`maximum`) profile; the slider still permits selecting cheaper modes.

Current implementation clarification: the historical `VisibilityAll`/level-0
description above refers to legacy line-art behavior and does not disable
back-face classification for physical faces. The current five profiles all
apply mandatory surface back-face culling; they differ in edge policy, LOD,
depth/hidden-line processing, and detail density.

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
visible. Billboard presentations must not paint a large opaque screen-space
mask. A presentation may instead declare a cheap analytic point occluder (for
example, a projected sphere) so sparse background points are rejected before
draw submission. Ordinary solid model faces use projected geometry occluders,
which preserve genuine gaps between surfaces. Each optional detail primitive receives a deterministic
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

Keep the physical tile radius distinct from an optional, larger visual-horizon
radius. Horizon tiles follow only the viewed craft and contain presentation
geometry without installations, collision, targeting, point occlusion, or CPU
depth writes/rasterization. Its vectors may cheaply test depth already written
by foreground structures so distant terrain cannot show through them. This
extends the apparent surface cheaply without
multiplying authoritative work for every nearby actor. Coplanar grid vectors
must not depth-test against their own supporting face; other physical surfaces
may still occlude them, avoiding shallow-angle depth instability and shimmer.

Manual near-surface flight includes optional horizon assistance, enabled in
the default profile. An environment explicitly declares its local level/up
axis; enclosed rooms or frames without a meaningful horizon leave it unset.
When the player supplies neither yaw nor roll input, the assist applies the
shortest bounded local-roll correction toward level flight while preserving
heading and pitch. Explicit turning/rolling always wins, orbital flight is
unchanged, and `L` toggles the assistance at runtime. Correction gain, maximum
roll rate, angle deadzone, and turn-input deadzone are profile data rather than
game-loop constants.

Rendered tiles and collision tiles are generated from the same validated
module definitions so visible floors, trench sides, trench bottoms, towers,
cannon emplacements, antennae, and block structures have matching collision
geometry. Use a small set of reusable collider primitives: swept fighter
spheres against finite planes/rectangles for decks and trench walls, and
oriented boxes or other simple convex bounds for structures. Projectiles use
the same earliest-time-of-impact query. Broad-phase tile bounds reject distant
features before narrow-phase tests; collision geometry is independent of visual
LOD, so a hidden far-detail feature cannot become non-physical accidentally.

#### Surface-installation variety and destruction — Step 20 slice

This slice precedes the first flyable hangar. Replace the current shared
scaled-cube tower/cannon placeholder with a small reusable catalog of distinct
Death Star installations: at least a stepped tower, a recognizable laser-cannon
emplacement, a slender aerial/antenna, and a low block or vent structure.
Keep the original arcade vocabulary of sparse green vectors, strong silhouettes,
and large negative spaces. Models should have valid faces, winding, bounds, and
opaque physical surfaces so they use the existing culling, depth, and prepared
frame pipeline; do not add installation-specific renderer branches. A feature
prototype owns its immutable model parts and a simple matching collider or
small compound of colliders. Use the existing instance transform and scale
rather than building a fresh mesh per tile.

Choose feature kinds, orientation, size, and clustering with a deterministic
tile-coordinate seed. Preserve stable feature IDs and destroyed/damaged state
when tiles stream out and back in. Keep useful low-clutter flight corridors,
make a few larger installations visible as navigation landmarks, and avoid
placing destructible modules across the open trench route or future hangar
approach. Visual LOD may suppress small antennas and surface detail, but must
never change authoritative collision, targeting, cannon activity, or damage.

Replace the single hit-to-disappear behavior with a small authoritative
per-feature damage contract: type-specific durability, hit location/normal,
active/disabled/destroyed state, and a deterministic destruction event.
Fragile aerials may fail on one hit; armored towers and cannon bases should
survive at least an initial hit where gameplay balance allows. Cannon fire
stops when the emplacement is disabled. On impact, show a short bounded vector
flash/spark; on destruction, emit a few short-lived fragments with momentum
from the impact and leave an appropriate inert wreck/base or scorch outline
before optional cleanup. Keep persistent wreck collision only where the visible
remnant warrants it. These effects must be capped, frame-aware, and derived
from simulation events so multiple players see the same outcome; they must not
be driven by tile visibility or render timing.

Implement in this order: reusable geometry/collider prototypes; deterministic
placement and landmark density; per-feature hit state and cannon disablement;
bounded impact/fracture presentation; then interactive tuning. Add deterministic
tests for feature identity and layout across tile regeneration, collider/visual
alignment, targetability and damage transitions, cannon shutdown, wreck
persistence, and non-interaction across frames. Check prepared-candidate,
depth, triangle, and vector counts while approaching a dense feature cluster;
reuse aggregate bounds and LOD rather than expanding every tile into render
work. Do not fold the separate flyable hangar, interiors, or exhaust-port bomb
mechanic into this slice.

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
later post-Yavin cut-scene step. Yavin's concrete launch, transition and outcome
presentations should first be composed from the focused camera and presentation
mechanisms that already exist; they do not require the generic timeline schema.
Orbital and surface scenes use ordinary catalog objects,
camera transforms, targeting, controllers, and rendering profiles; neither the
renderer nor HUD may branch on a Death Star type.

## Live Simulation and Multiplayer Server

**Priority status: deferred until the Battle of Yavin vertical slice has proved
the single-player game loop.** The contracts below remain useful future design
guidance, but they are not prerequisites for Yavin and must not force a
participant/server refactor into the immediate mission work.

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

Local single-player mode may eventually become the one-participant case of this
session model. Until multiplayer again becomes a priority, avoid spreading new
singleton-player assumptions into reusable simulation contracts, but do not
delay Yavin in order to migrate every existing `fighterID`, shield, destruction,
respawn, targeting, and controller-role field. Perform that migration as part
of the future authoritative-server milestone, informed by the completed game
rather than speculative requirements.

## Customization and Extension Architecture

**Priority status: foundation substantially established; public API
stabilization deferred until after Yavin.** The existing registries and
extension hooks remain the internal architecture used by normal gameplay. A
later architectural milestone may turn them into a coherent public
customization API before network transport or external agents. Contributors
should eventually be able to add a behavior,
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
validated Go values; `Pilot` preserves the established gameplay, the title
screen selects among the curated difficulties, and the command line supplies
the initial selection. `Game.NewWithProfile` clones and retains the caller's
resolved configuration until the player explicitly changes difficulty. The
fresh gameplay session is constructed only when the player launches from the
briefing. External JSON/YAML loading remains later work after the schema has
seen further use.

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
be validated at load time. The application shell selects a curated profile
before starting a game while preserving an explicitly supplied custom profile
until the player changes that selection. A future authoritative server will
reject mismatched profiles in a multiplayer session.

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
21. Battle of Yavin Milestone 1 — game shell and clean session lifecycle
22. Battle of Yavin Milestone 2 — arcade mission structure and fighting approach
23. Battle of Yavin Milestone 3 — authoritative attack and escape rules
24. Battle of Yavin Milestone 4 — score and result loop
25. Battle of Yavin Milestone 5 — outcome presentation and audio
26. Battle of Yavin Milestone 6 — balance, performance, documentation and release gate

The former post-Step-20 sequence is preserved as future work but no longer
controls immediate implementation order:

| Former step | Previous scope | Revised position |
|---|---|---|
| 21 | Generalized camera anchors | Existing camera/anchor capability is sufficient for Yavin; additional generalization moves after Yavin unless a concrete scene requires it. |
| 22 | General cut-scene orchestration | Only the concrete Yavin launch, transition and outcome presentations move into the vertical slice. The generic registered timeline remains post-Yavin. |
| 23 | Authoritative server | Deferred until single-player Yavin is complete and evaluated. |
| 24 | Rule-driven intelligence library | Reuse current pursuit and surface-combat behavior. Only Yavin phase-aware encounter tuning moves forward; the general library remains post-Yavin. |
| 25 | Multiplayer client | Deferred with the server work. |
| 26 | External agent adapter | Deferred until multiplayer/external control is again a priority. |
| 27 | Score, complete game states, difficulty UI and sound | Split across Yavin Milestones 1, 4 and 5 and brought forward. |
| 28 | Public extension API | Deferred until the working game reveals which contracts deserve stabilization. |
| 29 | Prediction, reconciliation, replay, persistence and multiple rooms | Network portions remain deferred; only focused deterministic mission tests are required for Yavin. Existing room/portal capability is retained. |
| 30 | Controller evaluation, tournaments and hot-loading | Deferred until after game and multiplayer priorities. |
| 31 | CRT/vector-glow and fixed-point experiments | Optional post-Yavin presentation work. |

## Notes
- Step 5 is first visually demonstrable milestone (target early win).
- Keep each step in its own commit/branch for incremental review.
- Physical surface back-face culling is mandatory in every profile. Hidden-line,
  depth, LOD, and edge-policy sophistication remain runtime-switchable through
  the five-level realism slider.
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
- Concave dish/sensor modules: do not apply a convex `OrientOutward` heuristic
  to bowl geometry. Author or validate the opening-facing winding explicitly,
  with front concave faces and a separately modeled rear profile (normally a
  convex dome) when both sides can be viewed. Add rear rings/spokes as real
  surface geometry with the correct opposite normals, rather than relying on
  duplicated or screen-facing line art. Keep front/rear curvature, cap
  thickness, and depth ordering explicit so back-face culling and hidden-line
  removal show the intended detail from every orientation. This pattern is
  reusable for future sensor dishes, turrets, engine nozzles, radar bowls, and
  other recessed emitters.
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
and five built-in progressive profiles (`arcade`, `culled`, `hidden-line`,
`depth-cue`, and `maximum`). Mandatory back-face culling applies to physical
surface geometry in every profile. Profiles are selected from
`Display.RenderingProfile`; legacy culler support remains compatible while
depth, LOD, clipping, and visibility stages evolve independently.

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

The streamed surface now has two ranges: a compact physical neighborhood around
relevant craft and a larger visual-only horizon centered on the viewed craft.
The outer shell preserves deck and trench line art but strips features,
colliders, targeting metadata, sparse-point occlusion, and CPU depth writes.
Its lines remain depth-test-only so foreground structures occlude the distant
surface without rasterizing the horizon geometry itself.
Surface grid strokes also bypass self-depth sampling to prevent distant
coplanar lines sparkling at grazing camera angles.

The Death Star environment declares local `+Y` as its level reference. Default
player profiles enable bounded surface auto-level assistance; the manual
control layer supplies a roll-only correction when yaw and roll are
uncommanded, with a runtime `L` toggle and HUD state. The correction is generic
to any environment that declares a level axis and does not activate in exterior
space or enclosed rooms by assumption.

Near-surface combat now has a profile-owned flight and encounter envelope rather
than changing the global simulation clock. Surface entry raises the viewed
craft to a faster local cruise speed, permits a higher local maximum, and starts
one deterministic host/frame-scoped encounter. The direct development entry and
the ordinary orbital transition share this path. Initial attackers and capped
reinforcement waves reuse registered craft and controllers; an optional
controller engagement hook removes only the initial idle gap while preserving
ordinary attack-run behavior.

Combat allegiance is explicit `scene.TeamID` data. Controller targeting selects
and then retains an eligible targetable hostile in the same spatial frame; it
does not infer factions from models, colors, or a hard-coded local-player ID.
This is the baseline contract for multiple players and allied autonomous craft.
Environment encounters are created once per bound host/frame, do not restart
for each participant, and select available participants by team. All scheduling
uses fixed-tick simulation time, stable IDs, sorted choices, and deterministic
seeds.

Addressable surface features carry team identity. Nearby cannon installations
use persistent per-feature cooldown state, select the nearest hostile in their
frame, lead imperfectly, and emit faction-styled projectiles under configurable
range, cadence, lifetime, accuracy, active-cannon, and projectile caps. Visual
tile visibility cannot activate or retime the encounter. Feature destruction
and cooldowns survive visual tile streaming; friendly projectiles may be
physically stopped by friendly installations but cannot destroy them.

Planned follow-up — traversing surface cannons: separate each cannon's fixed
base from a yawing/pitching turret and barrel assembly. Keep target selection
and predicted aim in authoritative simulation state, then slew the assembly
toward that aim under configurable traverse speed and angle limits. Fire only
when the muzzle has a valid line of fire and is sufficiently aligned; derive
both the rendered turret pose and projectile origin/direction from the same
orientation so bolts never appear to leave a sideways barrel. Preserve the
current forward-arc restriction as a mechanical traverse limit, not a static
model orientation. Keep per-feature aim state stable across visual tile
streaming and deterministic for multiple players; damage/disablement must
stop tracking and firing. Use ordinary articulated part transforms and the
existing visibility/collision pipeline, with no cannon-specific renderer path.
Test target acquisition, slew and limits, moving-target lead, firing alignment,
obstruction, tile regeneration, and cross-frame/team exclusions.

Status (2026-09-17): the cannon foundation stays fixed while a separate turret
housing yaws and its paired barrels yaw/elevate around authored pivots. Stable
per-installation simulation state slews toward imperfect predictive aim under
profile-controlled limits; firing waits for alignment and a clear path past
other installations. Both the visible barrels and the bolt's origin/direction
use the same articulated transform. The existing conservative installation
colliders remain fixed; exact moving-barrel collision is a separate refinement
if playtesting shows the approximation is noticeable.

Autonomous surface flight composes terrain guidance around the existing pursuit
decision. The guidance samples authoritative deck or trench-floor planes for
predicted clearance and nearby installation boxes for forward obstruction,
then adds bounded climb and lateral avoidance intent before the existing
acceleration and angular-rate limits are applied. It therefore preserves normal
pursuit, attack arcs, trench-floor flight, and smooth physical steering.
Short-lived, capped vector impact sparks and close crossing attack waves provide
speed and battle cues without adding persistent collision geometry. Projectiles retain
only a one-tile neighboring collision stream rather than allocating the full
physical tile square around every long-lived bolt.

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
The surface-installation slice now has four shared sparse solid prototypes:
stepped towers, paired-barrel cannons, sensor aerials, and low armored vents.
Deterministic tile seeds vary orientation, proportions, rare tall landmarks,
and occasional vent/aerial clusters without changing stable feature IDs.
Prototype-local collider components and cannon muzzle anchors use the same
instance transform as their visible geometry; static cannons select targets
within their forward firing arc so bolts do not pass through their own bodies.
Per-bound-environment hit state survives tile regeneration: aerials fail in one
hit, towers/vents in two, and cannons disable after two hits before breaking on
the third. Hits tint the
structure and emit capped sparks; final hits emit short-lived moving shards
and leave a non-colliding persistent scorch foundation. The render path uses
ordinary prepared candidates, bounds, opacity, and depth rather than special
installation drawing. Deterministic geometry, regeneration, damage, firing,
and collision tests cover the slice. Each hit also emits a tick-scoped
`sim.FeatureDamageEvent` in snapshots, separating the authoritative outcome
from its bounded spark/shard presentation; future multiplayer synchronization
must include persistent feature state for late joiners. Interactive
visual/performance tuning of the new silhouettes and effect timing remains the
acceptance check before the first flyable hangar/portal interior; trench
dimensions can be tuned there too.

The first flyable Death Star hangar is now an open-front, host-bound room on
the near-surface deck. A surface doorway and matching interior portal provide
bidirectional, heading-gated frame transfers that retain the fighter's
orientation, lateral position and motion; a short inward nudge prevents
immediate trigger bounce. The exterior shell and interior floor, walls, roof
and back wall have matching collision boundaries. Sparse landing guides and
ceiling ribs preserve the vector style. Through the open doorway, a bounded
set of ordinary surface tiles is retained and rendered through the prepared
pipeline. Exterior lines and prepared physical triangles are clipped against
the projected convex doorway before depth, point occlusion, fill or vector
submission; the open portal exposes skyfield. The same authored opening is
now considered from both linked frames. Adjacent-frame fighters and other
ordinary scene objects enter the prepared frame with the doorway clip; laser
bolts crossing its finite polygon transfer authoritatively into the adjacent
frame with continuous pose and velocity. Interactive review should check actor
and bolt continuity on both sides. Near-plane clipping of the portal polygon,
source-side collision ordering for a bolt that traverses a doorway in one tick,
and target acquisition across a visible opening remain focused follow-ups.

### Mission direction — Yavin vertical slice, Endor and Hoth later

The current playable mission is explicitly the Battle of Yavin assault on the
first, completed Death Star. Do not introduce an interior reactor-shaft route
into this mission. The hangar remains an optional flyable location and a useful
validation of portals/interiors, but its rear wall is closed and it is not a
path to the station's reactor.

The longer-term selectable missions are:

- Episode IV — Battle of Yavin;
- Episode V — Battle of Hoth; and
- Episode VI — Battle of Endor.

Only Yavin is active development. A title/mission-selection screen may show
Hoth and Endor as unavailable future missions, but it must not instantiate
their environments, rules or speculative shared abstractions.

The Battle of Yavin mission loop is:

1. arrive from hyperspace in orbital space and **break through Imperial
   defences** while physically advancing toward the Death Star;
2. approach the Death Star and transition into near-surface flight;
3. cross the surface, attack or evade installations, locate the finite trench,
   and enter it through ordinary continuous flight;
4. fly the trench under pressure from pursuing fighters, surface cannon fire,
   walls and authored obstacles;
5. reach the terminal exhaust port and deliver a proton torpedo into it under
   explicit range, alignment and approach constraints;
6. survive the reactor-chain-reaction warning, escape the trench and surface,
   use the existing surface-to-exterior transition, and reach safe distance;
7. show the Death Star destruction/outcome presentation, award success, and enter
   a clear completed mission state. Destruction, timeout or a missed attack run
   enters a clear failed/retry state without silently resetting mission state.

The orbital phase is not an isolated arena and must not be expressed as a
visible `DESTROY N TIE FIGHTERS` gate. The intended objective is approximately:

```text
BREAK THROUGH IMPERIAL DEFENCES
```

The player continues to make spatial progress toward the Death Star while
fighting its fighter screen. Mission progression should use an appropriate
combination of spatial/progress evidence and minimum engagement or survival
evidence. The exact rule should be selected from implementation and playtest
evidence; it must not make the Death Star feel artificially unavailable until a
kill counter reaches an arbitrary number.

After a valid exhaust-port hit, the authoritative and presentation sequence is:

```text
EXHAUST PORT HIT
    -> REACTOR CHAIN REACTION / ESCAPE WARNING
    -> PLAYER ESCAPES TRENCH
    -> SURFACE ESCAPE
    -> EXISTING SURFACE-TO-EXTERIOR TRANSITION
    -> REACH SAFE DISTANCE
    -> DEATH STAR DESTRUCTION PRESENTATION
    -> MISSION RESULT / SCORE
```

The exhaust-port hit begins the escape; it does not complete the mission. The
player must survive until the declared safe-distance condition is met. Player
destruction or expiry of the authoritative escape deadline produces a clear
failure.

Existing foundations already cover orbital combat, approach presentation,
surface flight, a finite trench and terminal exhaust-port feature, surface
installations, collisions, pursuit across the orbital/surface boundary, and
the focused camera/presentation primitives. The authoritative mission phases,
proton-torpedo weapon and exhaust-port validation are also implemented. The
remaining Yavin work must integrate those foundations rather than add more
renderer architecture:

- an escape countdown, safe-distance rule and deterministic station-destruction
  outcome consumed by presentation code;
- an authoritative score breakdown and source attribution for score-bearing
  destruction events;
- HUD cues for current objective, torpedo count, target lock/readiness,
  countdown and success/failure, while keeping targeting state authoritative;
- tests for objective ordering, invalid shortcuts, torpedo/exhaust-port impact,
  ownership/team rules, escape success, timeout/failure and deterministic
  replay of mission decisions. These tests should preserve future multiplayer
  compatibility without requiring multiplayer implementation now.

Continue in the cheapest gameplay-first order defined by the six approved
milestones below. Do not require a new rendering abstraction for these rules.

Status: Milestones 1 and 2 are implemented. The renderer-independent
simulation world and snapshots retain the legal ordered phases `orbital battle
-> approach -> surface assault -> trench run -> exhaust-port attack -> escape
-> success`, with failure available from any active phase and deterministic
restart to orbital battle. The orbital approach now requires real closure on
the logical Death Star and cumulative time under an active Imperial fighter
screen; it is not a visible kill quota. The surface environment owns a
forward-only entry mouth and ordered route gates derived from its physical
trench tiles, so a terminal drop or reverse shortcut cannot enable the attack
run. Existing surface pursuers and reinforcements escalate from the initial
surface assault to the trench run without creating a second behavior system.
Gameplay advances these phases from authoritative player frame/transition and
physical trench-region data; camera and render visibility are not inputs.
Player destruction fails the mission, the direct surface-development start
advances through the skipped phases explicitly, mission changes emit
tick-scoped events, and the debug HUD reports the current objective. Proton
torpedoes are catalogued, visually distinct projectiles with finite player
ammunition, a dedicated command/cooldown, authoritative ownership/team
identity, and travel distance accumulated by the simulation. The terminal
exhaust port accepts only an Alliance player's proton torpedo inside the
configured arming/range, alignment, and forward/downward approach envelope.
Invalid payload, owner, range, aim, or direction attempts retain the attack
objective and publish an authoritative reason; a valid impact advances to
escape. Escape completion and success remain deliberately unreachable until
the countdown/outcome slice.
The main player's first-person cockpit also has a compact vector targeting
computer that names the current objective and uses a restrained red direction
arrow. Its target is semantic mission geometry rather than whatever happens to
be rendered: the host Death Star in orbital/approach phases, the nearest valid
trench guide point during surface assault, and the authored exhaust-port point
during the trench/attack phases. Camera-space direction handling continues to
guide when the objective is off-screen or behind the fighter.

The interior-superstructure assault is reserved for a later Battle of Endor
mode against the incomplete second Death Star. That mode may use open structural
flight volumes, tunnels and rooms leading to a reactor chamber followed by a
timed escape. It should reuse the generic host-bound frame, room, portal,
collision, prepared-frame and projectile-transfer contracts, but have its own
mission profile, environment registrations, objectives and set pieces. Nothing
specific to Death Star II should be registered or active in the Yavin mission.

Endor is expected eventually to add an unfinished Death Star II orbital vector
presentation, superstructure entry, linked interior flight spaces, a reactor
assault, interior escape, surface breakout and exterior escape. Its
surface-to-interior transitions should build on the existing
`exterior -> transition -> local environment` pattern. Do not build the
superstructure graph or a generic interior-domain framework during Yavin.

Hoth is expected to require genuinely different capabilities: planetary or
natural terrain flight, T-47 snowspeeders, Rebel defensive positions, AT-ST and
AT-AT ground units, articulated/hierarchical models, and tow-cable attacks.
Hoth must not be forced into the Death Star large-object abstraction merely
because both missions contain low-altitude flight. Define its reusable terrain
architecture only when Hoth supplies concrete requirements.

### Approved Yavin-first implementation roadmap

#### Milestone 1 — Game shell and clean session lifecycle

Add an explicit application-flow state outside `sim.MissionPhase`, covering
title/mission selection, briefing, playing, outcome presentation and result.
Keep pause and development diagnostics orthogonal to that flow. Replace the
current partial reset behavior with a complete Yavin-session construction/reset
path so retry and return-to-title cannot retain projectiles, feature damage,
controllers, respawns, transitions, timers, score or mission feedback from the
previous run.

Use a deliberately small mission-selection descriptor list. Show Yavin as
playable and Hoth/Endor as future missions. Expose the existing curated
difficulty profiles without creating a mission scripting or plugin framework.
Compose the existing hyperspace arrival into Yavin's launch presentation.

Acceptance follows this application path, plus deterministic clean retry and
return-to-title state:

```text
title -> select Yavin/difficulty -> briefing -> hyperspace launch -> playing
```

#### Milestone 2 — Give the mission arcade structure

Turn the implemented mechanics into a directed assault rather than a sandbox.
The orbital objective is `BREAK THROUGH IMPERIAL DEFENCES`: combat and spatial
approach happen together. Investigate a small deterministic combination of
progress toward the station, time under engagement, hostile pressure, and/or
combat participation. Do not expose a simple kill quota and do not hold the
Death Star behind an unrelated arena-wave gate.

Add ordered, directional trench checkpoints derived from the authored physical
trench so reverse entry or appearing near the terminal tile cannot skip the
attack run. Drive existing orbital fighters, surface reinforcements, cannons and
trench pursuers from mission phase to produce a clear escalation. Reuse current
controllers and encounter state; do not first build a generic behavior library.

Acceptance: normal play reliably flows through fighting approach, surface
assault, trench discovery, legal trench entry and the terminal attack run while
leaving the player in physical control.

Status: implemented with an authoritative closure-plus-engagement approach
gate, forward-only trench entry and checkpoint progression, and phase-driven
surface encounter escalation. Attack validation, escape and outcome remain
Milestone 3+ work.

#### Milestone 3 — Complete the authoritative attack and escape rules

Retain the implemented proton-torpedo and exhaust-port validation. Add a
pre-launch readiness result derived from the same authoritative range,
alignment, arming and approach rules so the cockpit can guide rather than only
explain a rejected impact.

A valid hit records the reactor-chain-reaction state and escape deadline. The
objective then guides the player out of the trench, across/away from the
surface, through the existing altitude-based exterior transfer and toward a
safe-distance condition. Success is impossible at the instant of impact.
Destruction, timeout, or exhausting the permitted attack without success enters
an explicit failure state rather than silently resetting.

For the first vertical slice, retry may reconstruct a clean mission from the
result screen. A general save/checkpoint framework and speculative multiplayer
ownership migration are not prerequisites.

Acceptance: valid attack, safe escape, timeout, destruction and missed-attack
paths all reach deterministic authoritative outcomes.

#### Milestone 4 — Score and result loop

Add an authoritative mission score and stable breakdown for fighter kills,
appropriate surface installations, bounded time/accuracy/survival bonuses and
mission completion. Add source identity to damage/destruction events where it
is currently missing. Completion of the Death Star objective must dominate the
scoring model: cap farmable combat points or make the completion award greater
than the maximum possible combat subtotal.

Add a result screen showing outcome, score breakdown and controls for clean
mission retry or return to title. Player destruction in an active mission goes
to this flow instead of the old indefinite sandbox respawn behavior.

Acceptance: farming respawning enemies cannot outscore completing Yavin, and
success/failure both complete the application loop.

#### Milestone 5 — Outcome presentation and audio

Compose the existing fixed/chase/cockpit cameras, catalog geometry, vector
effects and mission events into focused Yavin launch, warning and outcome
presentations. Use a bounded vector Death Star destruction effect; do not feed
the station into fighter-fragmentation logic and do not require a general
registered cut-scene timeline.

Add a small event-driven sound layer for UI selection, lasers, proton
torpedoes, impacts, warnings and the final explosion. Use original/generated or
appropriately licensed assets and keep audio outside authoritative simulation.

Acceptance: mission success and failure read as deliberate endings rather than
debug state changes, while skip behavior reaches the same final state.

#### Milestone 6 — Balance, performance, documentation and release gate

Tune mission duration, spatial approach pressure, fighter waves, surface and
trench speed, cannon density, escape deadline, score balance and all four
difficulty profiles. Add representative full-Yavin orbital, surface, trench
and outcome workloads to the existing performance instrumentation. Complete
only renderer work demonstrated to block correctness or frame budget; do not
make unrelated migration cleanup a release prerequisite.

Update README, controls and mission documentation. Validate every realism
profile, the full automated suite under Xvfb, vet/static checks, Windows 11
build/deployment, and interactive end-to-end runs.

Acceptance: one complete, repeatable and enjoyable Battle of Yavin mission is
the project's new playable baseline.

### Work deliberately placed after Yavin

- Full registered/data-driven cut-scene orchestration.
- Authoritative server, first-class participant migration, multiplayer client,
  prediction/reconciliation and network persistence.
- External/MCP agent adapters and controller tournaments.
- Public extension API stabilization and strategy hot-loading.
- Complete retained-overlay migration or renderer cleanup not justified by a
  Yavin correctness/performance issue.
- Battle of Endor and Battle of Hoth content and their new capabilities.

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
artwork) is the legitimate exception. The TIE Interceptor is now the first
post-migration model and must remain a normal catalog consumer of these rules.

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
  with deterministic detail reveal and a declared analytic spherical point
  occluder. It no longer paints a black circular starfield mask; sparse stars
  are rejected before submission. This remains separate from shared geometry
  depth and inter-object hidden-line processing.
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

- [x] Capture deterministic correctness workloads for the TIE, X-Wing, orbital
  sphere/dish, deck/trench/tower scene, repeated instances and fixed/rotated
  camera poses. The original pre-migration elapsed-time/allocation baseline was
  not captured and must not be reconstructed or claimed retroactively.
- [ ] Establish repeatable current performance baselines for orbital,
  near-surface, trench, interior/portal, showcase and dense-combat views. Record
  input topology, prepared/output work, elapsed time and allocations so future
  changes have a trustworthy comparison point.
- [x] Compile topology and add normal/winding/adjacency/degeneracy tests first.
- [x] Repair primitives, then major models and debris. Test multiple viewpoints,
  transformed classification, boundary/silhouette/internal/decorative behavior.
- [x] Remove bypasses; enforce baseline back-face behavior in every mode. Update
  old tests such as TestRenderCubeProducesEveryEdge to the approved semantics.
- [x] Add bounds, prepared-frame queue, practical tile/feature hierarchy and
  projected-size LOD. Test spheres outside/inside/
  intersecting frustum, off-centre/scaled bounds, hysteresis and module rejection.
- [x] Add robust near/far polygon clipping, line viewport clipping and tiny-edge
  tests, including near-plane crossings and huge potential projected coordinates.
- [ ] Complete general side-frustum polygon clipping; current portal-aperture and
  near/far clipping cover implemented scenes but do not constitute the fully
  general polygon-frustum contract originally requested.
- [x] Integrate five profiles across gameplay, showcase, surface and transition.
- [x] Add shared depth and partial-line tests: crossing occluders, self-occlusion,
  submission-order independence, varying depth, bias and thin geometry.
- [x] Expose per-frame HUD/debug counters only when debug info is requested:
  objects input/culled/surviving; modules culled; faces input/classified/back/front;
  vertices transformed; input edges; rejection by back adjacency, policy, LOD,
  tiny length and depth; clipped edges; final segments submitted; depth work.
  Separate rejected edges from generated visible segments to avoid misleading
  counts when hidden-line splitting increases segment count.
- [x] Format touched code; run package/full tests and vet. The installed
  Xvfb-backed `make test` target covers the Ebitengine suite without a physical
  display. Interactive reviews have exercised the slider, cockpit, showcase,
  surface/trench, transitions and disintegration paths.
- [x] Document the new-model authoring requirements and validate them with the
  TIE Interceptor as a normal topology/LOD/catalog consumer without renderer
  special cases.
- [ ] Maintain a checked-in benchmark/report covering representative before/after
  workload, allocation, regression and quality tradeoffs. Recent near-surface
  profiling is useful evidence but is not yet the durable multi-view baseline
  required by this checkpoint.

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

#### Cockpit occlusion follow-up

Intact TIE-family cockpits are already closed finite surface bodies. The ideal
refinement is therefore to make the existing depth pass handle self-occlusion
reliably for that topology. They currently use the stable per-part ownership
path without self-depth testing; this avoids rotation-dependent shimmering but
can allow sparse rear cockpit or pylon edges to appear through the cockpit
shell. Do not fix this by widening back-face tolerances or aggressively
increasing self-edge sampling. If closed-body self-occlusion cannot be made
stable with the existing surface depth, a reusable depth-only cockpit occluder
(or equivalent prepared shell) is the fallback—not a replacement for the
cockpit's physical topology. It should:

- be usable by both the TIE fighter and TIE Interceptor;
- remain separate from visible cockpit line art and rendering-profile policy;
- hide rear/internal pylon and cockpit geometry without self-occlusion sparkle;
- preserve the existing non-shimmering baseline when disabled or unavailable;
- participate in the same clipping, bounds, and depth-owner pipeline as other
  solid surfaces; and
- be covered by deterministic tests for front/rear cockpit views and rotating
  pylon intersections before enabling it in gameplay.

Focused model, topology, winding, adjacency, culling, clipping, depth, profile,
catalog, environment, and registry tests pass; `go vet ./...` passes; the game
package compiles with `go test -c`. Xvfb is now installed and `make test` runs
the complete Ebitengine-backed suite successfully without a physical display;
interactive visual acceptance still requires running the game. Remaining work
is general side-frustum polygon clipping, durable multi-view
allocation/workload benchmarks, visual regression checks, and any topology
corrections found during those checks. The
TIE Interceptor now exercises the post-migration catalog, appearance, topology,
occlusion, weapon-style, showcase, and destruction contracts.

Current post-migration rules now also include the following implementation
details. `model.Prepare` is the required compilation boundary for cached
normals, plane constants, adjacency, edge classifications, and bounds;
runtime instances must reuse its immutable topology. The five realism profiles
all retain mandatory back-face culling, while only higher profiles add stronger
edge policy, shared CPU depth, hidden-line splitting, and finer LOD/detail.

Starfield visibility is additive. The default `skyfield` mode places stars on
a distant camera-relative directional shell; the renderer rejects hidden stars
before Ebitengine submission. Large line-art bodies may register analytic
projected-volume occluders, while ordinary surface models contribute
camera-facing projected triangle occluders. No large billboard/background fill,
texture mask, or bounding-volume-as-opaque shortcut may be added. Solid model
faces must leave `SkipDepth` disabled so panels and hulls occlude stars without
hiding stars through genuine gaps.

The Death Star retains its scalable arcade billboard as visual line art, but
its former opaque background mask has been removed. Its analytic spherical
occluder and the generic projected geometry occluders share the same screen
point/depth abstraction. This distinction—visual presentation versus physical
opacity—applies to every future large object.

Destruction fragments now receive explicit fracture-cap faces and structural
cap edges. They are ordinary prepared models and opt into self-depth testing so
front caps/body surfaces can hide rear wireframe lines; intact multipart
objects retain per-part depth ownership for stable self-edge rendering.

Laser-bolt geometry is shared across factions and receives a shooter-selected
style: Rebel X-Wing fire is red-orange with blue-white cores; Imperial
TIE-family fire is green with yellow-green cores. New Imperial craft must join
the style mapping through catalog data rather than renderer conditionals.

## New-model authoring contract — TIE Interceptor handoff

This is the authoritative handoff for adding a model. It supersedes older
fighter-specific notes elsewhere in this document; historical sections remain
for milestone traceability only.

The Imperial TIE Interceptor is the first post-migration catalog object. It is
registered as an ordinary model, not through a renderer, camera, HUD,
controller, or collision special case. Its canonical identifiers are:

```text
model:      TIE Interceptor
definition: builtin/tie-interceptor
appearance: builtin/tie-interceptor-model (or another explicit model-3d style)
```

Do not add a legacy alias. Do not rename or mutate `builtin/tie-fighter`; the
existing TIE Fighter and the Interceptor are separate logical definitions.

### Geometry and topology requirements

Use the shared local convention: `+Z` is the nose/forward direction, `+Y` is
up, and `+X` is right. The model should be a composed assembly of prepared
components rather than one fragile monolithic wireframe. At minimum, consider
separate parts for the command pod/cockpit and window, pylons and central
braces, left and right solar-panel assemblies, panel perimeter/braces, and
laser-cannon or muzzle geometry if the craft is fire-capable.

Every surface-based component must provide explicit polygon faces in addition
to vertices and edges. Thin solar panels are extruded solids with front, back,
and side faces; a zero-thickness sketch is not sufficient for reliable
back-face, hidden-line, or point-occluder behavior. Use counter-clockwise
winding when viewed from outside so right-hand normals point outward. Do not
use an axis heuristic, centroid reversal, or an empty-front fallback to hide
bad authoring. Concave or disconnected components must be authored or repaired
component-by-component.

Before registration, `model.Prepare` must compile and cache face normals and
plane constants, triangulation used for depth/point occlusion, edge-to-face
adjacency, edge intent (`structural`, `decorative`, or `internal`), importance,
and conservative local bounds. Depth triangulation must not expose construction
diagonals as visible lines. Faces require at least three distinct, finite,
non-degenerate vertices, and shared edges require compatible winding.

Standalone decorative lines bypass face classification; surface-associated
markings follow their owner's visibility and depth rules. Solid panels and
hulls must leave `SkipDepth` disabled. Intentional double-sided surfaces must
be explicit and rare. The whole fighter need not be one manifold solid:
individually valid closed components may intersect as an assembly, while open
surfaces must declare their intended side(s).

### Catalog, registry, and object assembly

Add immutable shared geometry templates to `internal/model`, then assemble the
object in `internal/catalog` through the existing object-definition registry.
The constructor must supply a stable `builtin/tie-interceptor` definition,
multipart styled `scene.Part` values with explicit detail tiers, named anchors
at least for `center`, `cockpit`, `chase`, and every muzzle used by combat,
independent visual and collision bounds, and validated physical/hittable/
targetable/destructible capabilities as appropriate. If destructible, provide
fragment and polygon-shard factories through the same lifecycle registry.

The catalog must share prepared templates between instances. Per-instance pose,
motion, ownership, controller state, shields, and destruction state remain in
scene/simulation objects; mutable runtime state must not live in model or
catalog globals. Register any model-3d appearance separately from the logical
object. Appearance controls presentation/detail policy and styling, not
collision, identity, ownership, or behavior. Add showcase technical
specification data through the catalog specification path rather than a
renderer type switch.

### Rendering and point occlusion

The Interceptor must work at all five realism levels. Mandatory surface
back-face classification applies even to the retro profile. Higher profiles
may remove coplanar/internal edges, apply projected-size LOD, and resolve
hidden lines using the shared CPU depth surface. Do not disable culling to make
one viewpoint look correct.

Camera-facing solid faces may become projected geometry point occluders for the
sparse skyfield. This must hide stars behind actual panel/hull triangles while
leaving stars visible through empty spaces between panels. Do not use the
object's bounding sphere, convex hull, billboard fill, or a full-screen mask
for an open fighter silhouette. Reuse prepared faces, winding, back-face
classification, clipping, and perspective-correct depth conventions from the
line renderer.

The default starfield mode is `skyfield`: stars are placed on a distant
camera-relative directional shell and rejected before Ebitengine submission
when an analytic or projected-geometry occluder hides them. The legacy `world`
mode remains configurable for tests or scenes that intentionally need parallax.

### Weapons and faction style

Shared laser-bolt geometry is styled from the shooter definition. Rebel X-Wing
fire uses red-orange rays with blue-white cores. Imperial TIE-family fire uses
green rays with yellow-green cores. Add `builtin/tie-interceptor` to the
Imperial style mapping; do not add renderer branches for the new fighter. If a
new weapon is genuinely different, add a catalog style or appearance entry and
keep speed, lifetime, convergence, ownership, and collision rules in combat
configuration.

### Destruction geometry

If destructible, define two or three coherent component fragments that retain
the recognizable silhouette and inherit the parent trajectory with deterministic
blast variation. Add explicit fracture-cap faces at component break planes,
with correct winding and structural cap edges. Polygon shards are generated
from prepared component faces and must not recurse indefinitely.

Destruction components opt into self-depth testing so front caps/body can hide
rear wireframe lines. Intact multipart objects retain per-part depth ownership
to avoid shimmer while allowing one physical part to occlude another. Fragments
and shards remain ordinary catalog objects and use the same renderer; no
destruction-specific drawing branch is permitted.

### Required tests before gameplay integration

Add deterministic tests for the Interceptor before adding it to a live profile:

1. model validation, finite bounds, expected component/face/edge counts, and
   non-degenerate fracture caps;
2. outward normals and winding from fixed, oblique, and panel edge-on views;
3. edge adjacency, boundary/crease/internal classification, and suppression of
   triangulation diagonals;
4. back-face classification and hidden-line/depth behavior for command pod,
   panels, braces, and gaps between panels;
5. projected point occlusion: a star behind a panel is rejected, a star in
   front is retained, and a star through an opening remains visible;
6. near-plane, viewport, projected-size LOD, and off-centre bound behavior;
7. registry construction, anchors, capabilities, appearance selection,
   showcase specification, and shared immutable templates;
8. laser style selection, muzzle orientation, convergence lifetime, and
   projectile ownership; and
9. fragment/shard topology and self-occlusion without regressions in existing
   TIE Fighter/X-Wing tests.

The acceptance test is that adding the Interceptor consists of model data,
catalog/appearance registration, specification/style data, and focused tests—
not changes to renderer branches, camera logic, HUD type switches, or the
simulation loop. Run renderer/model/catalog tests and `go vet ./...`; use the
Xvfb-backed `make test` target for the complete game suite. Interactive
validation should cover all five realism levels, cockpit/chase/follow views,
rapid panel rotations, skyfield occlusion, laser fire, collisions, and the
full two-stage disintegration sequence.

## Rendering architecture simplification — locked decision (2026-09-12)

This decision supersedes the earlier proposal for arbitrarily composable
runtime rendering stages and the strict "additive only" rule wherever they
conflict. Keep the renderer internally phased, because coordinate transforms
and cheap-before-expensive rejection have a necessary order, but implement one
explicit visibility pipeline rather than a graph of interchangeable edge-stage
interfaces.

The fixed pipeline is conceptually:

```text
gather active-frame candidates
  -> object/room/tile/feature bounds rejection
  -> projected-size LOD and material selection
  -> model-space face and edge classification
  -> transform only required vertices
  -> line and polygon frustum clipping
  -> shared depth/visibility resolution where required
  -> background preparation
  -> opaque surface, vector-line and overlay batches
  -> Ebitengine submission
```

Gameplay, showcase, transition/cut-scene, exterior, surface and interior views
must submit candidates through this same frame preparation route. The current
generic `Stage`/legacy `Culler` mechanisms should be removed after their useful
behavior has moved into explicit typed phases. Rendering profiles, if retained,
are data presets for the one pipeline (surface mode, detail thresholds, minimum
line size and depth precision); they do not assemble different algorithms.
Removing the realism selector later must therefore require no renderer redesign.
The retro vector appearance remains an appearance/LOD/edge-policy choice rather
than a less-correct visibility path.

### Generalized scene background

Background selection belongs to the active view/environment rather than to the
global game draw loop. Introduce a small `ViewContext` carrying the camera pose,
authoritative `FrameID`, and a background specification. Initially support
`none`, distant `skyfield`, and world-space stars. Exterior and Death Star
surface views normally select `skyfield`; enclosed Falcon/Death Star interiors
select `none`. Later portal/window visibility may constrain a background to
projected visible regions without introducing background-colored masks.

The background implementation consumes generic visibility information and must
not know about Death Stars, fighters, rooms or docking bays. Analytic sphere
occluders belong to frame visibility preparation, as do projected physical
surfaces. Prepare background visibility after gathering/classifying the scene
so existing depth and projected results can be reused instead of transforming
the same geometry again.

### Visibility-first rendering, replacing strict additive-only rendering

Preserve the performance lesson behind the old rule, not its literal wording.
The governing rule is now:

> Clear once, choose the cheapest correct visibility representation, avoid
> background-colored erasure masks, and do not run an expensive occlusion
> algorithm when ordinary batched opaque drawing is cheaper.

Use the complementary strategies deliberately:

- A large object rendered only as sparse line art, such as the arcade Death
  Star, uses a cheap analytic occluder to reject sparse background stars. Do not
  restore a tessellated black vector fill or paint thousands of background
  pixels merely to simulate opacity.
- Physical wireframe surfaces may reuse prepared faces/shared depth to hide
  background points and other lines. Reuse frame-preparation results; avoid a
  stars-times-every-triangle loop when a depth lookup is already available.
- A genuinely filled opaque surface is allowed to cover previously submitted
  background pixels through normal source-over rendering. That is the object's
  real visible surface, not an erasure workaround. When the background is a
  small prebatched star draw, harmless GPU overdraw may be cheaper than CPU
  point-versus-geometry rejection and is therefore permitted.
- Flat/textured opaque surfaces should be emitted as batched triangles. Luminous
  vector outlines, emissive details, lasers and HUD elements are layered after
  opaque surfaces. Transparent/glass surfaces remain a separately sorted,
  explicitly more expensive edge case.

The renderer must measure both sides of this choice. Track background points,
analytic tests, geometry/depth tests, opaque triangle batches, raster/depth work
and submission time. Select optimizations from measured scene cost rather than
assuming that minimum overdraw always means minimum frame time.

### Retained screen-space vector overlays — planned refinement (2026-09-20)

Cockpit instrumentation, menus, showcase specification panels, mission text,
reticles and debug information are presentation overlays, not physical scene
geometry. They must not enter `preparedFrame`, world/view transformation,
frustum or bounds culling, LOD selection, face classification, surface
triangulation, point occlusion, CPU depth domains, hidden-line resolution, or
world visibility statistics. Their known screen-space draw order is sufficient.
This decision applies to a first-person cockpit representation only; the
externally viewed cockpit/canopy on a fighter remains ordinary physical model
geometry and continues through the full visibility pipeline.

Use this explicit frame composition:

```text
resolved world view
  -> generalized background
  -> filled physical surfaces
  -> depth-resolved world vectors and effects
  -> cockpit overlay
       -> retained static frame/artwork
       -> retained labels and instrument furniture
       -> small dynamic value/cue batches
  -> menus, mission messages and optional debug overlay
```

Opaque or translucent cockpit regions may cover the completed world through
ordinary final-layer source-over composition. This is legitimate overlay
composition, not a physical occlusion mask, and must not cause cockpit artwork
to be registered as a depth writer or scene occluder.

Implement a deliberately small retained screen-space vector facility rather
than a general UI framework. A suitable concrete design is:

```go
type OverlayLayer struct {
    Static  []VectorBatch
    Dynamic []VectorBatch
}

type VectorBatch struct {
    Color color.RGBA
    Width float32
    Path  vector.Path
}
```

The exact names may change, but retain these properties:

- paths use logical screen coordinates and are clipped only to the viewport;
- lines are grouped by color, width and blend policy before Ebitengine
  submission;
- immutable artwork and glyph topology are compiled once and shared;
- static paths/vertex data are retained across frames rather than reconstructed;
- dynamic batches are rebuilt only when their value or geometry changes;
- value invalidation uses explicit compact keys such as shield value, speed
  bucket, mission revision, ammunition count, selected fighter or realism
  profile—not an unconditional per-frame rebuild;
- moving cues such as a targeting arrow remain tiny dynamic batches and never
  force rebuilding the surrounding instrument panel;
- authoritative values come from the simulation snapshot, while layout,
  caching, color and visibility remain client-local presentation state; and
- overlay batching is independent of the realism profile and cannot activate
  physical scene depth.

Do not use an off-screen texture merely to avoid organizing vector geometry if
retained vector paths or reusable vertex/index buffers provide the same result.
A cached image is appropriate only for a genuinely static, expensive layer and
only after measurement shows that one image composite is cheaper on the target
backend. Preserve the vector appearance regardless of the chosen cache form.

Migrate incrementally in this order:

1. Add overlay-specific counters and a deterministic cockpit workload so the
   current submission/allocation cost is recorded before migration.
2. Extract immutable glyph definitions from draw functions and compile text
   into batched paths without per-glyph maps, slices or draw calls.
3. Move the static first-person cockpit frame, shield outline and fixed labels
   into retained batches.
4. Move shield digits, speed, torpedo count, objective status and targeting
   cues into independently invalidated dynamic batches.
5. Move controls/restart cards, pause/quit prompts, realism selector and the
   fighter-showcase specification panel onto the same overlay facility.
6. Remove superseded immediate-mode helpers once no caller depends on them;
   keep one simple immediate path only for rare development diagnostics if it
   is measurably harmless.

Instrumentation should distinguish `overlay static batches`, `overlay dynamic
batches`, `overlay vectors`, `overlay rebuilds`, `overlay submissions`, overlay
CPU time and overlay allocation count from world geometry statistics. Normal
gameplay must not collect fine counters unless diagnostics are visible.

Add deterministic tests for glyph/path compilation, centering and viewport
clipping; batch separation by style; static-batch reuse; dynamic invalidation;
mission/ammunition changes rebuilding only their owning batch; cockpit overlay
exclusion from prepared candidates and depth domains; and final layer ordering.
Keep screenshot tests secondary to math/state tests.

Acceptance criteria:

- enabling cockpit instrumentation does not change prepared candidate, face,
  triangle, depth-writer or hidden-line sample counts;
- an unchanged cockpit frame performs no overlay geometry rebuilds;
- one changing instrument does not rebuild unrelated artwork;
- ordinary cockpit presentation has bounded, near-zero per-frame allocations;
- overlay submissions scale with style batches, not glyph/segment count; and
- representative orbital, near-surface, trench and interior benchmarks remain
  within the frame budget with the overlay enabled and disabled.

This refinement is performance isolation as well as code organization: future
interior cockpit artwork may grow substantially without increasing the cost of
physical visibility resolution, and future renderer changes cannot
accidentally treat UI decoration as world topology.

### Immediate implementation order

1. Add missing per-phase counters and deterministic surface/interior workloads.
2. Fix depth activation so it considers only the culled active-frame render
   plan, never inactive showcase objects or unrelated environments.
3. Introduce one reusable prepared-frame candidate list and route star, depth,
   line, showcase and transition processing through it.
4. Add aggregate tile/feature bounds, shared feature prototypes and environment
   LOD before substantially increasing Death Star surface density.
5. Precompute face triangles/planes and reuse face classification across
   surface, depth, point-occlusion and line passes.
6. Add an explicit `ViewContext`/background specification, followed by room and
   portal environment providers for interiors.
7. Add flat opaque triangle surfaces first, then textured materials and finally
   sorted translucent materials; do not make filled rendering a prerequisite
   for the vector modes.
8. Extract cockpit and UI artwork into the retained screen-space overlay path
   above, beginning with glyph batching and the static cockpit frame before
   migrating dynamic instruments and secondary cards/panels.

Status (2026-09-13): items 1–6 are complete. Gameplay, showcase, and the
implemented approach-transition cut scene now produce the same prepared
candidates and use the same bounds, depth-domain, point-occlusion, clipping,
line-generation, batching, and diagnostics paths. Transition destination tiles
are acquired during update, not generated by frame preparation or drawing.
The former direct transition renderer has been removed. The shared environment
candidate builder accepts an explicit frame transform, providing the same
preparation boundary for later registered cut-scene views without adding a
second rendering path. Streamed environments now carry precomputed aggregate
tile and feature bounds, projected-size LOD with bounded hysteresis state, and
shared immutable feature prototypes rendered through instance transforms.
Tile, feature, and part rejection happens before candidates reach transform,
depth, point-occlusion, or line work, with corresponding HUD diagnostics.
Immutable model topology also caches ordinary face triangulation. Each
surviving frame candidate prepares camera-space vertices, transformed normals,
front-face classification, policy-selected edges, and—only when requested—
near/far-clipped projected surface triangles once. Line generation, CPU depth,
and sparse background occlusion consume that shared record instead of walking,
transforming, classifying, clipping, and projecting the model independently.

`internal/view.Context` is now the common presentation input for frame ID,
camera/view transform and background selection. Gameplay, showcase, surface and
transition preparation retain that resolved context rather than consulting
independent global camera and starfield state. Background drawing consumes only
the generic `none`, distant `skyfield`, or world-space-stars policy. Registered
enclosed rooms override the normal exterior policy with `none` while
unregistered exterior and surface frames preserve the configured star mode.

The environment registry now also acts as the initial room/portal provider.
Rooms contribute immutable prepared local geometry, derived aggregate bounds,
background policy and ordered planar portal apertures linking simulation
frames. Room surfaces enter the existing bounds, prepared-geometry, depth,
point-occlusion and line pipeline; they do not create a separate interior
renderer. Portal destination rendering and aperture-constrained background
projection remain a later consumer of this metadata, to be introduced with the
first concrete interior rather than speculated into the core pipeline.

Item 7's flat and textured opaque increments are complete:
`scene.Part` may opt into a validated flat opaque surface material while the
zero value remains vector-only. Filled surfaces reuse the prepared,
camera-facing, near/far-clipped triangles; only opted-in triangles incur a
stable back-to-front sort and they are submitted in bounded reusable Ebitengine
batches before luminous vector outlines. Because a real opaque fill covers the
prebatched background naturally, those candidates skip redundant sparse-star
geometry tests while retaining ordinary physical depth participation. HUD
diagnostics report opaque candidates, triangles, batches and submission time.

The Death Star near-surface environment is the first concrete flat-fill
workload. Streamed deck faces use a neutral grey opaque material and the trench
floor/walls a slightly darker grey beneath their existing luminous green
vector edges. These candidates still write/test physical depth for scene lines but no
longer participate in sparse star-versus-triangle tests: the batched GPU fill
covers the prebatched skyfield naturally. Deterministic preparation tests guard
that separation. Interactive review should compare star geometry rejection,
opaque candidates/triangles/batches, depth work, opaque submission time and
overall responsiveness before adopting fill more widely.

Textured opaque surfaces now use optional per-face UVs that remain attached to
the authored face through model preparation, transforms, merges and near/far
clipping. A small immutable CPU texture registry owns RGBA data by ID; the game
creates Ebitengine images lazily on the render thread. The existing far-to-near
opaque order is preserved, with consecutive same-texture triangles submitted
in bounded batches. HUD diagnostics distinguish textured triangles/batches
from the total opaque workload. The first restrained use is the physical Death
Star deck: subtle low-contrast procedural plating under its existing green
vectors, while distant horizon tiles remain flat and inexpensive. Flat fills,
wireframe presentation, depth/point visibility and all five realism profiles
continue through the same prepared-frame path.

Item 7 is complete. `SurfaceTranslucent` accepts a flat tint or registered
texture with material alpha strictly between 0 and 255. It reuses prepared,
camera-facing, clipped triangles and shares the stable far-to-near surface
order with opaque fills, so nearer opaque faces cover farther glass while
nearer glass blends over opaque surfaces. Translucent parts do not write the
CPU depth buffer or reject background points; their vector outlines still
use ordinary depth testing. The X-Wing, TIE fighter, TIE Interceptor, and
Millennium Falcon cockpit windows share the restrained amber-glass material;
their surrounding hulls, pylons, and corridor remain opaque. HUD diagnostics
separate translucent candidates, triangles,
batches and submission time from opaque work. This is a painter-sorted edge
case, not an exact solution for intersecting translucent polygons; add a more
expensive visibility treatment only when a concrete scene demonstrates need.
Before applying textures to large oblique near-camera geometry, assess affine
UV interpolation artifacts and subdivide only where visibly necessary; do not
impose that cost on flat or small surfaces.

Item 8 is planned and specified above. The recent mission-computer work exposed
the cost of rebuilding glyph maps and submitting individual vector segments;
the immediate batching cleanup is only the first step. Complete the retained
overlay boundary before substantially expanding cockpit or interior
instrumentation.
