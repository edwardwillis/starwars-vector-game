# Star Wars Vector Game

A non-commercial learning project inspired by the wireframe vector presentation
of Atari's 1983 *Star Wars* arcade game. Models and code are original and no
game assets are copied.

The implementation follows [the build plan](star-wars-vector-game-plan.md) and
uses Go, [Ebitengine](https://ebitengine.org/), and a custom 3D math/rendering
pipeline.

## Current milestone

Step 9: interactive manual flight plus fixed, chase, cockpit, and orbit cameras
targeted through stable object IDs and catalog-defined camera anchors.

## Prerequisites

- Go 1.24 or newer
- Native libraries required by Ebitengine on your operating system

On Ubuntu/Debian, Ebitengine's desktop build dependencies can be installed with:

```sh
sudo apt update
sudo apt install gcc libc6-dev libgl1-mesa-dev libxcursor-dev \
  libxi-dev libxinerama-dev libxrandr-dev libxxf86vm-dev libasound2-dev pkg-config
```

## Run locally

```sh
go mod tidy
go test ./...
go run .
```

You should see a 960×540 dark window with a red, low-poly twin-panel fighter
moving along a gentle curved path while yawing and rolling.

## Controls

- `M`: switch between autopilot and manual flight
- `V`: cycle fixed, chase, cockpit, and orbit views
- `W` / `S`: increase forward or backward speed
- Arrow keys: yaw and pitch
- `Q` / `E`: roll
- `Space`: stop
- `P`: pause or resume simulation
- `R`: reset fighter pose and motion
- `+` / `-` or mouse wheel: camera zoom

## Roadmap

The scene architecture supports additional ships, projectiles, laser cannons,
and compound Death Star geometry through the same rendering pipeline. The next
milestone adds manual input for movement and rotation. Surface visibility
processing remains off by default to preserve the arcade-like visual style.

Future visibility modes will switch at runtime between drawing every edge,
backface culling, and full depth-based hidden-line removal. The all-edges mode
remains the default arcade presentation.

An interactive realism slider will progress from transparent arcade wireframes
through backface rejection, per-object hidden-line removal, scene-wide
occlusion, and distance-based depth cues.

Later milestones add named camera anchors for cockpit, chase, spectator, and
Death Star viewpoints, followed by an authoritative Go server for a shared live
simulation containing autonomous and user-controlled objects.

A pluggable controller architecture will support static objects, human input,
deterministic rule-driven behavior, and asynchronous external AI agents such as
MCP-backed controllers without granting them authority over simulation state.

Directional objects use `+Z` as their front and support signed axial speed:
positive moves forward and negative moves backward. Pose and yaw/pitch/roll
rates use quaternion orientation to avoid gimbal lock while preserving intuitive
flight controls.

## License

This learning project is released under the MIT License. "Star Wars" and related
marks belong to their respective owners; this project is not affiliated with or
endorsed by them.
