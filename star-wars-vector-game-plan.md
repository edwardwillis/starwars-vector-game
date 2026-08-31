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
intact object and spawns two or three transient fragment objects at its pose.
Each fragment inherits the object's motion, receives a different outward linear
velocity and angular velocity, and spins away independently for exactly two
seconds before being removed from the universe. Fragments are visual debris:
they have no controller, cannot fire, do not receive damage, and do not produce
further fragments. The laser bolt that caused the hit is consumed.

Physical collisions are resolved symmetrically: when two participating objects
collide, both objects disintegrate and each produces its own two or three
fragments. This includes collisions such as fighter against fighter. The
collision system resolves each object at most once per simulation tick so a
single pile-up cannot spawn duplicate fragment sets. Background-only items such
as starfield points and non-physical navigation markers do not participate in
collision detection. Transient disintegration fragments also do not collide.

Fragment count, directions, speeds, and spin must be deterministic from stable
simulation data such as the destroyed object ID and impact tick. This keeps
local tests, replay, and the later authoritative server consistent. Networked
clients receive the destruction and fragment spawn/removal events rather than
generating authoritative debris locally. If a camera targets a destroyed
object, it follows an explicitly selected fragment when supported or safely
falls back to an appropriate spectator view.

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

The core boundary is intentionally small:

```go
type Controller interface {
    Decide(Context) Intent
}

type Intent struct {
    Thrust Vec3
    Turn   Vec3
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
11. Visibility modes — faces, hidden-line depth resolver, interactive realism slider
12. Starfield — deterministic world points, wrapping, projection, motion reference
13. Cockpit targeting — pointer aim, right-button steering, firing cone, converging bolts
14. Dogfight — multiple catalog fighter instances, deterministic pursuit/wander/excursion and variable-speed heuristics, symmetric collisions, two-second disintegration debris, player and swarm respawn lifecycle
15. Death Star — reusable surface modules, trench, towers, targeting reticle
16. Camera anchors — cockpit, chase, spectator, and Death Star viewpoints
17. Simulation extraction — stable IDs and renderer-independent world updates
18. Authoritative server — fixed ticks, autonomous objects, sessions, snapshots
19. Controller interface — intents, registry, static and manual strategies
20. Rule-driven intelligence — patrol, pursuit, evasion, targeting, formations
21. Multiplayer client — control input, interpolation, ownership, view switching
22. External agent adapter — asynchronous AI/MCP decisions and safe fallback
23. Score, game states, sound
24. Stretch: prediction/reconciliation, replay, persistence, multiple rooms
25. Stretch: controller evaluation, tournaments, and strategy hot-loading
26. Stretch: CRT/vector-glow shader, fixed-point math (period-accurate)

## Notes
- Step 5 is first visually demonstrable milestone (target early win).
- Keep each step in its own commit/branch for incremental review.
- Visibility defaults to `VisibilityAll`; backface and hidden-line modes arrive in
  step 11 and remain runtime-switchable features.
