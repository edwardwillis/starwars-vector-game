// Package combat creates and describes transient combat objects.
package combat

import (
	"fmt"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

const (
	LaserSpeed    = 10.0
	LaserSpinRate = 5.5
	LaserLifetime = 2.0
)

type Spawn struct {
	Object   scene.Object
	OwnerID  scene.ObjectID
	Lifetime float64
}

// FireLaser creates a bolt at a named shooter anchor. The bolt inherits the
// shooter's signed axial speed, moves forward rapidly, and spins about +Z.
func FireLaser(shooter scene.Object, id scene.ObjectID, muzzle string) (Spawn, error) {
	pose, ok := shooter.Anchor(muzzle)
	if !ok {
		return Spawn{}, fmt.Errorf("object %d has no %q anchor", shooter.ID, muzzle)
	}
	bolt := catalog.LaserBolt(id, pose)
	bolt.Motion = kinematics.Motion{
		Speed:    shooter.Motion.Speed + LaserSpeed,
		RollRate: LaserSpinRate,
	}
	return Spawn{
		Object:   bolt,
		OwnerID:  shooter.ID,
		Lifetime: LaserLifetime,
	}, nil
}
