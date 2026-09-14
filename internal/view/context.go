// Package view describes the presentation context for one rendered frame.
// It deliberately contains no Ebitengine values so simulation frames,
// environments, cameras, and render preparation can share it.
package view

import (
	"fmt"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

// BackgroundKind selects the source behind prepared scene geometry.
type BackgroundKind string

const (
	BackgroundNone       BackgroundKind = "none"
	BackgroundSkyfield   BackgroundKind = "skyfield"
	BackgroundWorldStars BackgroundKind = "world-stars"
)

// Background is intentionally small. Portal-constrained regions and richer
// background providers can be added without changing candidate preparation.
type Background struct {
	Kind BackgroundKind
}

func (background Background) Validate() error {
	switch background.Kind {
	case BackgroundNone, BackgroundSkyfield, BackgroundWorldStars:
		return nil
	default:
		return fmt.Errorf("unknown background kind %q", background.Kind)
	}
}

// Context is the authoritative presentation input for one frame. ViewMatrix
// transforms coordinates in FrameID into camera space.
type Context struct {
	FrameID    scene.FrameID
	ViewMatrix math3d.Mat4
	Background Background
}

func (context Context) Validate() error {
	if context.FrameID == "" {
		return fmt.Errorf("view context requires a frame")
	}
	if err := context.Background.Validate(); err != nil {
		return fmt.Errorf("view context: %w", err)
	}
	return nil
}
