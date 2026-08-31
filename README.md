# Star Wars Vector Game

A non-commercial learning project inspired by the wireframe vector presentation
of Atari's 1983 *Star Wars* arcade game. Models and code are original and no
game assets are copied.

The implementation follows [the build plan](star-wars-vector-game-plan.md) and
uses Go, [Ebitengine](https://ebitengine.org/), and a custom 3D math/rendering
pipeline.

## Current milestone

Step 14 in progress: five autonomous instances of the existing fighter use
independent deterministic pursuit heuristics, widely separated starting and
return positions,
randomized pitch/yaw wander, and timed fly-away excursions before returning to
the player's vicinity. Independently randomized attack windows can overlap, so
multiple fighters may attack the player together while flying curved approaches
with different radii, directions, and firing cadences; bolts are aimed at the
player throughout the attack arc rather than requiring exact nose alignment,
with deterministic lateral and vertical aim error so attacks are threatening
without being guaranteed hits.
Each fighter also selects
a new near-maximum cruising-
speed variation every five seconds and accelerates smoothly toward it, while
retaining a high minimum forward speed during turns and avoidance. Swept laser
collision detection consumes a hitting bolt and replaces the struck fighter with three
independently drifting and spinning components. Those components remain valid
laser targets; a second hit breaks one into its constituent wireframe polygons,
which receive a fresh two-second drift-and-spin lifetime. Components and final
polygon shards are non-physical, so the effect cannot create collision cascades.
Disintegration trajectories retain the destroyed object's original travel vector
while adding a deterministic per-piece blast spread.
Fighter collisions disintegrate both participants. Destroyed autonomous fighters
return after three seconds at safe positions; a destroyed player enters a
three-second external orbit view of the disintegration, then follows a randomly
selected surviving swarm fighter until `R` respawns the player. Swarm replacements are held until
the entire current swarm has been destroyed, then return together after the
normal delay. Respawns are placed well away from the nearest surviving swarm
fighter and player respawns avoid all live fighters, bolts, and disintegration
debris before restoring cockpit view. Swarm replacements face back toward the
nearest surviving fighter. Flight, pursuit, firing,
projectiles, and debris use fast arcade-tempo tuning with a `2.0x` world-motion
scale. Autonomous fighters predict close approaches, hold deterministic evasive
yaw/pitch/roll manoeuvres, and slow modestly to reduce—but not eliminate—physical
collisions. A wider proximity fallback handles curved trajectories that linear
prediction cannot see. Real-time firing, behavior, respawn, and debris timers
are not shortened. Swarm avoidance continues while the player is in the
destroyed/spectator state, although swarm firing pauses until the player
respawns.

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

You should see a 960×540 dark window with green, low-poly twin-panel fighters.
The game starts inside the player's cockpit at maximum forward speed, heading
toward a distant forward formation of five autonomous fighters that pursue and
attack with varied steering.

In cockpit view, border threat markers point toward the eight nearest fighters
or incoming enemy bolts. Each marker's urgency progresses from blue to orange to
red, with a flashing red marker for immediate danger.

## Controls

- `M`: switch between autopilot and manual flight
- Any `W`/`S`, arrow, `Q`/`E`, or `Space` navigation input automatically enters
  manual flight, regardless of the current camera view
- `G`: toggle captured-mouse yaw/pitch steering
- `V`: cycle fixed, chase, cockpit, and orbit views
- `Shift`: follow a random active swarm fighter
- `W` / `S`: increase forward or backward speed
- Arrow keys: yaw and pitch
- `Q` / `E`: roll
- `Space`: toggle the textual heads-up display
- Move the mouse in cockpit view to aim the targeting crosshairs
- Hold right mouse in cockpit view to steer toward the pointer
- `F` or left mouse: fire alternating paired laser bolts toward the crosshairs
  (maximum three paired volleys in any 1.5-second window)
- Firing automatically returns the camera to the player cockpit
- Opposing laser bolts can intercept each other in flight
- `P`: pause or resume simulation
- `R`: reset the fighter, or respawn after destruction
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

Planned difficulty selection will use curated `Cadet`, `Pilot`, `Ace`, and
optional `Nightmare` profiles that bundle swarm size, speed, attack cadence, aim
error, avoidance, and recovery settings.

Player shields start at eight strength points, shown as eight mirrored segments on each side; a laser hit loses one point and a collision loses three
to a collision, recharge one segment after 20 seconds without damage, and
destroy the fighter only when strength falls below zero. The cockpit HUD shows
the mirrored eight-segment-per-side triangular shield indicator at the top center.

Directional objects use `+Z` as their front and support signed axial speed:
positive moves forward and negative moves backward. Pose and yaw/pitch/roll
rates use quaternion orientation to avoid gimbal lock while preserving intuitive
flight controls.

## License

This learning project is released under the MIT License. "Star Wars" and related
marks belong to their respective owners; this project is not affiliated with or
endorsed by them.
