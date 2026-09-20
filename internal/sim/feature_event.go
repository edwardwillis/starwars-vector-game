package sim

import (
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

// FeatureDamageEvent is a fixed-tick installation outcome. Presentation can
// derive sparks and fragments from it, while a future server can relay the
// same outcome without sharing render state or tile-generation timing.
type FeatureDamageEvent struct {
	Tick      uint64
	HostID    scene.ObjectID
	Frame     scene.FrameID
	FeatureID string
	Point     math3d.Vec3
	Normal    math3d.Vec3
	Hits      int
	Disabled  bool
	Destroyed bool
}
