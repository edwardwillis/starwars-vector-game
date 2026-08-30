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
  -> Model (verts + edges)
  -> Transform (object world matrix)
  -> Camera (view matrix)
  -> Project (perspective -> screen)
  -> Clip (near plane, screen bounds)
  -> Cull (optional: backface/HSR toggle)
  -> Draw (line draw)
```

Each stage is an interface. Renderer holds pipeline config, runs stages in order. Stages toggle on/off independently (e.g. culling off = original arcade look, culling on = later upgrade).

Core types:

```go
type Vec3 struct{ X, Y, Z float64 }
type Edge struct{ A, B int } // vertex indices
type Model struct {
    Verts []Vec3
    Edges []Edge
}

type Part struct {
    Mesh      Model
    Color     color.RGBA
    LineWidth float32
}

type Object struct {
    Name      string
    Transform Mat4
    Parts     []Part
}

type Culler interface {
    Cull(verts []Vec3, edges []Edge) []Edge
}

type Pipeline struct {
    Culler Culler // nil = disabled
}
```

New stages (e.g. `BackfaceCull`, `PainterAlgoHSR`) implement the interface and slot in later without touching rest of code.

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
7. Rotate fighter on Y axis, per-frame update
8. Input — mouse/keys rotate and zoom model
9. Multi-axis rotation, camera transform, multiple objects on screen
10. Object catalog — additional ships, laser bolt, cannon emplacement
11. Starfield background, ship movement (WASD)
12. Dogfight — enemy movement, spawning, lifetime, simple AI, collisions
13. Death Star — reusable surface modules, trench, towers, targeting reticle
14. Camera anchors — cockpit, chase, spectator, and Death Star viewpoints
15. Simulation extraction — stable IDs and renderer-independent world updates
16. Authoritative server — fixed ticks, autonomous objects, sessions, snapshots
17. Controller interface — intents, registry, static and manual strategies
18. Rule-driven intelligence — patrol, pursuit, evasion, targeting, formations
19. Multiplayer client — control input, interpolation, ownership, view switching
20. External agent adapter — asynchronous AI/MCP decisions and safe fallback
21. Score, game states, sound
22. Stretch: prediction/reconciliation, replay, persistence, multiple rooms
23. Stretch: controller evaluation, tournaments, and strategy hot-loading
24. Stretch: CRT/vector-glow shader, fixed-point math (period-accurate)

## Notes
- Step 5 is first visually demonstrable milestone (target early win).
- Keep each step in its own commit/branch for incremental review.
- Culling/HSR stays off until step 8+ to match original arcade look; toggle exists from step 3 onward.
