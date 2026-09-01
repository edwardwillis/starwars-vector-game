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

// LaserConfig contains the tunable behavior of a laser projectile. Geometry
// and styling remain owned by the catalog.
type LaserConfig struct {
	Speed    float64
	SpinRate float64
	Lifetime float64
}

func DefaultLaserConfig() LaserConfig {
	return LaserConfig{
		Speed:    LaserSpeed,
		SpinRate: LaserSpinRate,
		Lifetime: LaserLifetime,
	}
}

func (config LaserConfig) Validate() error {
	if config.Speed <= 0 || math.IsNaN(config.Speed) || math.IsInf(config.Speed, 0) {
		return fmt.Errorf("laser speed must be positive")
	}
	if math.IsNaN(config.SpinRate) || math.IsInf(config.SpinRate, 0) {
		return fmt.Errorf("laser spin rate must be finite")
	}
	if config.Lifetime <= 0 || math.IsNaN(config.Lifetime) || math.IsInf(config.Lifetime, 0) {
		return fmt.Errorf("laser lifetime must be positive")
	}
	return nil
}

type Spawn struct {
	Object   scene.Object
	OwnerID  scene.ObjectID
	Lifetime float64
}

// FireLaserToward creates a bolt whose local forward axis points from its
// muzzle toward a world-space convergence point.
func FireLaserToward(shooter scene.Object, id scene.ObjectID, muzzle string, target math3d.Vec3) (Spawn, error) {
	return FireLaserTowardWithConfig(shooter, id, muzzle, target, DefaultLaserConfig())
}

// FireLaserTowardWithConfig is FireLaserToward with session-selected projectile
// tuning supplied by the active game profile.
func FireLaserTowardWithConfig(shooter scene.Object, id scene.ObjectID, muzzle string, target math3d.Vec3, config LaserConfig) (Spawn, error) {
	if err := config.Validate(); err != nil {
		return Spawn{}, err
	}
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
		Speed:    shooter.Motion.Speed + config.Speed,
		RollRate: config.SpinRate,
	}
	return Spawn{Object: bolt, OwnerID: shooter.ID, Lifetime: config.Lifetime}, nil
}

// FireLaser creates a bolt at a named shooter anchor. The bolt inherits the
// shooter's signed axial speed, moves forward rapidly, and spins about +Z.
func FireLaser(shooter scene.Object, id scene.ObjectID, muzzle string) (Spawn, error) {
	return FireLaserWithConfig(shooter, id, muzzle, DefaultLaserConfig())
}

// FireLaserWithConfig is FireLaser with session-selected projectile tuning
// supplied by the active game profile.
func FireLaserWithConfig(shooter scene.Object, id scene.ObjectID, muzzle string, config LaserConfig) (Spawn, error) {
	if err := config.Validate(); err != nil {
		return Spawn{}, err
	}
	pose, ok := shooter.Anchor(muzzle)
	if !ok {
		return Spawn{}, fmt.Errorf("object %d has no %q anchor", shooter.ID, muzzle)
	}
	bolt := catalog.LaserBolt(id, pose)
	bolt.Motion = kinematics.Motion{
		Speed:    shooter.Motion.Speed + config.Speed,
		RollRate: config.SpinRate,
	}
	return Spawn{
		Object:   bolt,
		OwnerID:  shooter.ID,
		Lifetime: config.Lifetime,
	}, nil
}
