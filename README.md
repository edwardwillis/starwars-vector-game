# Star Wars Vector Game

A non-commercial learning project inspired by the wireframe vector presentation
of Atari's 1983 *Star Wars* arcade game. Models and code are original and no
game assets are copied.

The implementation follows [the build plan](star-wars-vector-game-plan.md) and
uses Go, [Ebitengine](https://ebitengine.org/), and a custom 3D math/rendering
pipeline.

## Current milestone

Step 6: an original, hand-authored twin-panel fighter plus a scene-object layer
for independently transformed, multipart wireframe objects.

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

You should see a 960×540 dark window with a red, low-poly twin-panel fighter.

## Roadmap

The scene architecture supports additional ships, projectiles, laser cannons,
and compound Death Star geometry through the same rendering pipeline. The next
milestone animates object transforms. Culling remains off by default to preserve
the arcade-like visual style.

## License

This learning project is released under the MIT License. "Star Wars" and related
marks belong to their respective owners; this project is not affiliated with or
endorsed by them.
