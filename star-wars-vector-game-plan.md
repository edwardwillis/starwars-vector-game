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
14. Score, game states, sound
15. Stretch: CRT/vector-glow shader, fixed-point math (period-accurate)

## Notes
- Step 5 is first visually demonstrable milestone (target early win).
- Keep each step in its own commit/branch for incremental review.
- Culling/HSR stays off until step 8+ to match original arcade look; toggle exists from step 3 onward.
