# Star Wars Vector Arcade Homage — Go Build Plan

Non-commercial learning project. Homage to 1983 Atari arcade "Star Wars". Wireframe vector style, not photo-realism. All models hand-defined, no copied assets.

## Stack
- Language: Go
- Rendering: Ebiten (window, input, line drawing)
- Math: custom Vec3/Mat4 package, no 3D engine

## Render Pipeline (extensible, stage-based)

```
Model (verts + edges)
  -> Transform (world matrix)
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

type Culler interface {
    Cull(verts []Vec3, edges []Edge) []Edge
}

type Pipeline struct {
    Culler Culler // nil = disabled
}
```

New stages (e.g. `BackfaceCull`, `PainterAlgoHSR`) implement the interface and slot in later without touching rest of code.

## Build Steps

1. Go basics refresher — structs, slices, methods, goroutines
2. Ebiten hello world — window, draw single line
3. Math package — Vec3, Mat4, rotate/translate/scale, perspective projection
4. Static wireframe cube — validate pipeline end to end
5. X-Wing model — hardcoded verts/edges, render wireframe
6. Rotate X-Wing on Y axis, per-frame update
7. Input — mouse/keys rotate and zoom model
8. Multi-axis rotation, camera transform, multiple objects on screen
9. TIE Fighter model
10. Starfield background, ship movement (WASD)
11. Dogfight — enemy movement, simple AI, collision detection
12. Death Star trench run — tracking camera, wall geometry, targeting reticle
13. Score, game states, sound
14. Stretch: CRT/vector-glow shader, fixed-point math (period-accurate)

## Notes
- Step 5 is first visually demonstrable milestone (target early win).
- Keep each step in its own commit/branch for incremental review.
- Culling/HSR stays off until step 8+ to match original arcade look; toggle exists from step 3 onward.
