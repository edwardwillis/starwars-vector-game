# Star Wars Vector Game

A non-commercial learning project inspired by the wireframe vector presentation
of Atari's 1983 *Star Wars* arcade game. Models and code are original and no
game assets are copied.

The implementation follows [the build plan](star-wars-vector-game-plan.md) and
uses Go, [Ebitengine](https://ebitengine.org/), and a custom 3D math/rendering
pipeline.

## Current milestone

The immediate goal is a complete single-player **Battle of Yavin** vertical
slice. The current beta has a playable arcade loop:

```text
Mission select → briefing → hyperspace launch → orbital combat
→ Death Star approach → surface assault → trench run
→ exhaust-port torpedo attack → timed escape → mission result
```

The game includes curated `Cadet`, `Pilot`, `Ace`, and `Nightmare` profiles,
autonomous Imperial fighters and surface installations, proton torpedoes,
authoritative mission phases, a clean retry lifecycle, and a capped score
breakdown. The Death Star remains a performant orbital vector representation
that transfers the player into its local surface environment for near-surface
flight.

The renderer retains five graduated vector-realism profiles, compiled model
topology, culling/LOD, clipping, hidden-line processing, and profiling HUD and
CSV telemetry support. The project deliberately remains a sparse Atari-style
vector game rather than a conventional filled 3D renderer.

## Prerequisites

- Go 1.24 or newer
- Native libraries required by Ebitengine on your operating system

On Ubuntu/Debian, Ebitengine's desktop build dependencies can be installed with:

```sh
sudo apt update
sudo apt install gcc libc6-dev libgl1-mesa-dev libxcursor-dev \
  libxi-dev libxinerama-dev libxrandr-dev libxxf86vm-dev libasound2-dev pkg-config
```

Headless test runs additionally require a virtual X11 display:

```sh
sudo apt install xvfb libgl1 libgl1-mesa-dri libx11-6 libxcursor1 \
  libxext6 libxi6 libxinerama1 libxrandr2 libxrender1
```

## Testing

Run the complete suite with:

```sh
make test
```

On headless Linux, this automatically runs Ebitengine under Xvfb. On systems
with an active display, it runs `go test ./...` directly. Additional package
patterns or test flags can be passed to the wrapper when needed:

```sh
./scripts/test.sh ./internal/game -run TestFireLaser
```

For a faster display-independent development loop, run:

```sh
make test-unit
```

The CI workflow runs the complete Xvfb-backed suite on every push and pull
request.

## Run locally

```sh
make test
go run .
```

The title screen selects the mission and one of the curated `Cadet`, `Pilot`,
`Ace`, or `Nightmare` difficulty profiles. The optional command-line profile
chooses the difficulty initially highlighted by the game shell:

```sh
go run . -profile cadet
go run . -profile pilot
go run . -profile ace
go run . -profile nightmare
```

The normal flow is mission selection, difficulty selection, the Battle of
Yavin briefing, and an explicit launch through the hyperspace arrival into the
existing mission. Battle of Hoth and Battle of Endor are visible future
missions but cannot be launched. A newly launched Yavin mission always starts
from a clean session.

## Windows beta download

The current Windows x64 beta is published as a GitHub prerelease:

- [Download Battle of Yavin Beta 1](https://github.com/edwardwillis/starwars-vector-game/releases/tag/v0.4.0-beta.1)

Download and extract `StarWarsVectorGame-v0.4.0-beta.1-win64.zip`, then run
`starwars-vector.exe`. The accompanying `.sha256` file can be used to verify
the ZIP download. This is an unsigned beta executable, so Windows SmartScreen
may require an explicit confirmation before it runs.

For post-run performance analysis, the optional development telemetry flag
writes buffered one-second CSV samples. It is off by default and records the
existing renderer/HUD metrics, update and draw-stage timings, and actual
FPS/TPS without logging every frame:

```sh
go run . -telemetry surface-flight.csv
```

When developing from WSL, use the native Windows graphics stack for interactive
play and renderer profiling:

```sh
make build-windows
make run-windows
make run-windows WINDOWS_RUN_ARGS='-profile cadet -telemetry C:\Users\YOUR_WINDOWS_USER\surface-flight-native.csv'
```

`build-windows` writes `starwars-vector.exe` to the current Windows user's home
directory by default. Override `WINDOWS_EXE` with a Windows-mounted WSL path if
you need a different output location.

In cockpit view, border threat markers point toward the eight nearest fighters
or incoming enemy bolts. Each marker's urgency progresses from blue to orange to
red, with a flashing red marker for immediate danger.

## Controls

- On the title screen, `Up` / `Down` select a mission, `Left` / `Right` select
  difficulty, and `Enter`, `Space`, `F`, or left mouse continue
- On the briefing, `Enter`, `Space`, `F`, or left mouse launches; `Backspace`
  returns to mission selection
- On the outcome screen, `Enter`, `Space`, or left mouse opens the mission
  result; on the result screen `Enter` or `R` retries through the briefing and
  `T` returns to the title screen
- `N` on the title screen retains the direct near-surface development start
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
- `?`: show or hide the controls card
- Move the mouse in cockpit view to aim the targeting crosshairs
- Hold right mouse in cockpit view to steer toward the pointer
- `F` or left mouse: fire alternating paired laser bolts toward the crosshairs
  (maximum three paired volleys in any 1.5-second window)
- `T`: fire a proton torpedo during the exhaust-port attack
- Firing automatically returns the camera to the player cockpit
- Opposing laser bolts can intercept each other in flight
- `P`: pause or resume simulation
- `R`: reset the fighter during development/play; mission failure normally
  proceeds to the outcome and result screens
- `L`: toggle automatic roll-leveling during near-surface flight
- `[` / `]`: choose a lower or higher vector-realism profile
- `+` / `-` or mouse wheel: camera zoom

## Roadmap

The maintained [project plan](star-wars-vector-game-plan.md) is the source of
truth for implementation status and architectural decisions.

Current development is focused on completing and balancing Battle of Yavin:
outcome presentation, audio, gameplay tuning, performance validation, and beta
feedback. Battle of Endor and Battle of Hoth are future missions; they will
reuse proven systems only where their gameplay genuinely fits. Multiplayer,
server, external-agent, and public extension work are explicitly deferred until
the single-player Yavin loop is complete.

## License

This learning project is released under the MIT License. "Star Wars" and related
marks belong to their respective owners; this project is not affiliated with or
endorsed by them.
