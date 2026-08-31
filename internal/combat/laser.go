// Package combat creates and describes transient combat objects.
package combat

import (
	"fmt"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

const (
	LaserSpeed    = 18.0
	LaserSpinRate = 8.0
	LaserLifetime = 2.0
)

type Spawn struct {
	Object   scene.Object
	OwnerID  scene.ObjectID
	Lifetime float64
}

// FireLaserToward creates a bolt whose local forward axis points from its
// muzzle toward a world-space convergence point.
func FireLaserToward(shooter scene.Object, id scene.ObjectID, muzzle string, target math3d.Vec3) (Spawn, error) {
	pose, ok := shooter.Anchor(muzzle)
	if !ok {
		return Spawn{}, fmt.Errorf("object %d has no %q anchor", shooter.ID, muzzle)
	}
	direction := target.Sub(pose.Position).Normalize()
	if direction == (math3d.Vec3{}) {
		return Spawn{}, fmt.Errorf("object %d muzzle %q is already at its target", shooter.ID, muzzle)
	}
	yaw := math.Atan2(direction.X, direction.Z)
	pitch := -math.Asin(max(-1, min(1, direction.Y)))
	pose.Orientation = math3d.QuaternionFromYawPitchRoll(yaw, pitch, 0)
	bolt := catalog.LaserBolt(id, pose)
	bolt.Motion = kinematics.Motion{
		Speed:    shooter.Motion.Speed + LaserSpeed,
		RollRate: LaserSpinRate,
	}
	return Spawn{Object: bolt, OwnerID: shooter.ID, Lifetime: LaserLifetime}, nil
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
